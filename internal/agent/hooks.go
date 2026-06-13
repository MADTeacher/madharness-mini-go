package agent

import (
	"fmt"

	"github.com/MADTeacher/madharness-mini-go/internal/hooks"
)

func emitHook(manager *hooks.Manager, name string, kind string, data map[string]any) hooks.Decision {
	if manager == nil {
		return hooks.Allow()
	}
	return manager.Emit(name, kind, data)
}

func emitSessionError(manager *hooks.Manager, kind string, err error, turn any) {
	data := map[string]any{
		"error_type": errorType(err),
		"message":    err.Error(),
	}
	if turn != nil {
		data["turn"] = turn
	}
	emitHook(manager, "session_error", kind, data)
}

func modelMessageSummary(message map[string]any) map[string]any {
	calls := toolCalls(message)
	tools := []map[string]any{}
	for _, call := range calls {
		callMap, ok := call.(map[string]any)
		if !ok {
			continue
		}
		function, _ := callMap["function"].(map[string]any)
		tools = append(tools, map[string]any{
			"id":   stringFromAny(callMap["id"]),
			"name": stringFromAny(function["name"]),
		})
	}
	return map[string]any{
		"content_preview":  truncateForHook(messageContent(message), 1000),
		"tool_calls_count": len(calls),
		"tools":            tools,
	}
}

func errorType(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%T", err)
}

func stringFromAny(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func truncateForHook(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}
