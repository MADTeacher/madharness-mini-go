package agentcontext

import (
	"encoding/json"
	"fmt"
)

// Консервативная цена токена без tokenizer конкретной модели.
const TokenEstimateBytesPerToken = 3

// EstimateTokens оценивает размер payload через компактный JSON.
func EstimateTokens(payload any) int {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0
	}
	return (len(raw) + TokenEstimateBytesPerToken - 1) / TokenEstimateBytesPerToken
}

func estimateRequestTokens(messages []map[string]any, tools []map[string]any) map[string]int {
	payload := map[string]any{"messages": messages}
	messagesTokens := EstimateTokens(messages)
	toolsTokens := 0
	if len(tools) > 0 {
		payload["tools"] = tools
		toolsTokens = EstimateTokens(tools)
	}
	return map[string]int{
		"messages_tokens_estimate": messagesTokens,
		"tools_tokens_estimate":    toolsTokens,
		"request_tokens_estimate":  EstimateTokens(payload),
	}
}

func clipToolMessages(entries []HistoryEntry, limit int) []map[string]any {
	clipped := []map[string]any{}
	for entryIndex := range entries {
		for messageIndex := range entries[entryIndex].Messages {
			message := entries[entryIndex].Messages[messageIndex]
			if message["role"] != "tool" {
				continue
			}
			content, ok := message["content"].(string)
			if !ok || len(content) <= limit {
				continue
			}
			shortened := clipToolContent(content, limit)
			message["content"] = shortened
			clipped = append(clipped, map[string]any{
				"tool_call_id": fmt.Sprint(message["tool_call_id"]),
				"before_chars": len(content),
				"after_chars":  len(shortened),
				"saved_chars":  len(content) - len(shortened),
			})
		}
	}
	return clipped
}

func clipToolContent(content string, limit int) string {
	excerpt := clipText(content, maxInt(40, limit/2))
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return clipText(content, limit)
	}
	compact := map[string]any{"_context_truncated": true, "content_excerpt": excerpt}
	for _, key := range []string{"ok", "tool", "summary"} {
		if value, ok := payload[key]; ok {
			compact[key] = value
		}
	}
	rendered, err := json.Marshal(compact)
	if err != nil || len(rendered) > limit {
		return clipText(content, limit)
	}
	return string(rendered)
}

func clipText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	marker := fmt.Sprintf("\n...[context clipped %d chars]", len(text)-limit)
	keep := maxInt(limit-len(marker), 0)
	return text[:keep] + marker
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
