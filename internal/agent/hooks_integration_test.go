package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/approval"
)

func TestBeforeToolCallHookBlocksBeforeHandler(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-specific")
	}
	cfg := testAgentConfig(t)
	writeAgentHook(t, cfg.Root, "deny_shell.sh", "#!/bin/sh\nprintf '{\"ok\":false,\"block\":\"no shell\"}'\n")
	writeAgentHooksConfig(t, cfg.Root, `{"hooks":[{"id":"deny-shell","event":"before_tool_call","match":{"tool":"run_shell"},"command":"./deny_shell.sh","cwd":".","timeout_seconds":10}]}`)
	client := &sequenceClient{responses: []map[string]any{
		{"choices": []any{map[string]any{"message": map[string]any{
			"content": nil,
			"tool_calls": []any{map[string]any{
				"id":       "call_shell",
				"function": map[string]any{"name": "run_shell", "arguments": `{"command":"pwd"}`},
			}},
		}}}},
		{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}}

	result, tracePath, err := runWithClient("run pwd", cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	if result != "done" || len(client.seen) != 2 {
		t.Fatalf("result=%q seen=%d", result, len(client.seen))
	}
	secondRequest, _ := json.Marshal(client.seen[1])
	if !strings.Contains(string(secondRequest), "blocked by hook: no shell") {
		t.Fatalf("second request does not contain hook block: %s", secondRequest)
	}
	events := readTraceEvents(t, tracePath)
	if !hasEvent(events, "hook_blocked") {
		t.Fatalf("hook_blocked not found: %#v", events)
	}
	observation := firstObservation(events, "run_shell")
	if observation["ok"] != false || observation["hook_blocked"] != true {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestHookProcessFailureIsTracedWithoutBreakingAsk(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-specific")
	}
	cfg := testAgentConfig(t)
	writeAgentHook(t, cfg.Root, "broken_hook.sh", "#!/bin/sh\necho boom >&2\nexit 2\n")
	writeAgentHooksConfig(t, cfg.Root, `{"hooks":[{"id":"broken","event":"before_model_call","command":"./broken_hook.sh","cwd":".","timeout_seconds":10}]}`)
	client := &fakeClient{response: map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "ok"}}}}}

	result, tracePath, err := askWithClient("hello", cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	if result != "ok" {
		t.Fatalf("result = %q", result)
	}
	events := readTraceEvents(t, tracePath)
	failed := findEvent(events, "hook_failed")
	if failed == nil || failed["hook"] != "broken" || !strings.Contains(failed["error"].(string), "boom") {
		t.Fatalf("hook_failed = %#v", failed)
	}
}

