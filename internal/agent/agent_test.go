package agent

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/model"
	"github.com/MADTeacher/madharness-mini-go/internal/prompt"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

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

func testAgentConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
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

func splitLines(text string) []string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
