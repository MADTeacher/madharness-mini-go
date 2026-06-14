package filetools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func TestReadFileRejectsSymlinkOutsideWorkspace(t *testing.T) {
	cfg := testFileToolsConfig(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(cfg.Root, "secret-link.txt"))

	obs := readFile(testFileToolsContext(cfg), map[string]any{"path": "secret-link.txt"})

	if obs["ok"] != false || !strings.Contains(obs["summary"].(string), "outside workspace") {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestReadFileUsesBoundedPrefix(t *testing.T) {
	cfg := testFileToolsConfig(t)
	content := strings.Repeat("a", readFileMaxBytes+1024)
	if err := os.WriteFile(filepath.Join(cfg.Root, "huge.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	obs := readFile(testFileToolsContext(cfg), map[string]any{"path": "huge.txt", "start": 1, "end": 1})

	if obs["ok"] != true || obs["truncated"] != true {
		t.Fatalf("obs = %+v", obs)
	}
	if len(obs["content"].(string)) > tools.MaxOutput+64 {
		t.Fatalf("content was not clipped: len=%d", len(obs["content"].(string)))
	}
}

func testFileToolsConfig(t *testing.T) *config.Config {
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

func testFileToolsContext(cfg *config.Config) *tools.Context {
	return &tools.Context{Config: cfg, Policy: policy.New(cfg)}
}

func mustSymlink(t *testing.T, oldname string, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlink is not available: %v", err)
	}
}
