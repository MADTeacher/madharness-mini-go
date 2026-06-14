package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAskWritesContextReportToTrace(t *testing.T) {
	cfg := testAgentConfig(t)
	cfg.Data.APIKey = "token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()
	cfg.Data.BaseURL = server.URL

	result, tracePath, err := Ask("hello", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Fatalf("result = %q", result)
	}
	events := readTraceEvents(t, tracePath)
	found := false
	for _, event := range events {
		if event["event"] != "model_call_started" {
			continue
		}
		report, ok := event["context_report"].(map[string]any)
		if !ok {
			t.Fatalf("context_report missing: %+v", event)
		}
		if _, ok := report["request_tokens_estimate"].(float64); !ok {
			t.Fatalf("bad context_report: %+v", report)
		}
		found = true
	}
	if !found {
		t.Fatal("model_call_started event not written")
	}
}

func TestRunClipsLargeToolOutputBeforeNextModelCall(t *testing.T) {
	cfg := testAgentConfig(t)
	cfg.Data.ContextMaxTokens = 5000
	if err := os.WriteFile(filepath.Join(cfg.Root, "huge.txt"), []byte(strings.Repeat("x", 5000)), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &sequenceClient{responses: []map[string]any{
		{"choices": []any{map[string]any{"message": map[string]any{
			"content": nil,
			"tool_calls": []any{map[string]any{
				"id":       "call_1",
				"function": map[string]any{"name": "read_file", "arguments": `{"path":"huge.txt"}`},
			}},
		}}}},
		{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}}

	result, _, err := runWithClientOptions("read huge file", cfg, client, RunOptions{OrchestrationMode: "off"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "done" || len(client.seen) != 2 {
		t.Fatalf("result=%q seen=%d", result, len(client.seen))
	}
	secondRequest, _ := json.Marshal(client.seen[1])
	if !strings.Contains(string(secondRequest), "context clipped") {
		t.Fatalf("second request was not context-clipped: %s", secondRequest)
	}
	if strings.Contains(string(secondRequest), strings.Repeat("x", 1000)) {
		t.Fatalf("second request still contains huge tool output")
	}
}
