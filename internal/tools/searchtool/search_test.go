package searchtool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/workspace"
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

func TestSearchCodeWaitsBehindConflictingWriteLock(t *testing.T) {
	cfg := testSearchToolConfig(t)
	path := filepath.Join(cfg.Root, "locked.txt")
	if err := os.WriteFile(path, []byte("NEEDLE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scheduler := workspace.NewScheduler()
	unlock := scheduler.Acquire(workspace.LockWrite, []string{path})
	ctx := testSearchToolContext(cfg)
	ctx.Scheduler = scheduler
	done := make(chan tools.Observation, 1)

	go func() {
		done <- searchCode(ctx, map[string]any{"query": "NEEDLE", "glob": "*.txt"})
	}()

	assertSearchStillBlocked(t, done)
	unlock()
	obs := waitSearchDone(t, done)
	if obs["ok"] != true || len(obs["results"].([]map[string]any)) != 1 {
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

func assertSearchStillBlocked(t *testing.T, done <-chan tools.Observation) {
	t.Helper()
	select {
	case obs := <-done:
		t.Fatalf("search_code finished before conflicting write lock was released: %+v", obs)
	case <-time.After(50 * time.Millisecond):
	}
}

func waitSearchDone(t *testing.T, done <-chan tools.Observation) tools.Observation {
	t.Helper()
	select {
	case obs := <-done:
		return obs
	case <-time.After(time.Second):
		t.Fatal("search_code did not finish after write lock was released")
		return nil
	}
}
