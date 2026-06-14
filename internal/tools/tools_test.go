package tools

import (
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

func TestObservationFormat(t *testing.T) {
	if OK("x", "done", nil)["ok"] != true {
		t.Fatal("OK should set ok=true")
	}
	if Fail("x", "bad")["ok"] != false {
		t.Fatal("Fail should set ok=false")
	}
}

func TestParseToolArgs(t *testing.T) {
	name, args, err := ParseToolArgs(map[string]any{
		"function": map[string]any{"name": "read_file", "arguments": `{"path":"hello.txt"}`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "read_file" || args["path"] != "hello.txt" {
		t.Fatalf("name=%s args=%v", name, args)
	}
}

func TestIgnoredDetectsHiddenDirsInAbsolutePath(t *testing.T) {
	if !Ignored("/tmp/project/.git/config") {
		t.Fatal(".git path should be ignored")
	}
}

func TestRegistryFiltersAllowedTools(t *testing.T) {
	cfg := testToolsConfig(t)
	registry, err := NewRegistryWithOptions(cfg, RegistryOptions{AllowedTools: []string{"visible"}}, fakeProvider{
		specs: []Spec{
			{Name: "visible", Parameters: Obj(nil, nil), Handler: func(*Context, map[string]any) Observation { return OK("visible", "ok", nil) }},
			{Name: "hidden", Parameters: Obj(nil, nil), Handler: func(*Context, map[string]any) Observation { return OK("hidden", "ok", nil) }},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(registry.Tools(), ",") != "visible" {
		t.Fatalf("tools = %v", registry.Tools())
	}
	if obs := registry.Call("hidden", nil); obs["ok"] != false {
		t.Fatalf("hidden call = %+v", obs)
	}
}

func TestRegistryReturnsNormalizedToolEffects(t *testing.T) {
	cfg := testToolsConfig(t)
	registry, err := NewRegistry(cfg, fakeProvider{
		specs: []Spec{
			{Name: "read", Parameters: Obj(nil, nil), Handler: func(*Context, map[string]any) Observation { return OK("read", "ok", nil) }, Effect: EffectRead},
			{Name: "legacy", Parameters: Obj(nil, nil), Handler: func(*Context, map[string]any) Observation { return OK("legacy", "ok", nil) }},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if registry.Effect("read") != EffectRead {
		t.Fatalf("read effect = %s", registry.Effect("read"))
	}
	if registry.Effect("legacy") != EffectExclusive || registry.Effect("missing") != EffectExclusive {
		t.Fatalf("exclusive effects: legacy=%s missing=%s", registry.Effect("legacy"), registry.Effect("missing"))
	}
}

func TestContextWritePathScope(t *testing.T) {
	ctx := &Context{WritableSuffixes: []string{".md"}, WriteScopeDescription: "only markdown"}
	if err := ctx.WritePathError("PLAN.md"); err != "" {
		t.Fatalf("PLAN.md denied: %s", err)
	}
	if err := ctx.WritePathError("index.html"); !strings.Contains(err, "only markdown") {
		t.Fatalf("index.html error = %q", err)
	}
}

type fakeProvider struct {
	specs []Spec
}

func (p fakeProvider) Specs(ctx *Context) ([]Spec, error) {
	_ = ctx
	return p.specs, nil
}

func testToolsConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	t.Setenv("MADHARNESS_MINI_ORCHESTRATION_ENABLED", "")
	t.Setenv("MADHARNESS_MINI_ORCHESTRATION_MODE", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
