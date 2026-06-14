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

func TestDeniedRiskyShellReturnsObservationAndTrace(t *testing.T) {
	cfg := testAgentConfig(t)
	client := shellThenDoneClient(`{"command":"curl --version"}`)

	result, tracePath, err := runWithClient("try risky shell", cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	if result != "done" || len(client.seen) != 2 {
		t.Fatalf("result=%q seen=%d", result, len(client.seen))
	}
	secondRequest, _ := json.Marshal(client.seen[1])
	if !strings.Contains(string(secondRequest), "risky shell command denied") {
		t.Fatalf("second request = %s", secondRequest)
	}
	events := readTraceEvents(t, tracePath)
	if !hasEvent(events, "approval_request") || !hasEvent(events, "approval_decision") {
		t.Fatalf("approval events not found: %#v", events)
	}
}

func TestApprovedShellControlOperatorRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture")
	}
	cfg := testAgentConfig(t)
	client := shellThenDoneClient(`{"command":"printf ok && printf done"}`)

	result, tracePath, err := runWithClientOptions("run shell", cfg, client, RunOptions{
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
	observation := firstObservation(readTraceEvents(t, tracePath), "run_shell")
	if observation["ok"] != true || observation["stdout"] != "okdone" || observation["shell"] != true {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestProtectedFileReadDeniedAndApproved(t *testing.T) {
	for _, tc := range []struct {
		name     string
		options  RunOptions
		wantText string
	}{
		{name: "denied", wantText: "protected path: .env"},
		{
			name: "approved",
			options: RunOptions{
				ApprovalMode: approval.ModeAsk,
				ApprovalPrompter: approval.StaticPrompter{Decision: approval.Decision{
					Approved: true,
					Source:   approval.SourceCLI,
				}},
			},
			wantText: "SECRET=value",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testAgentConfig(t)
			if err := os.WriteFile(filepath.Join(cfg.Root, ".env"), []byte("SECRET=value\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			client := readFileThenDoneClient(".env")

			result, _, err := runWithClientOptions("read protected", cfg, client, tc.options)
			if err != nil {
				t.Fatal(err)
			}
			if result != "done" || len(client.seen) != 2 {
				t.Fatalf("result=%q seen=%d", result, len(client.seen))
			}
			secondRequest, _ := json.Marshal(client.seen[1])
			if !strings.Contains(string(secondRequest), tc.wantText) {
				t.Fatalf("second request = %s", secondRequest)
			}
		})
	}
}

func TestProtectedWriteDeniedAndPatchApproved(t *testing.T) {
	t.Run("write denied", func(t *testing.T) {
		cfg := testAgentConfig(t)
		if err := os.WriteFile(filepath.Join(cfg.Root, ".env"), []byte("SECRET=old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		client := &sequenceClient{responses: []map[string]any{
			approvalToolCallResponse("call_write", "write_file", map[string]any{"path": ".env", "content": "SECRET=new\n"}),
			{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
		}}

		result, _, err := runWithClient("write protected", cfg, client)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(cfg.Root, ".env"))
		if err != nil {
			t.Fatal(err)
		}
		if result != "done" || string(raw) != "SECRET=old\n" {
			t.Fatalf("result=%q content=%q", result, raw)
		}
	})

	t.Run("write and patch approved", func(t *testing.T) {
		cfg := testAgentConfig(t)
		if err := os.WriteFile(filepath.Join(cfg.Root, ".env"), []byte("SECRET=old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		patch := "*** Begin Patch\n*** Update File: .env\n@@\n-SECRET=new\n+SECRET=patched\n*** End Patch"
		client := &sequenceClient{responses: []map[string]any{
			approvalToolCallResponse("call_write", "write_file", map[string]any{"path": ".env", "content": "SECRET=new\n"}),
			approvalToolCallResponse("call_patch", "apply_patch", map[string]any{"patch": patch}),
			{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
		}}

		result, tracePath, err := runWithClientOptions("write and patch protected", cfg, client, RunOptions{
			ApprovalMode: approval.ModeAsk,
			ApprovalPrompter: approval.StaticPrompter{Decision: approval.Decision{
				Approved: true,
				Source:   approval.SourceCLI,
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(cfg.Root, ".env"))
		if err != nil {
			t.Fatal(err)
		}
		if result != "done" || string(raw) != "SECRET=patched\n" {
			t.Fatalf("result=%q content=%q", result, raw)
		}
		if countEvents(readTraceEvents(t, tracePath), "approval_decision") != 2 {
			t.Fatalf("trace = %s", tracePath)
		}
	})
}

func TestYoloReadsProtectedPathButRejectsOutsideWorkspace(t *testing.T) {
	cfg := testAgentConfig(t)
	cfg.Data.YoloMode = true
	if err := os.WriteFile(filepath.Join(cfg.Root, ".env"), []byte("SECRET=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &sequenceClient{responses: []map[string]any{
		approvalToolCallResponse("call_read_env", "read_file", map[string]any{"path": ".env"}),
		approvalToolCallResponse("call_read_outside", "read_file", map[string]any{"path": "../outside.txt"}),
		{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}}

	result, _, err := runWithClient("read paths", cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	if result != "done" || len(client.seen) != 3 {
		t.Fatalf("result=%q seen=%d", result, len(client.seen))
	}
	secondRequest, _ := json.Marshal(client.seen[1])
	thirdRequest, _ := json.Marshal(client.seen[2])
	if !strings.Contains(string(secondRequest), "SECRET=value") {
		t.Fatalf("second request = %s", secondRequest)
	}
	if !strings.Contains(string(thirdRequest), "path outside workspace") {
		t.Fatalf("third request = %s", thirdRequest)
	}
}

func countEvents(events []map[string]any, name string) int {
	count := 0
	for _, event := range events {
		if event["event"] == name {
			count++
		}
	}
	return count
}

func shellThenDoneClient(arguments string) *sequenceClient {
	return &sequenceClient{responses: []map[string]any{
		{"choices": []any{map[string]any{"message": map[string]any{
			"content": nil,
			"tool_calls": []any{map[string]any{
				"id":       "call_shell",
				"function": map[string]any{"name": "run_shell", "arguments": arguments},
			}},
		}}}},
		{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}}
}

func readFileThenDoneClient(path string) *sequenceClient {
	return &sequenceClient{responses: []map[string]any{
		approvalToolCallResponse("call_read", "read_file", map[string]any{"path": path}),
		{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}}
}

func approvalToolCallResponse(id string, name string, args map[string]any) map[string]any {
	raw, _ := json.Marshal(args)
	return map[string]any{"choices": []any{map[string]any{"message": map[string]any{
		"content": nil,
		"tool_calls": []any{map[string]any{
			"id":       id,
			"function": map[string]any{"name": name, "arguments": string(raw)},
		}},
	}}}}
}
