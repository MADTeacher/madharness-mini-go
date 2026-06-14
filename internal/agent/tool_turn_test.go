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
