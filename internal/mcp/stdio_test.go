package mcp

import (
	"testing"

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
