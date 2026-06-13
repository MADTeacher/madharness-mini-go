package agent

import (
	"fmt"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

func runModelLoop(
	client chatClient,
	tr *trace.Trace,
	context *agentcontext.Manager,
	registry *tools.Registry,
	maxTurns int,
) (string, error) {
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
			return "", err
		}
		_ = tr.Write("model_call_started", map[string]any{
			"turn":           turn,
			"tools_count":    len(toolSchemas),
			"context_report": context.Report(),
		})
		raw, err := callModelWithRateLimitRetry(client, tr, messages, toolSchemas, map[string]any{"turn": turn})
		if err != nil {
			_ = tr.Write("model_error", map[string]any{"turn": turn, "error": err.Error()})
			_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
			return "", err
		}
		message, err := responseMessage(raw)
		if err != nil {
			_ = tr.Write("model_error", map[string]any{"turn": turn, "error": err.Error()})
			_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
			return "", err
		}
		_ = tr.Write("model_call_finished", map[string]any{"turn": turn, "message": message})
		context.RecordAssistant(message)
		calls := toolCalls(message)
		if len(calls) == 0 {
			result := messageContent(message)
			_ = tr.Write("session_end", map[string]any{"result": result})
			return result, nil
		}
		for _, call := range calls {
			name, args, obs, followups, callMap := executeToolCall(registry, call)
			applyHiddenObservationEffects(context, tr, obs)
			_ = tr.Write("tool_observation", map[string]any{
				"tool":        name,
				"args":        args,
				"observation": obs,
			})
			context.RecordToolResult(callMap, obs, followups)
		}
	}
	result := "Agent stopped: max_turns exceeded."
	_ = tr.Write("session_end", map[string]any{"result": result})
	return result, nil
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

func executeToolCall(registry *tools.Registry, call any) (string, map[string]any, tools.Observation, []map[string]any, map[string]any) {
	callMap, ok := call.(map[string]any)
	if !ok {
		obs := tools.Fail("tool_call", "invalid tool call: invalid shape")
		return "tool_call", map[string]any{}, obs, nil, map[string]any{"id": "tool_call"}
	}
	name, args, err := tools.ParseToolArgs(callMap)
	if err != nil {
		obs := tools.Fail("tool_call", "invalid tool call: "+err.Error())
		return "tool_call", map[string]any{}, obs, nil, callMap
	}
	obs, followups := registry.CallWithFollowups(name, args)
	return name, args, obs, followups, callMap
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