func TestObserveBeforeToolCallHookCannotBlockHandler(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-specific")
	}
	cfg := testAgentConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "note.txt"), []byte("hello from tool\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeAgentHook(t, cfg.Root, "observe_deny.sh", "#!/bin/sh\nprintf '{\"ok\":false,\"block\":\"observe cannot block\"}'\n")
	writeAgentHooksConfig(t, cfg.Root, `{"hooks":[{"id":"observe-deny","mode":"observe","event":"before_tool_call","match":{"tool":"read_file"},"command":"./observe_deny.sh","cwd":".","timeout_seconds":10}]}`)
	client := &sequenceClient{responses: []map[string]any{
		{"choices": []any{map[string]any{"message": map[string]any{
			"content": nil,
			"tool_calls": []any{map[string]any{
				"id":       "call_read",
				"function": map[string]any{"name": "read_file", "arguments": `{"path":"note.txt"}`},
			}},
		}}}},
		{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}}

	result, tracePath, err := runWithClient("read note", cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	if result != "done" || len(client.seen) != 2 {
		t.Fatalf("result=%q seen=%d", result, len(client.seen))
	}
	secondRequest, _ := json.Marshal(client.seen[1])
	if strings.Contains(string(secondRequest), "blocked by hook") || !strings.Contains(string(secondRequest), "hello from tool") {
		t.Fatalf("second request = %s", secondRequest)
	}
	events := readTraceEvents(t, tracePath)
	if !hasEvent(events, "hook_blocked") {
		t.Fatalf("observe hook_blocked not found: %#v", events)
	}
	observation := firstObservation(events, "read_file")
	if observation["ok"] != true || observation["hook_blocked"] == true {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestApprovalRequestHookBlocksBeforePromptDecision(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-specific")
	}
	cfg := testAgentConfig(t)
	writeAgentHook(t, cfg.Root, "deny_approval.sh", "#!/bin/sh\nprintf '{\"ok\":false,\"block\":\"no approval\"}'\n")
	writeAgentHooksConfig(t, cfg.Root, `{"hooks":[{"id":"deny-approval","event":"approval_request","match":{"tool":"run_shell"},"command":"./deny_approval.sh","cwd":".","timeout_seconds":10}]}`)
	client := shellThenDoneClient(`{"command":"curl --version"}`)

	result, tracePath, err := runWithClientOptions("run risky", cfg, client, RunOptions{
		ApprovalMode: approval.ModeAsk,
		ApprovalPrompter: approval.StaticPrompter{Decision: approval.Decision{
			Approved: true,
			Source:   approval.SourceCLI,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result != "done" || len(client.seen) != 2 {
		t.Fatalf("result=%q seen=%d", result, len(client.seen))
	}
	events := readTraceEvents(t, tracePath)
	if !hasEvent(events, "approval_request") || !hasEvent(events, "approval_decision") {
		t.Fatalf("approval events not found: %#v", events)
	}
	if blocked := findEvent(events, "hook_blocked"); blocked == nil || blocked["hook"] != "deny-approval" {
		t.Fatalf("hook_blocked = %#v", blocked)
	}
	decision := findEvent(events, "approval_decision")
	if decision["approved"] != false || decision["source"] != "deny-approval" {
		t.Fatalf("approval_decision = %#v", decision)
	}
}

func TestSubagentHookUsesChildTraceAndKind(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-specific")
	}
	cfg := testAgentConfig(t)
	writeAgentHook(t, cfg.Root, "subagent_marker.sh", "#!/bin/sh\npayload=$(cat)\nprintf '%s' \"$payload\" > subagent-hook-event.json\nprintf '{\"ok\":true,\"message\":\"seen\"}'\n")
	writeAgentHooksConfig(t, cfg.Root, `{"hooks":[{"id":"subagent-marker","event":"session_start","match":{"kind":"subagent"},"command":"./subagent_marker.sh","cwd":".","timeout_seconds":10}]}`)
	client := &sequenceClient{responses: []map[string]any{
		{"choices": []any{map[string]any{"message": map[string]any{
			"content": nil,
			"tool_calls": []any{map[string]any{
				"id":       "delegate_1",
				"function": map[string]any{"name": "delegate_task", "arguments": `{"subagent":"researcher","task":"Скажи ok"}`},
			}},
		}}}},
		{"choices": []any{map[string]any{"message": map[string]any{"content": "sub done"}}}},
		{"choices": []any{map[string]any{"message": map[string]any{"content": "parent done"}}}},
	}}

	result, tracePath, err := runWithClientOptions("Разбей задачу", cfg, client, RunOptions{OrchestrationMode: "required"})
	if err != nil {
		t.Fatal(err)
	}

	if result != "parent done" {
		t.Fatalf("result = %q", result)
	}
	raw, err := os.ReadFile(filepath.Join(cfg.Root, "subagent-hook-event.json"))
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["kind"] != "subagent" {
		t.Fatalf("payload = %#v", payload)
	}
	if payload["trace_id"] == "" {
		t.Fatalf("missing child trace id: %#v", payload)
	}
	parentEvents := readTraceEvents(t, tracePath)
	started := findEvent(parentEvents, "subagent_started")
	if started == nil {
		t.Fatalf("subagent_started not found: %#v", parentEvents)
	}
	childTrace := filepath.Join(cfg.CWD, started["trace_path"].(string))
	childEvents := readTraceEvents(t, childTrace)
	if !hasEvent(childEvents, "hook_finished") {
		t.Fatalf("child hook_finished not found: %#v", childEvents)
	}
}

func writeAgentHook(t *testing.T, root string, name string, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeAgentHooksConfig(t *testing.T, root string, body string) {
	t.Helper()
	stateDir := filepath.Join(root, ".madharness-mini")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "hooks.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasEvent(events []map[string]any, name string) bool {
	return findEvent(events, name) != nil
}

func findEvent(events []map[string]any, name string) map[string]any {
	for _, event := range events {
		if event["event"] == name {
			return event
		}
	}
	return nil
}

func firstObservation(events []map[string]any, tool string) map[string]any {
	for _, event := range events {
		if event["event"] != "tool_observation" || event["tool"] != tool {
			continue
		}
		observation, _ := event["observation"].(map[string]any)
		return observation
	}
	return nil
}
