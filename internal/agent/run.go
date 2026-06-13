package agent

import (
	"encoding/json"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/model"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/builtin"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

// Run запускает агентский цикл до финального ответа или max_turns.
func Run(task string, cfg *config.Config) (string, string, error) {
	tr, err := trace.New(cfg, "run")
	if err != nil {
		return "", "", err
	}
	client := model.New(cfg)
	registry, err := tools.NewRegistry(cfg, builtin.Provider{})
	if err != nil {
		return "", tr.Path, err
	}
	messages, err := BaseMessages(cfg, task)
	if err != nil {
		return "", tr.Path, err
	}
	for turn := 0; turn < cfg.Data.MaxTurns; turn++ {
		fields := map[string]any{"turn": turn, "tools_count": len(registry.Tools())}
		_ = tr.Write("model_call_started", fields)
		raw, err := callModelWithRateLimitRetry(client, tr, messages, registry.Schemas(), map[string]any{"turn": turn})
		if err != nil {
			_ = tr.Write("model_error", map[string]any{"turn": turn, "error": err.Error()})
			_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
			return "", tr.Path, err
		}
		message, err := responseMessage(raw)
		if err != nil {
			_ = tr.Write("model_error", map[string]any{"turn": turn, "error": err.Error()})
			_ = tr.Write("session_end", map[string]any{"result": "error: " + err.Error()})
			return "", tr.Path, err
		}
		_ = tr.Write("model_call_finished", map[string]any{"turn": turn, "message": message})
		messages = append(messages, message)
		calls := toolCalls(message)
		if len(calls) == 0 {
			result := messageContent(message)
			_ = tr.Write("session_end", map[string]any{"result": result})
			return result, tr.Path, nil
		}
		for _, call := range calls {
			name, args, obs := executeToolCall(registry, call)
			_ = tr.Write("tool_observation", map[string]any{
				"tool":        name,
				"args":        args,
				"observation": obs,
			})
			content, _ := json.Marshal(obs)
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": toolCallID(call, name),
				"content":      string(content),
			})
		}
	}
	result := "Agent stopped: max_turns exceeded."
	_ = tr.Write("session_end", map[string]any{"result": result})
	return result, tr.Path, nil
}

func toolCalls(message map[string]any) []any {
	calls, ok := message["tool_calls"].([]any)
	if !ok {
		return nil
	}
	return calls
}

func executeToolCall(registry *tools.Registry, call any) (string, map[string]any, tools.Observation) {
	callMap, ok := call.(map[string]any)
	if !ok {
		return "tool_call", map[string]any{}, tools.Fail("tool_call", "invalid tool call: invalid shape")
	}
	name, args, err := tools.ParseToolArgs(callMap)
	if err != nil {
		return "tool_call", map[string]any{}, tools.Fail("tool_call", "invalid tool call: "+err.Error())
	}
	return name, args, registry.Call(name, args)
}

func toolCallID(call any, fallback string) string {
	callMap, ok := call.(map[string]any)
	if !ok {
		return fallback
	}
	if id, ok := callMap["id"].(string); ok && id != "" {
		return id
	}
	return fallback
}
