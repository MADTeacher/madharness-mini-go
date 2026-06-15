package mcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

func TestJSONRPCRequestHasIncrementingID(t *testing.T) {
	rpc := NewJSONRPCBuilder()

	first := rpc.Request("tools/list", map[string]any{})
	second := rpc.Request("tools/call", map[string]any{"name": "echo"})

	if first["id"] != int64(1) || second["id"] != int64(2) {
		t.Fatalf("ids = %v, %v", first["id"], second["id"])
	}
	if first["jsonrpc"] != "2.0" {
		t.Fatalf("jsonrpc = %v", first["jsonrpc"])
	}
}

func TestJSONRPCErrorReturnsRuntimeError(t *testing.T) {
	_, err := ParseResponse(map[string]any{
		"jsonrpc": "2.0",
		"id":      int64(1),
		"error":   map[string]any{"code": -32601, "message": "Method not found"},
	}, 1)
	if err == nil || !strings.Contains(err.Error(), "MCP JSON-RPC error -32601") {
		t.Fatalf("err = %v", err)
	}
}

func TestMCPConfigMissingFileMeansNoServers(t *testing.T) {
	cfg := testConfig(t)

	configs, err := LoadServerConfigs(cfg, policy.New(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 0 {
		t.Fatalf("configs = %+v", configs)
	}
}

func TestMCPConfigSkipsDisabledInvalidServer(t *testing.T) {
	cfg := testConfig(t)
	writeMCPConfig(t, cfg, map[string]any{
		"servers": map[string]any{
			"off": map[string]any{
				"enabled": false,
				"command": 42,
				"env":     []any{"bad"},
			},
		},
	})

	configs, err := LoadServerConfigs(cfg, policy.New(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 0 {
		t.Fatalf("configs = %+v", configs)
	}
}

func TestMCPConfigOrdersEnabledServersByName(t *testing.T) {
	cfg := testConfig(t)
	writeMCPConfig(t, cfg, map[string]any{
		"servers": map[string]any{
			"zeta": map[string]any{
				"enabled": true,
				"command": "fake",
				"cwd":     ".",
			},
			"off": map[string]any{
				"enabled": false,
				"command": 42,
			},
			"alpha": map[string]any{
				"enabled": true,
				"command": "fake",
				"cwd":     ".",
			},
		},
	})

	configs, err := LoadServerConfigs(cfg, policy.New(cfg))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, config := range configs {
		names = append(names, config.Name)
	}
	if strings.Join(names, ",") != "alpha,zeta" {
		t.Fatalf("names = %v", names)
	}
}

func TestMCPConfigRejectsUnsafeCWD(t *testing.T) {
	cfg := testConfig(t)
	writeMCPConfig(t, cfg, fakeServerConfig(t, cfg, filepath.Join(cfg.Root, "closed.txt"), map[string]any{
		"cwd": "../outside",
	}))

	_, err := LoadServerConfigs(cfg, policy.New(cfg))
	if err == nil || !strings.Contains(err.Error(), "path outside workspace") {
		t.Fatalf("err = %v", err)
	}
}

func TestMCPConfigRejectsNonStringEnv(t *testing.T) {
	cfg := testConfig(t)
	writeMCPConfig(t, cfg, fakeServerConfig(t, cfg, filepath.Join(cfg.Root, "closed.txt"), map[string]any{
		"env": map[string]any{"DEMO_MODE": 1},
	}))

	_, err := LoadServerConfigs(cfg, policy.New(cfg))
	if err == nil || !strings.Contains(err.Error(), "env must be object of strings") {
		t.Fatalf("err = %v", err)
	}
}

func TestExportedToolNameReplacesUnsupportedChars(t *testing.T) {
	name, err := ExportedToolName("docs", "search.index")
	if err != nil {
		t.Fatal(err)
	}
	if name != "mcp__docs__search_index" {
		t.Fatalf("name = %q", name)
	}
}

func TestMCPResultDropsMediaBase64(t *testing.T) {
	obs := ResultToObservation("mcp__fake__echo", "fake", "echo", map[string]any{
		"content": []any{
			map[string]any{"type": "text", "text": "ok"},
			map[string]any{"type": "image", "mimeType": "image/png", "data": "SECRET_BASE64"},
			map[string]any{"type": "resource_link", "uri": "file:///tmp/a.txt", "name": "a.txt"},
		},
		"structuredContent": map[string]any{"answer": 1},
	})

	raw, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "SECRET_BASE64") {
		t.Fatalf("base64 leaked: %s", raw)
	}
	if obs["ok"] != true || obs["content"] != "ok" {
		t.Fatalf("obs = %+v", obs)
	}
}
