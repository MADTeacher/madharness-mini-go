package agentcontext

import (
	"encoding/json"
	"strings"
	"testing"
)

func testToolCall(id string, name string) map[string]any {
	return map[string]any{
		"id":       id,
		"function": map[string]any{"name": name, "arguments": map[string]any{"path": "README.md"}},
	}
}

func TestRecordAssistantStripsVendorFieldsFromHistory(t *testing.T) {
	ctx := NewManager("task", Options{MaxTokens: 20000, KeepRecentTurns: 3})
	ctx.RecordAssistant(map[string]any{
		"role":              "assistant",
		"content":           "I will call a tool.",
		"reasoning":         "secret provider reasoning",
		"reasoning_details": []any{map[string]any{"text": "very long chain"}},
		"refusal":           nil,
		"tool_calls": []any{map[string]any{
			"id":    "call_extra",
			"type":  "function",
			"index": 0,
			"function": map[string]any{
				"name":      "demo",
				"arguments": map[string]any{"path": "README.md"},
				"extra":     "drop me",
			},
			"provider_field": "drop me too",
		}},
	})

	messages, err := ctx.Messages(nil)
	if err != nil {
		t.Fatal(err)
	}
	rendered := mustJSON(t, messages)
	if strings.Contains(rendered, "reasoning") ||
		strings.Contains(rendered, "refusal") ||
		strings.Contains(rendered, "provider_field") {
		t.Fatalf("vendor fields leaked into messages: %s", rendered)
	}
	if !strings.Contains(rendered, `"tool_calls"`) || !strings.Contains(rendered, `\"path\":\"README.md\"`) {
		t.Fatalf("sanitized tool call missing expected data: %s", rendered)
	}
}

func TestRecordAssistantClipsLargeContent(t *testing.T) {
	ctx := NewManager("task", Options{MaxTokens: 20000, KeepRecentTurns: 3})
	ctx.RecordAssistant(map[string]any{"role": "assistant", "content": strings.Repeat("x", 20000)})

	messages, err := ctx.Messages(nil)
	if err != nil {
		t.Fatal(err)
	}
	rendered := mustJSON(t, messages)
	if !strings.Contains(rendered, "context clipped") || len(rendered) >= 12000 {
		t.Fatalf("large assistant content was not clipped: len=%d", len(rendered))
	}
}

func TestLockedFragmentsAndTaskSurviveSmallBudget(t *testing.T) {
	ctx := NewManager("do the task", Options{MaxTokens: 90, KeepRecentTurns: 0})
	ctx.AddFragment(Fragment{ID: "system", Source: "test", Text: "system rules stay visible", Priority: 0})
	for index := 0; index < 5; index++ {
		ctx.RecordAssistant(map[string]any{"role": "assistant", "content": strings.Repeat("x", 100)})
	}

	messages, err := ctx.Messages(nil)
	if err != nil {
		t.Fatal(err)
	}
	if messages[0]["role"] != "system" || !strings.Contains(messages[0]["content"].(string), "system rules stay visible") {
		t.Fatalf("system fragment disappeared: %+v", messages)
	}
	if messages[1]["content"] != "do the task" {
		t.Fatalf("task disappeared: %+v", messages)
	}
	stats := ctx.Stats()
	if stats["truncated"] != true || intFromTest(stats["dropped_entries"]) == 0 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestToolTurnIsRemovedAtomically(t *testing.T) {
	ctx := NewManager("task", Options{MaxTokens: 100, KeepRecentTurns: 1})
	ctx.AddFragment(Fragment{ID: "system", Source: "test", Text: "system"})
	oldCall := testToolCall("old_call", "demo")
	ctx.RecordAssistant(map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{oldCall}})
	ctx.RecordToolResult(oldCall, map[string]any{
		"ok": true, "tool": "demo", "summary": "old result",
		"stdout": "old unique output " + strings.Repeat("x", 1000),
	}, nil)
	ctx.RecordAssistant(map[string]any{"role": "assistant", "content": "new answer"})

	messages, err := ctx.Messages(nil)
	if err != nil {
		t.Fatal(err)
	}
	rendered := mustJSON(t, messages)
	if strings.Contains(rendered, "old_call") || strings.Contains(rendered, "old unique output") {
		t.Fatalf("old tool turn was not removed atomically: %s", rendered)
	}
	if !strings.Contains(rendered, "new answer") {
		t.Fatalf("new answer missing: %s", rendered)
	}
}

