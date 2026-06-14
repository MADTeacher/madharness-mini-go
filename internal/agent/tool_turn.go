package agent

import (
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
) []turnexec.Task {
	tasks := make([]turnexec.Task, 0, len(calls))
	for index, call := range calls {
		tasks = append(tasks, prepareToolTask(registry, index, call, bus, kind, turn))
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
) turnexec.Task {
	callMap, ok := call.(map[string]any)
	if !ok {
		return turnexec.Task{
			Index:       index,
			Call:        map[string]any{"id": "tool_call"},
			Name:        "tool_call",
			Args:        map[string]any{},
			Effect:      tools.EffectExclusive,
			Observation: tools.Fail("tool_call", "invalid tool call: invalid shape"),
		}
	}
	name, args, err := tools.ParseToolArgs(callMap)
	if err != nil {
		return turnexec.Task{
			Index:       index,
			Call:        callMap,
			Name:        "tool_call",
			Args:        map[string]any{},
			Effect:      tools.EffectExclusive,
			Observation: tools.Fail("tool_call", "invalid tool call: "+err.Error()),
		}
	}
	decision := publishEvent(bus, events.Event{
		Name: "before_tool_call",
		Kind: kind,
		HookData: map[string]any{
			"turn":    turn,
			"call_id": stringFromAny(callMap["id"]),
			"tool":    name,
			"args":    args,
		},
	})
	if !decision.OK {
		return turnexec.Task{
			Index:  index,
			Call:   callMap,
			Name:   name,
			Args:   args,
			Effect: tools.EffectExclusive,
			Observation: tools.Fail(name, "blocked by hook: "+decision.Block, map[string]any{
				"hook_blocked": true,
			}),
		}
	}
	return turnexec.Task{
		Index:    index,
		Call:     callMap,
		Name:     name,
		Args:     args,
		Effect:   registry.Effect(name),
		Runnable: true,
	}
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
	stop := turnexec.Commit(results, func(result turnexec.Result) bool {
		obs := result.Observation
		task := result.Task
		subagentStop := stringObservationField(obs, "_subagent_stop")
		if subagentStop != "" {
			delete(obs, "_subagent_stop")
		}
		applyHiddenObservationEffects(context, tr, obs)
		publishEvent(options.Events, events.Event{
			Name: "after_tool_call",
			Kind: kind,
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
		if options.StopOnUserInput && subagentStop == "needs_user_input" {
			stopped = stopForSubagentQuestion(options, kind, turn, obs)
			return true
		}
		context.RecordToolResult(task.Call, obs, result.Followups)
		if isParentUserInputRequest(obs) {
			stopped = stopForParentQuestion(options, kind, turn, tr, obs)
			return true
		}
		return false
	})
	return stopped, stop
}

func stopForSubagentQuestion(
	options loopOptions,
	kind string,
	turn int,
	obs tools.Observation,
) loopResult {
	question := stringObservationField(obs, "question")
	result := "needs_user_input: " + question
	publishSessionEnd(options.Events, kind, result, map[string]any{
		"status":         "needs_user_input",
		"turns":          turn + 1,
		"result_preview": truncateForHook(question, 1000),
	})
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
	publishSessionEnd(options.Events, kind, result, map[string]any{
		"status":         "needs_user_input",
		"turns":          turn + 1,
		"result_preview": truncateForHook(result, 1000),
	})
	return loopResult{Status: "needs_user_input", Result: result, Turns: turn + 1, Observation: obs}
}
