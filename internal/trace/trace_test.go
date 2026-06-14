package trace

import (
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
	exactSummary, err := Summarize(cfg, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(exactSummary, "result: ok") {
		t.Fatalf("summary = %s", exactSummary)
	}
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