func TestFollowupImageIsNotStoredInsideToolObservation(t *testing.T) {
	ctx := NewManager("inspect", Options{MaxTokens: 20000, KeepRecentTurns: 3})
	call := testToolCall("image_call", "read_image")
	followup := []map[string]any{{
		"role": "user",
		"content": []map[string]any{
			{"type": "text", "text": "Image from read_image is attached"},
			{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,abc", "detail": "auto"}},
		},
	}}

	ctx.RecordAssistant(map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{call}})
	ctx.RecordToolResult(call, map[string]any{
		"ok": true, "tool": "read_image", "summary": "read image metadata", "attached": true,
	}, followup)

	messages, err := ctx.Messages(nil)
	if err != nil {
		t.Fatal(err)
	}
	toolMessages := 0
	imageMessages := 0
	for _, message := range messages {
		if message["role"] == "tool" {
			toolMessages++
			if strings.Contains(message["content"].(string), "data:image") {
				t.Fatalf("tool observation contains image payload: %+v", message)
			}
		}
		if message["role"] == "user" {
			if _, ok := message["content"].([]any); ok {
				imageMessages++
			}
		}
	}
	if toolMessages != 1 || imageMessages != 1 {
		t.Fatalf("toolMessages=%d imageMessages=%d messages=%+v", toolMessages, imageMessages, messages)
	}
}

func TestReportDescribesFragmentsAndToolClippingWithoutContent(t *testing.T) {
	ctx := NewManager("task", Options{MaxTokens: 400, KeepRecentTurns: 3})
	ctx.AddFragment(Fragment{ID: "system", Source: "test-system", Text: "system rules", Priority: 0})
	call := testToolCall("call_clip", "run_shell")
	ctx.RecordAssistant(map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{call}})
	ctx.RecordToolResult(call, map[string]any{
		"ok": true, "tool": "run_shell", "summary": "ran command", "stdout": strings.Repeat("x", 2000),
	}, nil)

	if _, err := ctx.Messages(nil); err != nil {
		t.Fatal(err)
	}
	report := ctx.Report()
	rendered := mustJSON(t, report)
	if strings.Contains(rendered, strings.Repeat("x", 100)) || strings.Contains(rendered, "system rules") {
		t.Fatalf("report leaked content: %s", rendered)
	}
	if intFromTest(report["request_tokens_estimate"]) == 0 {
		t.Fatalf("report has no token estimate: %+v", report)
	}
	history := report["history"].(map[string]any)
	if lenFromTest(history["clipped_tool_messages"]) != 1 {
		t.Fatalf("history report = %+v", history)
	}
}

func TestReportCountsToolSchemasInRequestBudget(t *testing.T) {
	ctx := NewManager("task", Options{MaxTokens: 40, KeepRecentTurns: 3})
	tools := []map[string]any{{
		"type": "function",
		"function": map[string]any{
			"name":        "large_tool",
			"description": "tool schema " + strings.Repeat("x", 500),
			"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}}

	_, err := ctx.Messages(tools)
	if err == nil || !strings.Contains(err.Error(), "context budget exceeded") {
		t.Fatalf("expected context budget error, got %v", err)
	}
	report := ctx.Report()
	if intFromTest(report["tools_tokens_estimate"]) == 0 ||
		intFromTest(report["request_tokens_estimate"]) <= intFromTest(report["messages_tokens_estimate"]) ||
		report["hard_limit_exceeded"] != true {
		t.Fatalf("report = %+v", report)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func intFromTest(value any) int {
	switch item := value.(type) {
	case int:
		return item
	case float64:
		return int(item)
	default:
		return 0
	}
}

func lenFromTest(value any) int {
	switch items := value.(type) {
	case []any:
		return len(items)
	case []map[string]any:
		return len(items)
	default:
		return 0
	}
}
