package mcp

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func TestStdioClientInitializeAndListTools(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	config := loadSingleConfig(t, cfg, marker)
	client := NewStdioClient(config)
	defer client.Close()

	listed, err := client.Start()
	if err != nil {
		t.Fatal(err)
	}
	if listed[0]["name"] != "echo" {
		t.Fatalf("listed = %+v", listed)
	}
}

func TestStdioClientRoutesConcurrentResponsesByID(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	config := loadSingleConfig(t, cfg, marker)
	client := NewStdioClient(config)
	defer client.Close()
	if _, err := client.Start(); err != nil {
		t.Fatal(err)
	}

	type callResult struct {
		name   string
		result map[string]any
		err    error
	}
	done := make(chan callResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		result, err := client.CallTool("echo", map[string]any{"text": "slow", "delay_ms": 120})
		done <- callResult{name: "slow", result: result, err: err}
	}()
	time.Sleep(20 * time.Millisecond)
	go func() {
		defer wg.Done()
		result, err := client.CallTool("echo", map[string]any{"text": "fast"})
		done <- callResult{name: "fast", result: result, err: err}
	}()
	wg.Wait()
	close(done)

	results := map[string]callResult{}
	order := []string{}
	for item := range done {
		if item.err != nil {
			t.Fatalf("%s failed: %v", item.name, item.err)
		}
		order = append(order, item.name)
		results[item.name] = item
	}
	if len(order) != 2 || order[0] != "fast" {
		t.Fatalf("completion order = %v", order)
	}
	if mcpText(t, results["slow"].result) != "echo:slow" || mcpText(t, results["fast"].result) != "echo:fast" {
		t.Fatalf("results = %+v", results)
	}
}

func TestStdioClientTimeoutRemovesPendingRequest(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	config := loadSingleConfig(t, cfg, marker)
	config.Timeout = 40 * time.Millisecond
	client := NewStdioClient(config)
	defer client.Close()
	if _, err := client.Start(); err != nil {
		t.Fatal(err)
	}

	if _, err := client.CallTool("echo", map[string]any{"text": "late", "delay_ms": 120}); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout err = %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	result, err := client.CallTool("echo", map[string]any{"text": "next"})
	if err != nil {
		t.Fatal(err)
	}
	if text := mcpText(t, result); text != "echo:next" {
		t.Fatalf("text = %q", text)
	}
}

func TestStdioClientCloseUnblocksPendingRequests(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	config := loadSingleConfig(t, cfg, marker)
	config.Timeout = time.Second
	client := NewStdioClient(config)
	if _, err := client.Start(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := client.CallTool("echo", map[string]any{"hang": true})
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	client.Close()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "transport closed") {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pending MCP request was not unblocked")
	}
}

func TestStdioClientHandlesServerRequestDuringToolCall(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	config := loadSingleConfig(t, cfg, marker)
	client := NewStdioClient(config)
	defer client.Close()
	if _, err := client.Start(); err != nil {
		t.Fatal(err)
	}

	result, err := client.CallTool("echo", map[string]any{"text": "ok", "ask_client": true})
	if err != nil {
		t.Fatal(err)
	}
	if text := mcpText(t, result); text != "echo:ok" {
		t.Fatalf("text = %q", text)
	}
}

func TestStdioClientRespondsToServerPingWithEmptyResult(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	pingMarker := filepath.Join(cfg.Root, "mcp_ping_response.json")
	data := fakeServerConfig(t, cfg, marker, nil)
	server := data["servers"].(map[string]any)["fake"].(map[string]any)
	env := server["env"].(map[string]any)
	env["MCP_FAKE_PING_RESPONSE_MARKER"] = pingMarker
	writeMCPConfig(t, cfg, data)
	configs, err := LoadServerConfigs(cfg, policy.New(cfg))
	if err != nil {
		t.Fatal(err)
	}
	client := NewStdioClient(configs[0])
	defer client.Close()
	if _, err := client.Start(); err != nil {
		t.Fatal(err)
	}

	result, err := client.CallTool("echo", map[string]any{"text": "ok", "ping_client": true})
	if err != nil {
		t.Fatal(err)
	}
	if text := mcpText(t, result); text != "echo:ok" {
		t.Fatalf("text = %q", text)
	}
	response := readJSONWhenReady(t, pingMarker)
	if !sameID(response["id"], fakePingRequestID) {
		t.Fatalf("id = %v", response["id"])
	}
	if _, exists := response["error"]; exists {
		t.Fatalf("ping returned error: %+v", response)
	}
	pingResult, ok := response["result"].(map[string]any)
	if !ok || len(pingResult) != 0 {
		t.Fatalf("result = %+v", response["result"])
	}
}

