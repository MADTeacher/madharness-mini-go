package trace

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

func TestTraceWriteAndSummary(t *testing.T) {
	cfg := testTraceConfig(t)
	tr, err := New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(cfg.StateDir, "traces", tr.ID, tr.ID+".jsonl")
	if tr.Path != wantPath {
		t.Fatalf("trace path = %s, want %s", tr.Path, wantPath)
	}
	if err := tr.Write("tool_observation", map[string]any{"tool": "list_files"}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Write("skills_discovered", map[string]any{"count": 2}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Write("skill_activated", map[string]any{"name": "docs-writer"}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Write("skill_resource_used", map[string]any{"name": "docs-writer", "path": "references/style.md"}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Write("model_call_started", map[string]any{"context_report": map[string]any{
		"request_tokens_estimate": 120,
		"max_tokens":              60000,
		"tools_tokens_estimate":   30,
		"fragments":               []any{map[string]any{"id": "system"}},
		"history": map[string]any{
			"total_entries":         2,
			"rendered_entries":      1,
			"clipped_tool_messages": []any{map[string]any{"tool_call_id": "call_1"}},
			"dropped_entries":       []any{},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Write("session_end", map[string]any{"result": "ok"}); err != nil {
		t.Fatal(err)
	}
	events := readTraceEvents(t, tr.Path)
	for index, event := range events {
		if int(event["seq"].(float64)) != index+1 {
			t.Fatalf("seq at %d = %v", index, event["seq"])
		}
		if event["event_id"] == "" {
			t.Fatalf("missing event_id at %d: %#v", index, event)
		}
	}
	summary, err := Summarize(cfg, tr.ID[:8])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "tool calls: 1") || !strings.Contains(summary, "result: ok") {
		t.Fatalf("summary = %s", summary)
	}
	if !strings.Contains(summary, "context: 120/60000 estimated tokens") ||
		!strings.Contains(summary, "clipped tool messages: 1") {
		t.Fatalf("summary = %s", summary)
	}
	if !strings.Contains(summary, "skills: discovered 2; activated docs-writer; resources used 1") {
		t.Fatalf("summary = %s", summary)
	}
	if !strings.Contains(summary, "spans: total 1; open 1; max depth 1") {
		t.Fatalf("summary = %s", summary)
	}
	exactSummary, err := Summarize(cfg, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(exactSummary, "result: ok") {
		t.Fatalf("summary = %s", exactSummary)
	}
}

func TestTraceSpanNestingAndSummary(t *testing.T) {
	cfg := testTraceConfig(t)
	tr, err := New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	turn := tr.StartSpan("turn", tr.SessionSpanID(), map[string]any{"turn": 0})
	model := tr.StartSpan("model_call", turn.ID(), map[string]any{"turn": 0})
	if err := model.Trace().Write("model_call_started", map[string]any{"turn": 0}); err != nil {
		t.Fatal(err)
	}
	model.End("ok", nil)
	turn.End("done", nil)
	tr.EndSessionSpan("ok", nil)

	events := readTraceEvents(t, tr.Path)
	if got := collectSpanSummary(events); got.Total != 3 || got.Open != 0 || got.MaxDepth != 3 {
		t.Fatalf("span summary = %#v", got)
	}
	summary, err := Summarize(cfg, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "spans: total 3; open 0; max depth 3") {
		t.Fatalf("summary = %s", summary)
	}
}

func TestTraceWriteRedactsSensitiveEventPayloads(t *testing.T) {
	cfg := testTraceConfig(t)
	tr, err := New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	events := []struct {
		name   string
		fields map[string]any
	}{
		{
			name: "model_call_finished",
			fields: map[string]any{"message": map[string]any{
				"role":    "assistant",
				"content": "OPENAI_API_KEY=sk-model-secret-123456",
			}},
		},
		{
			name: "approval_request",
			fields: map[string]any{
				"tool":    "run_shell",
				"subject": `curl -H "Authorization: Bearer sk-approval-request-123456" https://example.test/v1`,
			},
		},
		{
			name: "approval_decision",
			fields: map[string]any{
				"tool":    "write_file",
				"message": "password=trace-decision-secret",
			},
		},
		{
			name: "tool_observation",
			fields: map[string]any{
				"tool": "write_file",
				"args": map[string]any{
					"path":    "config.env",
					"content": "refresh_token: ghp_trace_tool_token_123456",
				},
				"observation": map[string]any{"summary": "wrote config.env"},
			},
		},
		{
			name: "process_output",
			fields: map[string]any{
				"process_id": "proc-1",
				"stream":     "stdout",
				"content":    "Bearer sk-process-output-123456",
			},
		},
	}
	for _, event := range events {
		if err := tr.Write(event.name, event.fields); err != nil {
			t.Fatal(err)
		}
	}

	rendered := renderTraceEventsJSON(t, readTraceEvents(t, tr.Path))
	for _, leak := range []string{
		"sk-model-secret-123456",
		"sk-approval-request-123456",
		"trace-decision-secret",
		"ghp_trace_tool_token_123456",
		"sk-process-output-123456",
	} {
		if strings.Contains(rendered, leak) {
			t.Fatalf("trace leaked %q:\n%s", leak, rendered)
		}
	}
	for _, keep := range []string{
		"OPENAI_API_KEY=<redacted>",
		"Authorization: Bearer <redacted>",
		"password=<redacted>",
		"refresh_token: <redacted>",
		"Bearer <redacted>",
		"wrote config.env",
	} {
		if !strings.Contains(rendered, keep) {
			t.Fatalf("trace lost useful context %q:\n%s", keep, rendered)
		}
	}
}

func renderTraceEventsJSON(t *testing.T, events []map[string]any) string {
	t.Helper()
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(events); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestSummaryTruncatesUTF8Safely(t *testing.T) {
	text := strings.Repeat("Ж", 1200)
	if got := truncateRunes(text, 1000); len([]rune(got)) != 1000 || !utf8.ValidString(got) {
		t.Fatalf("bad truncation")
	}
}

func TestSummarizePrefersExactParentTraceAndShowsSubagents(t *testing.T) {
	cfg := testTraceConfig(t)
	parent, err := New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	child, err := parent.Child("subagent", "subagent-reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(child.Path) != filepath.Dir(parent.Path) {
		t.Fatalf("child path = %s, parent path = %s", child.Path, parent.Path)
	}
	if err := child.Write("session_end", map[string]any{"result": "child result"}); err != nil {
		t.Fatal(err)
	}
	if err := parent.Write("subagent_started", map[string]any{"name": "reviewer", "trace_id": child.ID}); err != nil {
		t.Fatal(err)
	}
	if err := parent.Write("subagent_finished", map[string]any{"name": "reviewer", "status": "done"}); err != nil {
		t.Fatal(err)
	}
	if err := parent.Write("session_end", map[string]any{"result": "parent result"}); err != nil {
		t.Fatal(err)
	}

	summary, err := Summarize(cfg, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "result: parent result") || strings.Contains(summary, "child result") {
		t.Fatalf("summary = %s", summary)
	}
	if !strings.Contains(summary, "subagents: events 2; names reviewer") {
		t.Fatalf("summary = %s", summary)
	}
}

func TestSummarizeReadsLegacyFlatTrace(t *testing.T) {
	cfg := testTraceConfig(t)
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	id := "legacy-trace"
	path := filepath.Join(cfg.StateDir, "traces", id+".jsonl")
	raw := strings.Join([]string{
		`{"event":"session_start","kind":"run","seq":1}`,
		`{"event":"session_end","result":"legacy result","seq":2}`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	summary, err := Summarize(cfg, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "result: legacy result") {
		t.Fatalf("summary = %s", summary)
	}
}

func TestSummarizeRejectsTraceIDPathSeparators(t *testing.T) {
	cfg := testTraceConfig(t)
	cases := []string{
		"../outside",
		"nested/trace",
		`nested\trace`,
		".",
		"..",
		"*",
	}
	for _, traceID := range cases {
		t.Run(traceID, func(t *testing.T) {
			_, err := Summarize(cfg, traceID)
			if err == nil {
				t.Fatal("expected invalid trace id error")
			}
			if !strings.Contains(err.Error(), "invalid trace id") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func testTraceConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	t.Setenv("MADHARNESS_MINI_ORCHESTRATION_ENABLED", "")
	t.Setenv("MADHARNESS_MINI_ORCHESTRATION_MODE", "")
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
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	return events
}
