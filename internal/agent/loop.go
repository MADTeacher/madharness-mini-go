package agent

import (
	"fmt"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/agent/turnexec"
	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/hooks"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

type loopOptions struct {
	StopOnUserInput      bool
	Hooks                *hooks.Manager
	Kind                 string
	MaxParallelToolCalls int
}

type loopResult struct {
	Status      string
	Result      string
	Turns       int
	Observation tools.Observation
}

func runModelLoop(
	client chatClient,
	tr *trace.Trace,
	context *agentcontext.Manager,
	registry *tools.Registry,
	maxTurns int,
	options loopOptions,
) (loopResult, error) {
	kind := options.Kind
	if kind == "" {
		kind = "run"
	}
	for turn := 0; turn < maxTurns; turn++ {
		toolSchemas := registry.Schemas()
		messages, err := context.Messages(toolSchemas)
		if err != nil {
			_ = tr.Write("context_error", map[string]any{
				"turn":           turn,
				"error":          err.Error(),
				"context_report": safeContextReport(context),
			})
			_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
			emitSessionError(options.Hooks, kind, err, turn)
			return loopResult{}, err
		}
		contextReport := context.Report()
		_ = tr.Write("model_call_started", map[string]any{
			"turn":           turn,
			"tools_count":    len(toolSchemas),
			"context_report": contextReport,
		})
		emitHook(options.Hooks, "before_model_call", kind, map[string]any{
			"turn":           turn,
			"tools_count":    len(toolSchemas),
			"context_report": contextReport,
		})
		raw, err := callModelWithRateLimitRetry(client, tr, messages, toolSchemas, map[string]any{"turn": turn})
		if err != nil {
			_ = tr.Write("model_error", map[string]any{"turn": turn, "error": err.Error()})
			_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
			emitSessionError(options.Hooks, kind, err, turn)
			return loopResult{}, err
		}
		message, err := responseMessage(raw)
		if err != nil {
			_ = tr.Write("model_error", map[string]any{"turn": turn, "error": err.Error()})
			_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
			emitSessionError(options.Hooks, kind, err, turn)
			return loopResult{}, err
		}
		_ = tr.Write("model_call_finished", map[string]any{"turn": turn, "message": message})
		emitHook(options.Hooks, "after_model_call", kind, map[string]any{
			"turn":    turn,
			"message": modelMessageSummary(message),
		})
		context.RecordAssistant(message)
		calls := toolCalls(message)
		if len(calls) == 0 {
			result := messageContent(message)
			_ = tr.Write("session_end", map[string]any{"result": result})
			emitHook(options.Hooks, "session_end", kind, map[string]any{
				"status":         "done",
				"turns":          turn + 1,
				"result_preview": truncateForHook(result, 1000),
			})
			return loopResult{Status: "done", Result: result, Turns: turn + 1}, nil
		}
		tasks := prepareToolTasks(registry, calls, options.Hooks, kind, turn)
		groups := turnexec.Plan(tasks)
		writeToolExecutionPlan(tr, turn, options.MaxParallelToolCalls, groups)
		results := turnexec.Execute(groups, options.MaxParallelToolCalls, func(task turnexec.Task) (tools.Observation, []map[string]any) {
			return registry.CallWithFollowups(task.Name, task.Args)
		})
		if result, stopped := commitToolResults(results, context, tr, options, kind, turn); stopped {
			return result, nil
		}
	}
	result := "Agent stopped: max_turns exceeded."
	_ = tr.Write("session_end", map[string]any{"result": result})
	emitHook(options.Hooks, "session_end", kind, map[string]any{
		"status":         "max_turns",
		"turns":          maxTurns,
		"result_preview": result,
	})
	return loopResult{Status: "max_turns", Result: result, Turns: maxTurns}, nil
}

func toolCalls(message map[string]any) []any {
	calls, ok := message["tool_calls"].([]any)
	if ok {
		return calls
	}
	mapped, ok := message["tool_calls"].([]map[string]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(mapped))
	for _, item := range mapped {
		out = append(out, item)
	}
	return out
}

func applyHiddenObservationEffects(context *agentcontext.Manager, tr *trace.Trace, observation tools.Observation) {
	rawFragments, hasFragments := observation["_context_fragments"]
	if hasFragments {
		delete(observation, "_context_fragments")
	}
	switch fragments := rawFragments.(type) {
	case []agentcontext.Fragment:
		for _, fragment := range fragments {
			context.AddFragment(fragment)
		}
	case []any:
		for _, raw := range fragments {
			if fragment, ok := raw.(agentcontext.Fragment); ok {
				context.AddFragment(fragment)
			}
		}
	}
	rawEvent, hasEvent := observation["_skill_event"]
	if hasEvent {
		delete(observation, "_skill_event")
	}
	if event, ok := rawEvent.(map[string]any); ok {
		_ = tr.Write("skill_activated", event)
	}
}

func safeContextReport(context *agentcontext.Manager) (report map[string]any) {
	defer func() {
		if recovered := recover(); recovered != nil {
			report = map[string]any{"error": fmt.Sprint(recovered)}
		}
	}()
	if context == nil {
		return map[string]any{"error": "context is nil"}
	}
	return context.Report()
}

func isParentUserInputRequest(observation tools.Observation) bool {
	return stringObservationField(observation, "tool") == "delegate_task" &&
		stringObservationField(observation, "status") == "needs_user_input" &&
		strings.TrimSpace(stringObservationField(observation, "question")) != ""
}

func renderUserInputRequest(observation tools.Observation) string {
	subagent := strings.TrimSpace(stringObservationField(observation, "subagent"))
	if subagent == "" {
		subagent = "subagent"
	}
	question := strings.TrimSpace(stringObservationField(observation, "question"))
	reason := strings.TrimSpace(stringObservationField(observation, "reason"))
	lines := []string{subagent + " просит уточнение:", "", question}
	options := observationOptions(observation["options"])
	if len(options) > 0 {
		lines = append(lines, "", "Варианты:")
		for index, option := range options {
			lines = append(lines, fmt.Sprintf("%d. %s", index+1, option))
		}
	}
	if reason != "" {
		lines = append(lines, "", "Причина: "+reason)
	}
	lines = append(lines, "", "Ответьте на вопрос и повторите команду `run` с выбранным решением в задаче.")
	return strings.Join(lines, "\n")
}

func stringObservationField(observation tools.Observation, key string) string {
	if value, ok := observation[key].(string); ok {
		return value
	}
	return ""
}

func observationOptions(raw any) []string {
	out := []string{}
	switch items := raw.(type) {
	case []string:
		out = append(out, items...)
	case []any:
		for _, item := range items {
			out = append(out, fmt.Sprint(item))
		}
	}
	return out
}
