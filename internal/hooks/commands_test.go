package hooks

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

func TestCommandHookDoesNotInheritMadharnessEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-specific")
	}
	cfg := testHooksConfig(t)
	script := filepath.Join(cfg.Root, "check_env.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nif [ -n \"$MADHARNESS_MINI_API_KEY\" ]; then echo '{\"ok\":false,\"block\":\"leaked\"}'; else echo '{\"ok\":true,\"message\":\"clean\"}'; fi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHooksJSON(t, cfg, `{"hooks":[{"id":"env-check","event":"session_start","command":"`+script+`","cwd":".","timeout_seconds":3}]}`)
	tr, err := trace.New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := FromConfig(cfg, tr)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MADHARNESS_MINI_API_KEY", "secret")

	decision := manager.Emit("session_start", "run", map[string]any{})

	if !decision.OK {
		t.Fatalf("decision = %#v", decision)
	}
	events := readHookTrace(t, tr.Path)
	found := false
	for _, event := range events {
		if event["event"] == "hook_finished" && event["message"] == "clean" {
			found = true
		}
	}
	if !found {
		t.Fatalf("hook_finished clean not found: %#v", events)
	}
}
