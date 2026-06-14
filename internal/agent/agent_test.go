package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/model"
	"github.com/MADTeacher/madharness-mini-go/internal/prompt"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x00\x00\x00\x00\x00")

type fakeClient struct {
	response map[string]any
	errs     []error
	calls    int
}

func (f *fakeClient) Chat(messages []map[string]any, tools []map[string]any) (map[string]any, error) {
	index := f.calls
	f.calls++
	if index < len(f.errs) && f.errs[index] != nil {
		return nil, f.errs[index]
	}
	return f.response, nil
}

type sequenceClient struct {
	responses []map[string]any
	seen      [][]map[string]any
	toolsSeen [][]map[string]any
}

func (s *sequenceClient) Chat(messages []map[string]any, tools []map[string]any) (map[string]any, error) {
	raw, _ := json.Marshal(messages)
	copied := []map[string]any{}
	_ = json.Unmarshal(raw, &copied)
	s.seen = append(s.seen, copied)
	toolsRaw, _ := json.Marshal(tools)
	toolsCopied := []map[string]any{}
	_ = json.Unmarshal(toolsRaw, &toolsCopied)
	s.toolsSeen = append(s.toolsSeen, toolsCopied)
	index := len(s.seen) - 1
	return s.responses[index], nil
}

func TestBaseMessagesLoadsSystemPrompt(t *testing.T) {
	cfg := testAgentConfig(t)
	messages, err := BaseMessages(cfg, "Return a short greeting")
	if err != nil {
		t.Fatal(err)
	}
	system, err := prompt.Load("system")
	if err != nil {
		t.Fatal(err)
	}
	if messages[0]["content"] != system || messages[1]["content"] != "Return a short greeting" {
		t.Fatalf("messages = %+v", messages)
	}
}

func TestBaseMessagesAppendsRootAgentsMD(t *testing.T) {
	cfg := testAgentConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "AGENTS.md"), []byte("Use project test command.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	messages, err := BaseMessages(cfg, "Return a short greeting")
	if err != nil {
		t.Fatal(err)
	}
	content := messages[0]["content"].(string)
	if !strings.Contains(content, "# Project instructions") || !strings.Contains(content, "Use project test command.") {
		t.Fatalf("system content = %q", content)
	}
}

func TestCallModelRetriesOnceAfterShortRateLimit(t *testing.T) {
	cfg := testAgentConfig(t)
	tr, err := trace.New(cfg, "ask")
	if err != nil {
		t.Fatal(err)
	}
	oldSleep := sleep
	sleep = func(time.Duration) {}
	defer func() { sleep = oldSleep }()
	client := &fakeClient{
		errs: []error{&model.RateLimitError{
			Status: 429, RetryAfter: "1", RetryAfterSeconds: 1, HasRetryAfter: true,
		}},
		response: map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "ok"}}}},
	}
	raw, err := callModelWithRateLimitRetry(client, tr, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 2 || raw == nil {
		t.Fatalf("calls=%d raw=%v", client.calls, raw)
	}
	events := readTraceEvents(t, tr.Path)
	found := false
	for _, event := range events {
		if event["event"] == "model_rate_limit_retry" {
			found = true
		}
	}
	if !found {
		t.Fatal("model_rate_limit_retry event not written")
	}
}

func TestCallModelDoesNotRetryLongRateLimit(t *testing.T) {
	cfg := testAgentConfig(t)
	tr, err := trace.New(cfg, "ask")
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{errs: []error{&model.RateLimitError{
		Status: 429, RetryAfter: "61", RetryAfterSeconds: 61, HasRetryAfter: true,
	}}}
	if _, err := callModelWithRateLimitRetry(client, tr, nil, nil, nil); err == nil {
		t.Fatal("expected rate limit error")
	}
	if client.calls != 1 {
		t.Fatalf("calls = %d", client.calls)
	}
}

func TestRunKeepsImageTextOnlyWhenVisionIsDisabled(t *testing.T) {
	cfg := testAgentConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "shot.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	client := imageSequenceClient()

	result, tracePath, err := runWithClient("inspect screenshot", cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	if result != "done" || len(client.seen) != 2 {
		t.Fatalf("result=%q seen=%d", result, len(client.seen))
	}
	secondRequest, _ := json.Marshal(client.seen[1])
	assertNoImagePayload(t, string(secondRequest))
	traceText := readFileText(t, tracePath)
	assertNoImagePayload(t, traceText)
}

func TestRunAttachesImageWhenVisionIsEnabled(t *testing.T) {
	cfg := testAgentConfig(t)
	cfg.Data.SupportsImageInput = true
	if err := os.WriteFile(filepath.Join(cfg.Root, "shot.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	client := imageSequenceClient()

	result, tracePath, err := runWithClient("inspect screenshot", cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	if result != "done" || len(client.seen) != 2 {
		t.Fatalf("result=%q seen=%d", result, len(client.seen))
	}
	secondRequest, _ := json.Marshal(client.seen[1])
	if !strings.Contains(string(secondRequest), "data:image/png;base64,") {
		t.Fatalf("second request has no image payload: %s", secondRequest)
	}
	traceText := readFileText(t, tracePath)
	assertNoImagePayload(t, traceText)
}

func testAgentConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	t.Setenv("MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT", "")
	t.Setenv("MADHARNESS_MINI_MAX_IMAGE_BYTES", "")
	t.Setenv("MADHARNESS_MINI_IMAGE_DETAIL", "")
	t.Setenv("MADHARNESS_MINI_APPROVAL_MODE", "")
	t.Setenv("MADHARNESS_MINI_YOLO", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func readTraceEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	events := []map[string]any{}
	for _, line := range splitLines(string(raw)) {
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	return events
}

func imageSequenceClient() *sequenceClient {
	return &sequenceClient{responses: []map[string]any{
		{"choices": []any{map[string]any{"message": map[string]any{
			"content": nil,
			"tool_calls": []any{map[string]any{
				"id":       "call_1",
				"function": map[string]any{"name": "read_image", "arguments": `{"path":"shot.png"}`},
			}},
		}}}},
		{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}}
}

func readFileText(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func assertNoImagePayload(t *testing.T, text string) {
	t.Helper()
	if strings.Contains(text, "data:image") || strings.Contains(text, "base64") {
		t.Fatalf("unexpected image payload in %s", text)
	}
}

func splitLines(text string) []string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
