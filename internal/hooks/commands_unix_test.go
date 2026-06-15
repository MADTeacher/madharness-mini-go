//go:build unix

package hooks

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCommandHookTimeoutCleansDescendants(t *testing.T) {
	cfg := testHooksConfig(t)
	pidPath := filepath.Join(cfg.Root, "child.pid")
	readyPath := filepath.Join(cfg.Root, "ready")
	script := filepath.Join(cfg.Root, "spawn_child.sh")
	body := "#!/bin/sh\nsleep 30 &\necho $! > \"$1\"\ntouch \"$2\"\nwait\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	provider := CommandProvider{Config: CommandConfig{
		ID:             "timeout-cleanup",
		Event:          "session_start",
		Command:        script,
		Args:           []string{pidPath, readyPath},
		CWD:            cfg.Root,
		Env:            map[string]string{},
		TimeoutSeconds: 2,
	}}
	done := make(chan error, 1)

	go func() {
		_, err := provider.Handle(Event{Name: "session_start", HookData: map[string]any{}})
		done <- err
	}()

	waitForFile(t, readyPath)
	pid := readPIDFile(t, pidPath)
	err := waitHookDone(t, done)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if !waitForProcessExit(pid, 2*time.Second) {
		t.Fatalf("descendant process %d still exists after hook timeout", pid)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file did not appear: %s", path)
}

func waitHookDone(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("hook did not time out")
		return nil
	}
}

func readPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	var raw []byte
	var err error
	for time.Now().Before(deadline) {
		raw, err = os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(raw)) != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("invalid pid file %q: %v", string(raw), err)
	}
	return pid
}

func waitForProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return !processExists(pid)
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
