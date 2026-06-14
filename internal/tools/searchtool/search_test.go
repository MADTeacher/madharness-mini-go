package searchtool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func TestSearchCodeSkipsSymlinkOutsideWorkspace(t *testing.T) {
	cfg := testSearchToolConfig(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("NEEDLE"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(cfg.Root, "secret.txt"))

	obs := searchCode(testSearchToolContext(cfg), map[string]any{"query": "NEEDLE", "glob": "*.txt"})

	if obs["ok"] != true || len(obs["results"].([]map[string]any)) != 0 {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestSearchCodeUsesBoundedPrefix(t *testing.T) {
	cfg := testSearchToolConfig(t)
	content := strings.Repeat("a", searchFileMaxBytes+1024) + "\nNEEDLE\n"
	if err := os.WriteFile(filepath.Join(cfg.Root, "huge.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	obs := searchCode(testSearchToolContext(cfg), map[string]any{"query": "NEEDLE", "glob": "*.txt"})

	if obs["ok"] != true || obs["truncated"] != true || len(obs["results"].([]map[string]any)) != 0 {
		t.Fatalf("obs = %+v", obs)
	}
}

func testSearchToolConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func testSearchToolContext(cfg *config.Config) *tools.Context {
	return &tools.Context{Config: cfg, Policy: policy.New(cfg)}
}

func mustSymlink(t *testing.T, oldname string, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlink is not available: %v", err)
	}
}
