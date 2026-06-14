package hooks

import (
	"strings"
	"testing"

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
			name: "invalid mode",
			body: `{"hooks":[{"id":"bad","mode":"async","event":"session_start","command":"true"}]}`,
			want: "mode must be enforce or observe",
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

func TestLoadCommandConfigsDefaultsModeToEnforce(t *testing.T) {
	cfg := testHooksConfig(t)
	writeHooksJSON(t, cfg, `{"hooks":[{"id":"audit","event":"session_start","command":"true"}]}`)

	configs, err := LoadCommandConfigs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 || configs[0].Mode != ModeEnforce {
		t.Fatalf("configs = %#v", configs)
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
		Name:     "before_tool_call",
		Kind:     "run",
		HookData: map[string]any{"tool": "run_shell"},
	})
	missed := provider.Matches(Event{
		Name:     "before_tool_call",
		Kind:     "ask",
		HookData: map[string]any{"tool": "run_shell"},
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
