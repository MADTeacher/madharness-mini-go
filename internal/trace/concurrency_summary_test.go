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
		"max_parallel_subagents":  2,
		"groups": []map[string]any{
			{"parallel": true, "kind": "read", "count": 2, "tools": []string{"read_file", "search_code"}},
			{"parallel": true, "kind": "mcp", "count": 2, "tools": []string{"mcp__fake__a", "mcp__fake__b"}},
			{"parallel": true, "kind": "delegate", "count": 2, "tools": []string{"delegate_task", "delegate_task"}},
			{"parallel": false, "kind": "barrier", "count": 1, "tools": []string{"write_file"}},
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
	want := "concurrency: max tool calls 3; max subagents 2; parallel read batches 1; parallel MCP batches 1; parallel delegate batches 1; barrier groups 1"
	if !strings.Contains(summary, want) {
		t.Fatalf("summary missing %q:\n%s", want, summary)
	}
}