func TestStdioClientSendsCancellationNotificationOnTimeout(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	cancelMarker := filepath.Join(cfg.Root, "mcp_cancel_notification.json")
	data := fakeServerConfig(t, cfg, marker, nil)
	server := data["servers"].(map[string]any)["fake"].(map[string]any)
	env := server["env"].(map[string]any)
	env["MCP_FAKE_CANCEL_MARKER"] = cancelMarker
	writeMCPConfig(t, cfg, data)
	configs, err := LoadServerConfigs(cfg, policy.New(cfg))
	if err != nil {
		t.Fatal(err)
	}
	config := configs[0]
	config.Timeout = 40 * time.Millisecond
	client := NewStdioClient(config)
	defer client.Close()
	if _, err := client.Start(); err != nil {
		t.Fatal(err)
	}

	if _, err := client.CallTool("echo", map[string]any{"text": "late", "delay_ms": 200}); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout err = %v", err)
	}
	notification := readJSONWhenReady(t, cancelMarker)
	if notification["method"] != "notifications/cancelled" {
		t.Fatalf("notification = %+v", notification)
	}
	params, ok := notification["params"].(map[string]any)
	if !ok {
		t.Fatalf("params = %+v", notification["params"])
	}
	if _, ok := idFromAny(params["requestId"]); !ok {
		t.Fatalf("requestId = %v", params["requestId"])
	}
	reason, _ := params["reason"].(string)
	if !strings.Contains(reason, "request timed out: tools/call") {
		t.Fatalf("reason = %q", reason)
	}
}

func TestMCPProviderExportsToolSpecs(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	writeMCPConfig(t, cfg, fakeServerConfig(t, cfg, marker, nil))
	provider := &ToolProvider{}
	registry, err := tools.NewRegistry(cfg, provider)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	names := registry.Tools()
	if !contains(names, "mcp__fake__echo") {
		t.Fatalf("tools = %v", names)
	}
}

func TestMCPToolCallReturnsObservation(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	writeMCPConfig(t, cfg, fakeServerConfig(t, cfg, marker, nil))
	registry, err := tools.NewRegistry(cfg, &ToolProvider{})
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	obs := registry.Call("mcp__fake__echo", map[string]any{"text": "hello"})

	if obs["ok"] != true || obs["tool"] != "mcp__fake__echo" || obs["content"] != "echo:hello" {
		t.Fatalf("obs = %+v", obs)
	}
	data, ok := obs["data"].(map[string]any)
	if !ok || data["seen"] == nil {
		t.Fatalf("data = %+v", obs["data"])
	}
}

func TestMCPToolCallIsErrorReturnsFailObservation(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	writeMCPConfig(t, cfg, fakeServerConfig(t, cfg, marker, nil))
	registry, err := tools.NewRegistry(cfg, &ToolProvider{})
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	obs := registry.Call("mcp__fake__echo", map[string]any{"fail": true})

	if obs["ok"] != false {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestMCPEnvDoesNotInheritModelAPIKey(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	writeMCPConfig(t, cfg, fakeServerConfig(t, cfg, marker, nil))
	t.Setenv("MADHARNESS_MINI_API_KEY", "secret")
	registry, err := tools.NewRegistry(cfg, &ToolProvider{})
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	obs := registry.Call("mcp__fake__echo", map[string]any{"check_env": true})

	if obs["ok"] != true || obs["content"] != "secret=;demo=1" {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestMCPProcessIsClosedAfterRegistryClose(t *testing.T) {
	cfg, marker := fakeServerWorkspace(t)
	writeMCPConfig(t, cfg, fakeServerConfig(t, cfg, marker, nil))
	provider := &ToolProvider{}
	registry, err := tools.NewRegistry(cfg, provider)
	if err != nil {
		t.Fatal(err)
	}

	registry.Close()

	if len(provider.clients) != 1 || provider.clients[0].cmd.ProcessState == nil {
		t.Fatalf("client was not closed: %+v", provider.clients)
	}
	if text := readText(t, marker); text != "closed" {
		t.Fatalf("marker = %q", text)
	}
}

func mcpText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content = %+v", result["content"])
	}
	item, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content item = %+v", content[0])
	}
	text, _ := item["text"].(string)
	return text
}
