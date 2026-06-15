package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

var fakeSendMu sync.Mutex

const fakePingRequestID int64 = 9002

func TestHelperProcessMCP(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_HELPER_PROCESS") != "1" {
		return
	}
	runFakeMCPServer()
	os.Exit(0)
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	t.Setenv("MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT", "")
	t.Setenv("MADHARNESS_MINI_MAX_IMAGE_BYTES", "")
	t.Setenv("MADHARNESS_MINI_IMAGE_DETAIL", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func fakeServerWorkspace(t *testing.T) (*config.Config, string) {
	t.Helper()
	cfg := testConfig(t)
	return cfg, filepath.Join(cfg.Root, "mcp_closed.txt")
}

func writeMCPConfig(t *testing.T, cfg *config.Config, data map[string]any) {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.StateDir, "mcp.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func fakeServerConfig(t *testing.T, cfg *config.Config, marker string, overrides map[string]any) map[string]any {
	t.Helper()
	server := map[string]any{
		"enabled": true,
		"command": os.Args[0],
		"args": []any{
			"-test.run=TestHelperProcessMCP",
			"--",
			marker,
		},
		"cwd": ".",
		"env": map[string]any{
			"GO_WANT_MCP_HELPER_PROCESS": "1",
			"DEMO_MODE":                  "1",
		},
		"timeout_seconds": 5,
	}
	for key, value := range overrides {
		server[key] = value
	}
	return map[string]any{"servers": map[string]any{"fake": server}}
}

func loadSingleConfig(t *testing.T, cfg *config.Config, marker string) ServerConfig {
	t.Helper()
	writeMCPConfig(t, cfg, fakeServerConfig(t, cfg, marker, nil))
	configs, err := LoadServerConfigs(cfg, policy.New(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("configs = %+v", configs)
	}
	return configs[0]
}

func runFakeMCPServer() {
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
		handleFakeMCPMessage(message)
	}
}

func handleFakeMCPMessage(message map[string]any) {
	method, _ := message["method"].(string)
	if method == "" {
		recordFakeClientResponse(message)
		return
	}
	if method == "notifications/cancelled" {
		recordFakeCancellation(message)
		return
	}
	requestID := message["id"]
	switch method {
	case "initialize":
		sendFakeMCP(map[string]any{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": map[string]any{
				"protocolVersion": ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "fake", "version": "1.0"},
			},
		})
	case "notifications/initialized":
		return
	case "tools/list":
		sendFakeMCP(map[string]any{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": map[string]any{
				"tools": []any{map[string]any{
					"name":        "echo",
					"description": "Echo input.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"text":      map[string]any{"type": "string"},
							"fail":      map[string]any{"type": "boolean"},
							"check_env": map[string]any{"type": "boolean"},
						},
						"additionalProperties": false,
					},
				}},
			},
		})
	case "tools/call":
		go handleFakeToolCall(message, requestID)
	default:
		sendFakeMCP(map[string]any{
			"jsonrpc": "2.0",
			"id":      requestID,
			"error":   map[string]any{"code": -32601, "message": "unknown method"},
		})
	}
}

func handleFakeToolCall(message map[string]any, requestID any) {
	params, _ := message["params"].(map[string]any)
	args, _ := params["arguments"].(map[string]any)
	if boolArg(args, "hang") {
		select {}
	}
	if delayMS := intArg(args, "delay_ms"); delayMS > 0 {
		time.Sleep(time.Duration(delayMS) * time.Millisecond)
	}
	if boolArg(args, "ask_client") {
		sendFakeMCP(map[string]any{
			"jsonrpc": "2.0",
			"id":      9001,
			"method":  "client/test",
			"params":  map[string]any{},
		})
		time.Sleep(20 * time.Millisecond)
	}
	if boolArg(args, "ping_client") {
		sendFakeMCP(map[string]any{
			"jsonrpc": "2.0",
			"id":      fakePingRequestID,
			"method":  "ping",
			"params":  map[string]any{},
		})
		time.Sleep(20 * time.Millisecond)
	}
	text, _ := args["text"].(string)
	content := "echo:" + text
	if args["check_env"] == true {
		content = "secret=" + os.Getenv("MADHARNESS_MINI_API_KEY") + ";demo=" + os.Getenv("DEMO_MODE")
	}
	sendFakeMCP(map[string]any{
		"jsonrpc": "2.0",
		"id":      requestID,
		"result": map[string]any{
			"isError": boolArg(args, "fail"),
			"content": []any{map[string]any{
				"type": "text",
				"text": content,
			}},
			"structuredContent": map[string]any{"seen": args},
		},
	})
}

func sendFakeMCP(message map[string]any) {
	raw, _ := json.Marshal(message)
	fakeSendMu.Lock()
	defer fakeSendMu.Unlock()
	os.Stdout.Write(append(raw, '\n'))
}

func recordFakeClientResponse(message map[string]any) {
	id, ok := idFromAny(message["id"])
	if !ok || id != fakePingRequestID {
		return
	}
	writeFakeMarker(os.Getenv("MCP_FAKE_PING_RESPONSE_MARKER"), message)
}

func recordFakeCancellation(message map[string]any) {
	writeFakeMarker(os.Getenv("MCP_FAKE_CANCEL_MARKER"), message)
}

func writeFakeMarker(path string, message map[string]any) {
	if path == "" {
		return
	}
	raw, err := json.Marshal(message)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}

func boolArg(args map[string]any, name string) bool {
	value, _ := args[name].(bool)
	return value
}

func intArg(args map[string]any, name string) int {
	switch value := args[name].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func readText(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func readJSONWhenReady(t *testing.T, path string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil && len(raw) > 0 {
			message := map[string]any{}
			if err := json.Unmarshal(raw, &message); err != nil {
				t.Fatal(err)
			}
			return message
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("marker was not written: %s", path)
	return nil
}
