//go:build !windows

package shelltool

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func TestRunShellCleansChildAfterParentExit(t *testing.T) {
	cfg := testShellToolConfig(t)
	ctx := &tools.Context{
		Config: cfg,
		Policy: policy.New(cfg),
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")

	obs := runShell(ctx, map[string]any{"command": helperShellToolCommand(t, "spawn-child", pidFile)})
	if obs["ok"] != true {
		t.Fatalf("obs = %+v", obs)
	}
	childPID := readShellToolPIDFile(t, pidFile)
	if !waitForShellToolProcessExit(childPID, 2*time.Second) {
		_ = syscall.Kill(childPID, syscall.SIGKILL)
		t.Fatalf("child process still running: pid=%d", childPID)
	}
}

func readShellToolPIDFile(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(raw))
	if err != nil {
		t.Fatalf("invalid pid file %s: %q", path, raw)
	}
	return pid
}

func waitForShellToolProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !shellToolProcessExists(pid) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return !shellToolProcessExists(pid)
}

func shellToolProcessExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
