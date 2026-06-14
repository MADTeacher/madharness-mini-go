package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/trace"
)

func TestMissingHooksConfigIsNoop(t *testing.T) {
	cfg := testHooksConfig(t)
	tr, err := trace.New(cfg, "run")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := FromConfig(cfg, tr)
	if err != nil {
		t.Fatal(err)
	}

	decision := manager.Emit("session_start", "run", map[string]any{"task": "hello"})

	if !decision.OK {
		t.Fatalf("decision = %#v", decision)
	}
	events := readHookTrace(t, tr.Path)
	for _, event := range events {
		if event["event"] == "hook_started" {
			t.Fatal("missing hooks config should not emit hook_started")
		}
	}
}

func TestLoadCommandConfigsValidatesInput(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "invalid json",
			body: "{",
			want: "invalid hooks config JSON",
		},
		{
			name: "invalid event",
			body: `{"hooks":[{"id":"bad","event":"nope","command":"true"}]}`,
			want: "event must be one of",
		},
		{
			name: "invalid cwd",
			body: `{"hooks":[{"id":"bad","event":"session_start","command":"true","cwd":"../outside"}]}`,
			want: "path outside workspace",
		},
		{
			name: "invalid args",
			body: `{"hooks":[{"id":"bad","event":"session_start","command":"true","args":[1]}]}`,
			want: "args must be list of strings",
		},
		{
			name: "invalid env",
			body: `{"hooks":[{"id":"bad","event":"session_start","command":"true","env":{"A":1}}]}`,
			want: "env must be object of strings",
		},
		{
			name: "invalid match",
			body: `{"hooks":[{"id":"bad","event":"session_start","command":"true","match":{"tool":{"nested":true}}}]}`,
			want: "match must be object with scalar values",
		},
		{
			name: "invalid timeout",
			body: `{"hooks":[{"id":"bad","event":"session_start","command":"true","timeout_seconds":0}]}`,
			want: "timeout_seconds must be positive number",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testHooksConfig(t)
			writeHooksJSON(t, cfg, tc.body)
			_, err := LoadCommandConfigs(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
		})
	}
}

func TestCommandProviderMatchesKindToolAndLists(t *testing.T) {
	provider := CommandProvider{Config: CommandConfig{
		ID:    "guard",
		Event: "before_tool_call",
		Match: map[string]any{
			"kind": "run",
			"tool": []any{"write_file", "run_shell"},
		},
	}}

	matched := provider.Matches(Event{
		Name: "before_tool_call",
		Kind: "run",
		Data: map[string]any{"tool": "run_shell"},
	})
	missed := provider.Matches(Event{
		Name: "before_tool_call",
		Kind: "ask",
		Data: map[string]any{"tool": "run_shell"},
	})

	if !matched || missed {
		t.Fatalf("matched=%v missed=%v", matched, missed)
	}
}

func TestCompactPayloadRedactsAndClips(t *testing.T) {
	payload := map[string]any{
		"api_key": "secret",
		"nested":  map[string]any{"access_token": "token"},
		"text":    strings.Repeat("x", 2100),
		"items":   manyItems(25),
	}

	compact, ok := CompactPayload(payload).(map[string]any)
	if !ok {
		t.Fatalf("unexpected compact type %#v", compact)
	}
	if compact["api_key"] != "<redacted>" {
		t.Fatalf("api_key not redacted: %#v", compact["api_key"])
	}
	nested := compact["nested"].(map[string]any)
	if nested["access_token"] != "<redacted>" {
		t.Fatalf("access_token not redacted: %#v", nested["access_token"])
	}
	if !strings.Contains(compact["text"].(string), "[clipped") {
		t.Fatalf("text was not clipped: %q", compact["text"])
	}
	items := compact["items"].([]any)
	if len(items) != 21 || !strings.Contains(items[20].(string), "<clipped 5 items>") {
		t.Fatalf("items = %#v", items)
	}
}

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

func testHooksConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func writeHooksJSON(t *testing.T, cfg *config.Config, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(cfg.StateDir, "hooks.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readHookTrace(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	events := []map[string]any{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	return events
}

func manyItems(count int) []any {
	items := make([]any, 0, count)
	for index := 0; index < count; index++ {
		items = append(items, index)
	}
	return items
}
