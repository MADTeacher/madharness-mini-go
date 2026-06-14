package hooks

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

func TestObserveHookDoesNotBlockBeforeToolCall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-specific")
	}
	cfg := testHooksConfig(t)
	script := filepath.Join(cfg.Root, "observe_block.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '{\"ok\":false,\"block\":\"observe says no\"}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHooksJSON(t, cfg, `{"hooks":[{"id":"observer","mode":"observe","event":"before_tool_call","command":"`+script+`","cwd":".","timeout_seconds":3}]}`)
	tr, err := trace.New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := FromConfig(cfg, tr)
	if err != nil {
		t.Fatal(err)
	}

	decision := manager.Emit("before_tool_call", "run", map[string]any{"tool": "run_shell"})
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}

	if !decision.OK {
		t.Fatalf("decision = %#v", decision)
	}
	events := readHookTrace(t, tr.Path)
	if !traceHasHookEvent(events, "hook_blocked", "observer") {
		t.Fatalf("observe hook_blocked not found: %#v", events)
	}
}

func TestObserveHookFailureIsTracedOnClose(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-specific")
	}
	cfg := testHooksConfig(t)
	script := filepath.Join(cfg.Root, "observe_fail.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho observe boom >&2\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHooksJSON(t, cfg, `{"hooks":[{"id":"observer","mode":"observe","event":"session_start","command":"`+script+`","cwd":".","timeout_seconds":3}]}`)
	tr, err := trace.New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := FromConfig(cfg, tr)
	if err != nil {
		t.Fatal(err)
	}

	manager.Emit("session_start", "run", map[string]any{})
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}

	events := readHookTrace(t, tr.Path)
	if !traceHasHookError(events, "observer", "observe boom") {
		t.Fatalf("hook_failed not found: %#v", events)
	}
}

func TestObserveQueueOverflowIsTraced(t *testing.T) {
	cfg := testHooksConfig(t)
	tr, err := trace.New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	manager := newManager(nil, []Provider{blockingProvider{id: "slow-observer", release: release}}, tr)

	for index := 0; index < ObserveQueueSize+4; index++ {
		manager.Emit("session_start", "run", map[string]any{"index": index})
	}
	close(release)
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}

	events := readHookTrace(t, tr.Path)
	if !traceHasHookError(events, "slow-observer", "observe hook queue full") {
		t.Fatalf("queue overflow hook_failed not found: %#v", events)
	}
}

type blockingProvider struct {
	id      string
	release <-chan struct{}
}

func (p blockingProvider) ID() string {
	return p.id
}

func (p blockingProvider) Matches(Event) bool {
	return true
}

func (p blockingProvider) Handle(Event) (Decision, error) {
	<-p.release
	return Allow(), nil
}
