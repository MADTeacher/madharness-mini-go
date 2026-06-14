package agent

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunCanManageMultipleShellProcesses(t *testing.T) {
	cfg := testAgentConfig(t)
	client := &sequenceClient{responses: []map[string]any{
		{"choices": []any{map[string]any{"message": map[string]any{
			"content": nil,
			"tool_calls": []any{
				map[string]any{
					"id":       "call_backend",
					"function": map[string]any{"name": "start_shell", "arguments": `{"command":"` + escapedJSON(helperManagedCommand(t, "backend")) + `","name":"backend","ready_pattern":"backend ready","ready_timeout_seconds":1}`},
				},
				map[string]any{
					"id":       "call_frontend",
					"function": map[string]any{"name": "start_shell", "arguments": `{"command":"` + escapedJSON(helperManagedCommand(t, "frontend")) + `","name":"frontend","ready_pattern":"frontend ready","ready_timeout_seconds":1}`},
				},
			},
		}}}},
		{"choices": []any{map[string]any{"message": map[string]any{
			"content": nil,
			"tool_calls": []any{
				map[string]any{
					"id":       "stop_backend",
					"function": map[string]any{"name": "stop_shell", "arguments": `{"name":"backend","timeout_seconds":1}`},
				},
				map[string]any{
					"id":       "stop_frontend",
					"function": map[string]any{"name": "stop_shell", "arguments": `{"name":"frontend","timeout_seconds":1}`},
				},
			},
		}}}},
		{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}}

	result, tracePath, err := runWithClient("start backend and frontend", cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	if result != "done" {
		t.Fatalf("result = %q", result)
	}
	events := readTraceEvents(t, tracePath)
	if countTraceEvents(events, "process_started") != 2 {
		t.Fatalf("process_started events = %+v", events)
	}
	if countTraceEvents(events, "process_stopped") != 2 {
		t.Fatalf("process_stopped events = %+v", events)
	}
}

func TestHelperAgentManagedShell(t *testing.T) {
	if os.Getenv("GO_WANT_AGENT_MANAGED_SHELL_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	os.Stdout.WriteString(mode + " ready\n")
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func helperManagedCommand(t *testing.T, mode string) string {
	t.Helper()
	t.Setenv("GO_WANT_AGENT_MANAGED_SHELL_HELPER", "1")
	return shellQuoteForAgent(os.Args[0]) + " -test.run=TestHelperAgentManagedShell -- " + mode
}

func shellQuoteForAgent(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func escapedJSON(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

func countTraceEvents(events []map[string]any, name string) int {
	total := 0
	for _, event := range events {
		if event["event"] == name {
			total++
		}
	}
	return total
}
