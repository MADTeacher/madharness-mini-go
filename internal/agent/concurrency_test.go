package agent

import (
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/events"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

func TestRunModelLoopOverlapsReadToolsWhenParallelEnabled(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 2)
	provider := concurrencyProvider{
		"slow_read_a": blockingTool("slow_read_a", tools.EffectRead, started, release),
		"slow_read_b": blockingTool("slow_read_b", tools.EffectRead, started, release),
	}
	done := runConcurrencyLoop(t, provider, 2, "slow_read_a", "slow_read_b")

	waitToolsStarted(t, started, "slow_read_a", "slow_read_b")
	close(release)
	waitLoopDone(t, done)
}

func TestRunModelLoopSerializesReadToolsWhenParallelDisabled(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 2)
	provider := concurrencyProvider{
		"slow_read_a": blockingTool("slow_read_a", tools.EffectRead, started, release),
		"slow_read_b": blockingTool("slow_read_b", tools.EffectRead, started, release),
	}
	done := runConcurrencyLoop(t, provider, 1, "slow_read_a", "slow_read_b")

	waitToolStarted(t, started, "slow_read_a")
	assertNoToolStart(t, started)
	close(release)
	waitToolStarted(t, started, "slow_read_b")
	waitLoopDone(t, done)
}

func TestRunModelLoopKeepsWriteToolAsBarrier(t *testing.T) {
	releaseRead := make(chan struct{})
	releaseWrite := make(chan struct{})
	started := make(chan string, 3)
	provider := concurrencyProvider{
		"slow_read_a": blockingTool("slow_read_a", tools.EffectRead, started, releaseRead),
		"slow_write":  blockingTool("slow_write", tools.EffectWrite, started, releaseWrite),
		"slow_read_b": instantTool("slow_read_b", tools.EffectRead, started),
	}
	done := runConcurrencyLoop(t, provider, 3, "slow_read_a", "slow_write", "slow_read_b")

	waitToolStarted(t, started, "slow_read_a")
	assertNoToolStart(t, started)
	close(releaseRead)
	waitToolStarted(t, started, "slow_write")
	assertNoToolStart(t, started)
	close(releaseWrite)
	waitToolStarted(t, started, "slow_read_b")
	waitLoopDone(t, done)
}

func TestRunModelLoopCommitsParallelReadTraceInToolCallOrder(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 2)
	provider := concurrencyProvider{
		"slow_read_a": blockingTool("slow_read_a", tools.EffectRead, started, release),
		"slow_read_b": instantTool("slow_read_b", tools.EffectRead, started),
	}
	cfg := testAgentConfig(t)
	tr, err := trace.New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	context, err := agentcontext.BaseContext(cfg, "test trace order")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := tools.NewRegistry(cfg, provider)
	if err != nil {
		t.Fatal(err)
	}
	client := &sequenceClient{responses: []map[string]any{
		concurrencyToolCallResponse("slow_read_a", "slow_read_b"),
		contentResponse("done"),
	}}
	bus := events.NewBus(events.NewTraceSubscriber(tr))
	done := make(chan error, 1)
	go func() {
		result, err := runModelLoop(client, tr, context, registry, 3, loopOptions{
			Events:               bus,
			MaxParallelToolCalls: 2,
		})
		if err == nil && result.Result != "done" {
			err = errUnexpectedResult(result.Result)
		}
		done <- err
	}()

	waitToolsStarted(t, started, "slow_read_a", "slow_read_b")
	close(release)
	waitLoopDone(t, done)

	observed := traceObservationTools(t, tr.Path)
	if len(observed) != 2 || observed[0] != "slow_read_a" || observed[1] != "slow_read_b" {
		t.Fatalf("tool observations = %#v", observed)
	}
}

func runConcurrencyLoop(t *testing.T, provider concurrencyProvider, maxParallel int, names ...string) <-chan error {
	t.Helper()
	cfg := testAgentConfig(t)
	tr, err := trace.New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	context, err := agentcontext.BaseContext(cfg, "test concurrency")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := tools.NewRegistry(cfg, provider)
	if err != nil {
		t.Fatal(err)
	}
	client := &sequenceClient{responses: []map[string]any{
		concurrencyToolCallResponse(names...),
		contentResponse("done"),
	}}
	done := make(chan error, 1)
	go func() {
		result, err := runModelLoop(client, tr, context, registry, 3, loopOptions{MaxParallelToolCalls: maxParallel})
		if err == nil && result.Result != "done" {
			err = errUnexpectedResult(result.Result)
		}
		done <- err
	}()
	return done
}

func concurrencyToolCallResponse(names ...string) map[string]any {
	calls := make([]any, 0, len(names))
	for _, name := range names {
		calls = append(calls, map[string]any{
			"id":       "call_" + name,
			"function": map[string]any{"name": name, "arguments": `{}`},
		})
	}
	return map[string]any{"choices": []any{map[string]any{"message": map[string]any{
		"content":    nil,
		"tool_calls": calls,
	}}}}
}

type concurrencyProvider map[string]tools.Spec

func (p concurrencyProvider) Specs(*tools.Context) ([]tools.Spec, error) {
	specs := make([]tools.Spec, 0, len(p))
	for _, name := range []string{"slow_read_a", "slow_read_b", "slow_write"} {
		if spec, ok := p[name]; ok {
			spec.Name = name
			spec.Description = "test concurrency tool"
			spec.Parameters = tools.Obj(map[string]any{}, nil)
			specs = append(specs, spec)
		}
	}
	return specs, nil
}

func blockingTool(name string, effect tools.Effect, started chan<- string, release <-chan struct{}) tools.Spec {
	return tools.Spec{
		Effect: effect,
		Handler: func(_ *tools.Context, _ map[string]any) tools.Observation {
			started <- name
			<-release
			return tools.OK("test_tool", "done", nil)
		},
	}
}

func instantTool(name string, effect tools.Effect, started chan<- string) tools.Spec {
	return tools.Spec{
		Effect: effect,
		Handler: func(_ *tools.Context, _ map[string]any) tools.Observation {
			started <- name
			return tools.OK("test_tool", "done", nil)
		},
	}
}

func waitToolsStarted(t *testing.T, started <-chan string, wants ...string) {
	t.Helper()
	remaining := map[string]bool{}
	for _, want := range wants {
		remaining[want] = true
	}
	for range wants {
		select {
		case name := <-started:
			if !remaining[name] {
				t.Fatalf("unexpected tool start: %s", name)
			}
			delete(remaining, name)
		case <-time.After(time.Second):
			t.Fatalf("tools did not start: %v", remaining)
		}
	}
}

func waitToolStarted(t *testing.T, started <-chan string, want string) {
	t.Helper()
	select {
	case name := <-started:
		if name != want {
			t.Fatalf("started %s, want %s", name, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("%s did not start", want)
	}
}

func assertNoToolStart(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case name := <-started:
		t.Fatalf("unexpected tool start: %s", name)
	case <-time.After(100 * time.Millisecond):
	}
}

func waitLoopDone(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("agent loop did not finish")
	}
}

func errUnexpectedResult(result string) error {
	return unexpectedResultError{result: result}
}

func traceObservationTools(t *testing.T, path string) []string {
	t.Helper()
	events := readTraceEvents(t, path)
	tools := []string{}
	for _, event := range events {
		if event["event"] == "tool_observation" {
			tools = append(tools, event["tool"].(string))
		}
	}
	return tools
}

type unexpectedResultError struct {
	result string
}

func (e unexpectedResultError) Error() string {
	return "unexpected result: " + e.result
}
