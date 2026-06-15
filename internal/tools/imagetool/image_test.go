package imagetool

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

var testPNGBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x00\x00\x00\x00\x00")

func TestReadImageRejectsSymlinkOutsideWorkspace(t *testing.T) {
	cfg := testImageToolConfig(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "shot.png"), testPNGBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(outside, "shot.png"), filepath.Join(cfg.Root, "shot.png"))

	obs := readImage(testImageToolContext(cfg), map[string]any{"path": "shot.png"})

	if obs["ok"] != false || !strings.Contains(obs["summary"].(string), "outside workspace") {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestReadImageWaitsBehindConflictingWriteLock(t *testing.T) {
	cfg := testImageToolConfig(t)
	path := filepath.Join(cfg.Root, "shot.png")
	if err := os.WriteFile(path, testPNGBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	scheduler := workspace.NewScheduler()
	unlock := scheduler.Acquire(workspace.LockWrite, []string{path})
	ctx := testImageToolContext(cfg)
	ctx.Scheduler = scheduler
	done := make(chan tools.Observation, 1)

	go func() {
		done <- readImage(ctx, map[string]any{"path": "shot.png"})
	}()

	assertReadImageStillBlocked(t, done)
	unlock()
	obs := waitReadImageDone(t, done)
	if obs["ok"] != true || obs["path"] != "shot.png" {
		t.Fatalf("obs = %+v", obs)
	}
}

func testImageToolConfig(t *testing.T) *config.Config {
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
	return cfg
}

func testImageToolContext(cfg *config.Config) *tools.Context {
	return &tools.Context{Config: cfg, Policy: policy.New(cfg)}
}

func mustSymlink(t *testing.T, oldname string, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlink is not available: %v", err)
	}
}

func assertReadImageStillBlocked(t *testing.T, done <-chan tools.Observation) {
	t.Helper()
	select {
	case obs := <-done:
		t.Fatalf("read_image finished before conflicting write lock was released: %+v", obs)
	case <-time.After(50 * time.Millisecond):
	}
}

func waitReadImageDone(t *testing.T, done <-chan tools.Observation) tools.Observation {
	t.Helper()
	select {
	case obs := <-done:
		return obs
	case <-time.After(time.Second):
		t.Fatal("read_image did not finish after write lock was released")
		return nil
	}
}
