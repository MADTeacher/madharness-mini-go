package trace

import (
	"strings"
	"testing"
)

func TestSummarizeShowsConcurrencyPlan(t *testing.T) {
	cfg := testTraceConfig(t)
	tr, err := New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Write("tool_execution_plan", map[string]any{
		"max_parallel_tool_calls": 3,
		"groups": []map[string]any{
			{"parallel": true, "count": 2, "tools": []string{"read_file", "search_code"}},
			{"parallel": false, "count": 1, "tools": []string{"write_file"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Write("session_end", map[string]any{"result": "done"}); err != nil {
		t.Fatal(err)
	}

	summary, err := Summarize(cfg, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := "concurrency: max parallel 3; parallel read batches 1; barrier groups 1"
	if !strings.Contains(summary, want) {
		t.Fatalf("summary missing %q:\n%s", want, summary)
	}
}
