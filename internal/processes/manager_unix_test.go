//go:build !windows

package processes

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestManagerCloseAllTerminatesDescendantProcessGroup(t *testing.T) {
	manager := NewManager()
	marker := filepath.Join(t.TempDir(), "descendants")
	status, err := manager.Start(helperStartOptions(t, "spawn-descendants", StartOptions{
		Command:      marker,
		ReadyPattern: "descendants ready",
		ReadyTimeout: time.Second,
	}))
	if err != nil {
		t.Fatal(err)
	}
	grandchildPID := readPIDFile(t, marker+".grandchild")

	manager.CloseAll(nil)
	waitForExit(t, manager, status.ProcessID)
	if !waitForProcessExit(grandchildPID, 2*time.Second) {
		_ = syscall.Kill(grandchildPID, syscall.SIGKILL)
		t.Fatalf("grandchild process still running: pid=%d", grandchildPID)
	}
}

func readPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(string(raw))
			if err != nil {
				t.Fatalf("invalid pid file %s: %q", path, raw)
			}
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pid file was not created: %s", path)
	return 0
}

func waitForProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return !processExists(pid)
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
