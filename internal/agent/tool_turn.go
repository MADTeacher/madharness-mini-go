package agent

import (
	"errors"

	"github.com/MADTeacher/madharness-mini-go/internal/agent/turnexec"
	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/events"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

func prepareToolTasks(
	registry *tools.Registry,
	calls []any,
	bus *events.Bus,
	kind string,
	turn int,
	tr *trace.Trace,
	parentSpanID string,
) []turnexec.Task {
	tasks := make([]turnexec.Task, 0, len(calls))
	for index, call := range calls {
		tasks = append(tasks, prepareToolTask(registry, index, call, bus, kind, turn, tr, parentSpanID))
	}
	return tasks
}

func prepareToolTask(
	registry *tools.Registry,
	index int,
	call any,
	bus *events.Bus,
	kind string,
	turn int,
	tr *trace.Trace,
	parentSpanID string,
) turnexec.Task {
	callID := "tool_call"
	callMap, ok := call.(map[string]any)
	if !ok {
		span := startToolCallSpan(tr, parentSpanID, turn, index, "", callID)
		return turnexec.Task{
			Index:        index,
			Call:         map[string]any{"id": "tool_call"},
			Name:         "tool_call",
			Args:         map[string]any{},
			Effect:       tools.EffectExclusive,
			SpanID:       span.ID(),
			EndSpan:      span.End,
			Observation:  tools.Fail("tool_call", "invalid tool call: invalid shape"),
			RecordResult: false,
		}
	}
	callID = stringFromAny(callMap["id"])
	name, args, err := tools.ParseToolArgs(callMap)
	if err == nil && name == "" {
		err = errors.New("missing function name")
	}
	span := startToolCallSpan(tr, parentSpanID, turn, index, name, callID)
	if err != nil {
		return turnexec.Task{
			Index:        index,
			Call:         callMap,
			Name:         "tool_call",
			Args:         map[string]any{},
			Effect:       tools.EffectExclusive,
			SpanID:       span.ID(),
			EndSpan:      span.End,
			Observation:  tools.Fail("tool_call", "invalid tool call: "+err.Error()),
			RecordResult: toolCallHasHistoryPeer(callMap),
		}
	}
	decision := publishEvent(bus, events.Event{
		Name:   "before_tool_call",
		Kind:   kind,
		SpanID: span.ID(),
		HookData: map[string]any{
			"turn":    turn,
			"call_id": callID,
			"tool":    name,
			"args":    args,
		},
	})
	if !decision.OK {
		return turnexec.Task{
			Index:        index,
			Call:         callMap,
			Name:         name,
			Args:         args,
			Effect:       tools.EffectExclusive,
			SpanID:       span.ID(),
			EndSpan:      span.End,
			RecordResult: true,
			Observation: tools.Fail(name, "blocked by hook: "+decision.Block, map[string]any{
				"hook_blocked": true,
			}),
		}
	}
	return turnexec.Task{
		Index:        index,
		Call:         callMap,
		Name:         name,
		Args:         args,
		Effect:       registry.Effect(name),
		Runnable:     true,
		RecordResult: true,
		SpanID:       span.ID(),
		EndSpan:      span.End,
	}
}

func toolCallHasHistoryPeer(call map[string]any) bool {
	function, ok := call["function"].(map[string]any)
	if !ok {
		return false
	}
	name, ok := function["name"].(string)
	return ok && name != ""
}

func startToolCallSpan(tr *trace.Trace, parentSpanID string, turn int, index int, tool string, callID string) *trace.Span {
	if tr == nil {
		return &trace.Span{}
	}
	fields := map[string]any{
		"turn":       turn,
		"tool_index": index,
		"call_id":    callID,
	}
	if tool != "" {
		fields["tool"] = tool
	}
	return tr.StartSpan("tool_call", parentSpanID, fields)
}

func commitToolResults(
	results []turnexec.Result,
	context *agentcontext.Manager,
	tr *trace.Trace,
	options loopOptions,
	kind string,
	turn int,
) (loopResult, bool) {
	var stopped loopResult
	stop := false
	for _, result := range results {
		obs := result.Observation
		task := result.Task
		subagentStop := stringObservationField(obs, "_subagent_stop")
		if subagentStop != "" {
			delete(obs, "_subagent_stop")
		}
		applyHiddenObservationEffects(context, tr, obs)
		publishEvent(options.Events, events.Event{
			Name:   "after_tool_call",
			Kind:   kind,
			SpanID: task.SpanID,
			HookData: map[string]any{
				"turn":        turn,
				"tool":        task.Name,
				"args":        task.Args,
				"observation": obs,
			},
			TraceName: "tool_observation",
			TraceData: map[string]any{
				"tool":        task.Name,
				"args":        task.Args,
				"observation": obs,
			},
		})
		endToolSpan(task, obs)
		if task.RecordResult {
			context.RecordToolResult(task.Call, obs, result.Followups)
		}
		if !stop && options.StopOnUserInput && subagentStop == "needs_user_input" {
			stopped = stopForSubagentQuestion(options, kind, turn, obs)
			stop = true
		}
		if !stop && isParentUserInputRequest(obs) {
			stopped = stopForParentQuestion(options, kind, turn, tr, obs)
			stop = true
		}
	}
	return stopped, stop
}

func endToolSpan(task turnexec.Task, obs tools.Observation) {
	if task.EndSpan == nil {
		return
	}
	status := "ok"
	if obs["ok"] == false {
		status = "error"
	}
	task.EndSpan(status, map[string]any{
		"tool":    task.Name,
		"summary": stringObservationField(obs, "summary"),
	})
}

func stopForSubagentQuestion(
	options loopOptions,
	kind string,
	turn int,
	obs tools.Observation,
) loopResult {
	question := stringObservationField(obs, "question")
	return loopResult{Status: "needs_user_input", Result: question, Turns: turn + 1, Observation: obs}
}

func stopForParentQuestion(
	options loopOptions,
	kind string,
	turn int,
	tr *trace.Trace,
	obs tools.Observation,
) loopResult {
	result := renderUserInputRequest(obs)
	_ = tr.Write("user_input_requested", map[string]any{
		"subagent":            stringObservationField(obs, "subagent"),
		"question":            stringObservationField(obs, "question"),
		"options":             obs["options"],
		"reason":              stringObservationField(obs, "reason"),
		"subagent_trace_id":   stringObservationField(obs, "subagent_trace_id"),
		"subagent_trace_path": stringObservationField(obs, "subagent_trace_path"),
	})
	return loopResult{Status: "needs_user_input", Result: result, Turns: turn + 1, Observation: obs}
}
