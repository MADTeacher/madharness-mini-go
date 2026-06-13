package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/mcp"
)

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

func TestHelperProcessAgentMCP(t *testing.T) {
	if os.Getenv("GO_WANT_AGENT_MCP_HELPER_PROCESS") != "1" {
		return
	}
	runAgentFakeMCPServer()
	os.Exit(0)
}

func writeAgentMCPConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(cfg.Root, "agent_mcp_closed.txt")
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
				"cwd": ".",
				"env": map[string]any{
					"GO_WANT_AGENT_MCP_HELPER_PROCESS": "1",
				},
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
			"result":  map[string]any{"tools": []any{}},
		})
	default:
		sendAgentFakeMCP(map[string]any{
			"jsonrpc": "2.0",
			"id":      requestID,
			"error":   map[string]any{"code": -32601, "message": "unknown method"},
		})
	}
}

func sendAgentFakeMCP(message map[string]any) {
	raw, _ := json.Marshal(message)
	os.Stdout.Write(append(raw, '\n'))
}
