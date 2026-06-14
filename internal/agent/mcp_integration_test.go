package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/mcp"
)

var agentFakeMCPSendMu sync.Mutex

func TestRunClosesMCPProcessAfterFinalAnswer(t *testing.T) {
	cfg := testAgentConfig(t)
	marker := writeAgentMCPConfig(t, cfg)
	client := &fakeClient{
		response: map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}

	result, _, err := runWithClient("finish", cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	if result != "done" {
		t.Fatalf("result = %q", result)
	}
	if text := readFileText(t, marker); text != "closed" {
		t.Fatalf("marker = %q", text)
	}
}

func TestAskDoesNotStartMCP(t *testing.T) {
	cfg := testAgentConfig(t)
	marker := writeAgentMCPConfig(t, cfg)
	client := &fakeClient{
		response: map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "done"}}}},
	}

	result, _, err := askWithClient("finish", cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	if result != "done" {
		t.Fatalf("result = %q", result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("MCP marker should not exist after ask, err=%v", err)
	}
}

func TestRunOverlapsMCPToolsWhenParallelEnabled(t *testing.T) {
	cfg := testAgentConfig(t)
	overlapDir := filepath.Join(t.TempDir(), "mcp-overlap")
	writeAgentMCPConfigWithEnv(t, cfg, map[string]string{
		"AGENT_MCP_OVERLAP_DIR": overlapDir,
	})
	client := &sequenceClient{responses: []map[string]any{
		mcpToolCallsResponse(),
		contentResponse("done"),
	}}

	result, tracePath, err := runWithClientOptions("call MCP twice", cfg, client, RunOptions{MaxParallelToolCalls: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result != "done" {
		t.Fatalf("result = %q", result)
	}
	contents := mcpObservationContents(t, readTraceEvents(t, tracePath))
	if mcpHasNoOverlapResult(contents) {
		t.Fatalf("MCP calls did not overlap: %v", contents)
	}
	if len(contents) != 2 || contents[0] != "echo:first" || contents[1] != "echo:second" {
		t.Fatalf("MCP observation order = %v", contents)
	}
	if !traceHasParallelMCPBatch(readTraceEvents(t, tracePath)) {
		t.Fatal("trace has no parallel MCP batch")
	}
}

func TestHelperProcessAgentMCP(t *testing.T) {
	if os.Getenv("GO_WANT_AGENT_MCP_HELPER_PROCESS") != "1" {
		return
	}
	runAgentFakeMCPServer()
	os.Exit(0)
}

func writeAgentMCPConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()
	return writeAgentMCPConfigWithEnv(t, cfg, nil)
}

func writeAgentMCPConfigWithEnv(t *testing.T, cfg *config.Config, extraEnv map[string]string) string {
	t.Helper()
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(cfg.Root, "agent_mcp_closed.txt")
	env := map[string]any{
		"GO_WANT_AGENT_MCP_HELPER_PROCESS": "1",
	}
	for key, value := range extraEnv {
		env[key] = value
	}
	data := map[string]any{
		"servers": map[string]any{
			"agentfake": map[string]any{
				"enabled": true,
				"command": os.Args[0],
				"args": []any{
					"-test.run=TestHelperProcessAgentMCP",
					"--",
					marker,
				},
				"cwd":             ".",
				"env":             env,
				"timeout_seconds": 5,
			},
		},
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.StateDir, "mcp.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return marker
}

func runAgentFakeMCPServer() {
	marker := ""
	if len(os.Args) > 0 {
		marker = os.Args[len(os.Args)-1]
	}
	if marker != "" {
		defer os.WriteFile(marker, []byte("closed"), 0o644)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := map[string]any{}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}
		handleAgentFakeMCPMessage(message)
	}
}

func handleAgentFakeMCPMessage(message map[string]any) {
	method, _ := message["method"].(string)
	requestID := message["id"]
	switch method {
	case "initialize":
		sendAgentFakeMCP(map[string]any{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": map[string]any{
				"protocolVersion": mcp.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "agentfake", "version": "1.0"},
			},
		})
	case "notifications/initialized":
		return
	case "tools/list":
		sendAgentFakeMCP(map[string]any{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": map[string]any{
				"tools": []any{map[string]any{
					"name":        "echo",
					"description": "Echo input.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"text":     map[string]any{"type": "string"},
							"delay_ms": map[string]any{"type": "number"},
						},
					},
				}},
			},
		})
	case "tools/call":
		go handleAgentFakeMCPToolCall(message, requestID)
	default:
		sendAgentFakeMCP(map[string]any{
			"jsonrpc": "2.0",
			"id":      requestID,
			"error":   map[string]any{"code": -32601, "message": "unknown method"},
		})
	}
}

