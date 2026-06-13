package agentcontext

import (
	"encoding/json"
	"fmt"
)

const (
	// Полный assistant-текст быстро начинает дублировать уже сделанную работу.
	assistantContentLimit = 8000
	// Если assistant вызывает tool, его content обычно служебный.
	assistantToolContentLimit = 2000
)

func sanitizeAssistantMessage(message map[string]any) map[string]any {
	toolCalls := sanitizeToolCalls(message["tool_calls"])
	content := message["content"]
	limit := assistantContentLimit
	if len(toolCalls) > 0 {
		limit = assistantToolContentLimit
	}
	stored := map[string]any{"role": "assistant"}
	switch value := content.(type) {
	case string:
		stored["content"] = clipText(value, limit)
	case nil:
		if len(toolCalls) > 0 {
			stored["content"] = nil
		} else {
			stored["content"] = ""
		}
	default:
		stored["content"] = value
	}
	if len(toolCalls) > 0 {
		stored["tool_calls"] = toolCalls
	}
	return stored
}

func sanitizeToolCalls(raw any) []map[string]any {
	calls := asSlice(raw)
	sanitized := []map[string]any{}
	for _, item := range calls {
		call, ok := item.(map[string]any)
		if !ok {
			continue
		}
		function, ok := call["function"].(map[string]any)
		if !ok {
			continue
		}
		name, ok := function["name"].(string)
		if !ok || name == "" {
			continue
		}
		arguments := "{}"
		switch value := function["arguments"].(type) {
		case string:
			arguments = value
		case nil:
			arguments = "{}"
		default:
			rawArgs, err := json.Marshal(value)
			if err == nil {
				arguments = string(rawArgs)
			}
		}
		callType, _ := call["type"].(string)
		if callType == "" {
			callType = "function"
		}
		id, _ := call["id"].(string)
		if id == "" {
			id = name
		}
		sanitized = append(sanitized, map[string]any{
			"id":   id,
			"type": callType,
			"function": map[string]any{
				"name":      name,
				"arguments": arguments,
			},
		})
	}
	return sanitized
}

func asSlice(raw any) []any {
	switch value := raw.(type) {
	case []any:
		return value
	case []map[string]any:
		out := make([]any, 0, len(value))
		for _, item := range value {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func toolCallName(call map[string]any, observation map[string]any) string {
	if function, ok := call["function"].(map[string]any); ok {
		if name, ok := function["name"].(string); ok && name != "" {
			return name
		}
	}
	if name, ok := observation["tool"].(string); ok && name != "" {
		return name
	}
	return fmt.Sprint("tool_call")
}
