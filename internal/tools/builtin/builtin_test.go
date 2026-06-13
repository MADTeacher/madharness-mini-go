package builtin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/builtin"
)

func TestBuiltinSchemasIncludeApplyPatch(t *testing.T) {
	registry := testRegistry(t)
	names := []string{}
	for _, schema := range registry.Schemas() {
		fn := schema["function"].(map[string]any)
		names = append(names, fn["name"].(string))
	}
	if !contains(names, "apply_patch") {
		t.Fatalf("schemas = %v", names)
	}
}

func TestBuiltinSchemasOrderIncludesReadImage(t *testing.T) {
	registry := testRegistry(t)
	names := []string{}
	for _, schema := range registry.Schemas() {
		fn := schema["function"].(map[string]any)
		names = append(names, fn["name"].(string))
	}
	want := []string{"list_files", "read_file", "read_image", "write_file", "apply_patch", "search_code", "run_shell"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %v", names)
	}
}

func TestReadAndWriteFileTools(t *testing.T) {
	registry := testRegistry(t)
	obs := registry.Call("write_file", map[string]any{"path": "example/hello.txt", "content": "hello\n"})
	if obs["ok"] != true {
		t.Fatalf("write obs = %+v", obs)
	}
	obs = registry.Call("read_file", map[string]any{"path": "example/hello.txt", "start": 1, "end": 1})
	if obs["ok"] != true || !strings.Contains(obs["content"].(string), "1: hello") {
		t.Fatalf("read obs = %+v", obs)
	}
}

func TestWriteFileRespectsPathPolicy(t *testing.T) {
	obs := testRegistry(t).Call("write_file", map[string]any{"path": "../nope.txt", "content": "bad"})
	if obs["ok"] != false {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestListFilesAndSearchCode(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "hello.go"), []byte("package hello\nfunc SafePath() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, ".env"), []byte("SECRET_TOKEN=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := registry.Call("list_files", map[string]any{"path": ".", "glob": "*.go"})
	if files["ok"] != true || len(files["files"].([]string)) != 1 {
		t.Fatalf("files = %+v", files)
	}
	protected := registry.Call("list_files", map[string]any{"path": ".", "glob": ".env"})
	if protected["ok"] != true || len(protected["files"].([]string)) != 0 {
		t.Fatalf("protected list = %+v", protected)
	}
	search := registry.Call("search_code", map[string]any{"query": "SafePath", "glob": "*.go"})
	if search["ok"] != true || len(search["results"].([]map[string]any)) != 1 {
		t.Fatalf("search = %+v", search)
	}
	secret := registry.Call("search_code", map[string]any{"query": "SECRET_TOKEN", "glob": "*"})
	if secret["ok"] != true || len(secret["results"].([]map[string]any)) != 0 {
		t.Fatalf("secret search = %+v", secret)
	}
}

func TestApplyPatchCreatesAndUpdatesFile(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	add := "*** Begin Patch\n*** Add File: hello.txt\n+one\n+two\n*** End Patch"
	obs := registry.Call("apply_patch", map[string]any{"patch": add})
	if obs["ok"] != true {
		t.Fatalf("add obs = %+v", obs)
	}
	update := "*** Begin Patch\n*** Update File: hello.txt\n@@\n one\n-two\n+TWO\n*** End Patch"
	obs = registry.Call("apply_patch", map[string]any{"patch": update})
	if obs["ok"] != true {
		t.Fatalf("update obs = %+v", obs)
	}
	raw, err := os.ReadFile(filepath.Join(cfg.Root, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "one\nTWO\n" {
		t.Fatalf("content = %q", raw)
	}
}

func TestApplyPatchReportsContextFailure(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "hello.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "*** Begin Patch\n*** Update File: hello.txt\n@@\n-missing\n+new\n*** End Patch"
	obs := registry.Call("apply_patch", map[string]any{"patch": patch})
	if obs["ok"] != false || obs["retryable"] != true {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestRunShellTool(t *testing.T) {
	obs := testRegistry(t).Call("run_shell", map[string]any{"command": "go version"})
	if obs["ok"] != true || obs["returncode"].(int) != 0 {
		t.Fatalf("obs = %+v", obs)
	}
}

func testRegistry(t *testing.T) *tools.Registry {
	t.Helper()
	_, registry := testRegistryWithConfig(t)
	return registry
}

func testRegistryWithConfig(t *testing.T) (*config.Config, *tools.Registry) {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT", "")
	t.Setenv("MADHARNESS_MINI_MAX_IMAGE_BYTES", "")
	t.Setenv("MADHARNESS_MINI_IMAGE_DETAIL", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := tools.NewRegistry(cfg, builtin.Provider{})
	if err != nil {
		t.Fatal(err)
	}
	return cfg, registry
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