func handleAgentFakeMCPToolCall(message map[string]any, requestID any) {
	params, _ := message["params"].(map[string]any)
	args, _ := params["arguments"].(map[string]any)
	text, _ := args["text"].(string)
	if overlapDir := os.Getenv("AGENT_MCP_OVERLAP_DIR"); overlapDir != "" {
		if !waitAgentMCPOverlap(overlapDir, text) {
			sendAgentFakeMCPToolResult(requestID, "no-overlap:"+text)
			return
		}
	}
	if delayMS := agentMCPIntArg(args, "delay_ms"); delayMS > 0 {
		time.Sleep(time.Duration(delayMS) * time.Millisecond)
	}
	sendAgentFakeMCPToolResult(requestID, "echo:"+text)
}

func waitAgentMCPOverlap(dir string, text string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	if err := os.WriteFile(filepath.Join(dir, text+".started"), []byte(text), 0o644); err != nil {
		return false
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		matches, err := filepath.Glob(filepath.Join(dir, "*.started"))
		if err == nil && len(matches) >= 2 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func sendAgentFakeMCPToolResult(requestID any, content string) {
	sendAgentFakeMCP(map[string]any{
		"jsonrpc": "2.0",
		"id":      requestID,
		"result": map[string]any{
			"content": []any{map[string]any{
				"type": "text",
				"text": content,
			}},
		},
	})
}

func sendAgentFakeMCP(message map[string]any) {
	raw, _ := json.Marshal(message)
	agentFakeMCPSendMu.Lock()
	defer agentFakeMCPSendMu.Unlock()
	os.Stdout.Write(append(raw, '\n'))
}

func mcpToolCallsResponse() map[string]any {
	firstArgs, _ := json.Marshal(map[string]any{"text": "first"})
	secondArgs, _ := json.Marshal(map[string]any{"text": "second"})
	return map[string]any{"choices": []any{map[string]any{"message": map[string]any{
		"content": nil,
		"tool_calls": []any{
			map[string]any{
				"id":       "call_first",
				"function": map[string]any{"name": "mcp__agentfake__echo", "arguments": string(firstArgs)},
			},
			map[string]any{
				"id":       "call_second",
				"function": map[string]any{"name": "mcp__agentfake__echo", "arguments": string(secondArgs)},
			},
		},
	}}}}
}

func mcpObservationContents(t *testing.T, events []map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, event := range events {
		if event["event"] != "tool_observation" || event["tool"] != "mcp__agentfake__echo" {
			continue
		}
		observation, _ := event["observation"].(map[string]any)
		content, _ := observation["content"].(string)
		out = append(out, content)
	}
	return out
}

func mcpHasNoOverlapResult(contents []string) bool {
	for _, content := range contents {
		if strings.HasPrefix(content, "no-overlap:") {
			return true
		}
	}
	return false
}

func traceHasParallelMCPBatch(events []map[string]any) bool {
	for _, event := range events {
		if event["event"] != "tool_execution_plan" {
			continue
		}
		groups, _ := event["groups"].([]any)
		for _, raw := range groups {
			group, _ := raw.(map[string]any)
			if group["kind"] == "mcp" && group["parallel"] == true {
				return true
			}
		}
	}
	return false
}

func agentMCPIntArg(args map[string]any, name string) int {
	switch value := args[name].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}
