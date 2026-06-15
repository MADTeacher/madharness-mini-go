package agent

import (
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/events"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

func TestRunModelLoopStopsSubagentTurnBeforeLaterEffectfulCall(t *testing.T) {
	cfg := testAgentConfig(t)
	tr, err := trace.New(cfg, "subagent")
	if err != nil {
		t.Fatal(err)
	}
	called := []string{}
	registry, err := tools.NewRegistry(cfg, subagentStopProvider{called: &called})
	if err != nil {
		t.Fatal(err)
	}
	context := agentcontext.NewManager("choose path", agentcontext.Options{})
	client := &sequenceClient{responses: []map[string]any{
		multiToolCallResponse(
			testToolCall{"call_question", "ask_user", map[string]any{"question": "Какой путь выбрать?"}},
			testToolCall{"call_write", "write_file", map[string]any{"path": "after-question.md"}},
		),
	}}
	bus := events.NewBus(events.NewTraceSubscriber(tr))

	result, err := runModelLoop(client, tr, context, registry, 3, loopOptions{
		StopOnUserInput: true,
		Events:          bus,
		Kind:            "subagent",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != "needs_user_input" || result.Result != "Какой путь выбрать?" {
		t.Fatalf("result = %+v", result)
	}
	if strings.Join(called, ",") != "ask_user" {
		t.Fatalf("called = %v", called)
	}
	observed := traceObservationTools(t, tr.Path)
	if strings.Join(observed, ",") != "ask_user" {
		t.Fatalf("tool observations = %v", observed)
	}
}

func TestRunModelLoopCommitsParallelResultsBeforeUserInputStop(t *testing.T) {
	cfg := testAgentConfig(t)
	tr, err := trace.New(cfg, "subagent")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan string, 2)
	registry, err := tools.NewRegistry(cfg, parallelStopProvider{started: started})
	if err != nil {
		t.Fatal(err)
	}
	context := agentcontext.NewManager("choose path", agentcontext.Options{})
	client := &sequenceClient{responses: []map[string]any{
		multiToolCallResponse(
			testToolCall{"call_question", "ask_user", map[string]any{"question": "Какой путь выбрать?"}},
			testToolCall{"call_read", "read_context", map[string]any{}},
		),
	}}
	bus := events.NewBus(events.NewTraceSubscriber(tr))

	result, err := runModelLoop(client, tr, context, registry, 3, loopOptions{
		StopOnUserInput:      true,
		Events:               bus,
		Kind:                 "subagent",
		MaxParallelToolCalls: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != "needs_user_input" || result.Result != "Какой путь выбрать?" {
		t.Fatalf("result = %+v", result)
	}
	waitToolsStarted(t, started, "ask_user", "read_context")
	observed := traceObservationTools(t, tr.Path)
	if strings.Join(observed, ",") != "ask_user,read_context" {
		t.Fatalf("tool observations = %v", observed)
	}
	messages, err := context.Messages(nil)
	if err != nil {
		t.Fatal(err)
	}
	if ids := toolMessageIDs(messages); strings.Join(ids, ",") != "call_question,call_read" {
		t.Fatalf("tool message ids = %v; messages=%+v", ids, messages)
	}
}

func TestRunModelLoopDoesNotRecordMalformedToolCallAsOrphanToolMessage(t *testing.T) {
	cfg := testAgentConfig(t)
	tr, err := trace.New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := tools.NewRegistry(cfg)
	if err != nil {
		t.Fatal(err)
	}
	context := agentcontext.NewManager("handle malformed call", agentcontext.Options{})
	client := &sequenceClient{responses: []map[string]any{
		malformedToolCallResponse(),
		contentResponse("done"),
	}}
	bus := events.NewBus(events.NewTraceSubscriber(tr))

	result, err := runModelLoop(client, tr, context, registry, 3, loopOptions{
		Events: bus,
		Kind:   "run",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != "done" || result.Result != "done" {
		t.Fatalf("result = %+v", result)
	}
	if len(client.seen) != 2 {
		t.Fatalf("model calls = %d", len(client.seen))
	}
	if ids := toolMessageIDs(client.seen[1]); len(ids) != 0 {
		t.Fatalf("orphan tool messages in second request: ids=%v messages=%+v", ids, client.seen[1])
	}
	observed := traceObservationTools(t, tr.Path)
	if strings.Join(observed, ",") != "tool_call" {
		t.Fatalf("tool observations = %v", observed)
	}
}

type subagentStopProvider struct {
	called *[]string
}

func (p subagentStopProvider) Specs(*tools.Context) ([]tools.Spec, error) {
	return []tools.Spec{
		{
			Name: "ask_user",
			Parameters: tools.Obj(map[string]any{
				"question": tools.StrParam("", "Question for the user.", true),
			}, []string{"question"}),
			Handler: func(_ *tools.Context, args map[string]any) tools.Observation {
				*p.called = append(*p.called, "ask_user")
				obs := tools.OK("ask_user", "user input requested", map[string]any{
					"question": strings.TrimSpace(tools.StringArg(args, "question", "")),
				})
				obs["_subagent_stop"] = "needs_user_input"
				return obs
			},
			Effect: tools.EffectState,
		},
		{
			Name: "write_file",
			Parameters: tools.Obj(map[string]any{
				"path": tools.StrParam("", "Path to write.", true),
			}, []string{"path"}),
			Handler: func(_ *tools.Context, _ map[string]any) tools.Observation {
				*p.called = append(*p.called, "write_file")
				return tools.OK("write_file", "wrote", nil)
			},
			Effect: tools.EffectWrite,
		},
	}, nil
}

type parallelStopProvider struct {
	started chan<- string
}

func (p parallelStopProvider) Specs(*tools.Context) ([]tools.Spec, error) {
	return []tools.Spec{
		{
			Name: "ask_user",
			Parameters: tools.Obj(map[string]any{
				"question": tools.StrParam("", "Question for the user.", true),
			}, []string{"question"}),
			Handler: func(_ *tools.Context, args map[string]any) tools.Observation {
				p.started <- "ask_user"
				obs := tools.OK("ask_user", "user input requested", map[string]any{
					"question": strings.TrimSpace(tools.StringArg(args, "question", "")),
				})
				obs["_subagent_stop"] = "needs_user_input"
				return obs
			},
			Effect: tools.EffectRead,
		},
		{
			Name:       "read_context",
			Parameters: tools.Obj(map[string]any{}, nil),
			Handler: func(_ *tools.Context, _ map[string]any) tools.Observation {
				p.started <- "read_context"
				return tools.OK("read_context", "read", nil)
			},
			Effect: tools.EffectRead,
		},
	}, nil
}

func malformedToolCallResponse() map[string]any {
	return map[string]any{"choices": []any{map[string]any{"message": map[string]any{
		"content": nil,
		"tool_calls": []any{map[string]any{
			"id":       "call_bad",
			"function": map[string]any{"arguments": `{}`},
		}},
	}}}}
}

func toolMessageIDs(messages []map[string]any) []string {
	ids := []string{}
	for _, message := range messages {
		if message["role"] != "tool" {
			continue
		}
		ids = append(ids, stringFromAny(message["tool_call_id"]))
	}
	return ids
}
