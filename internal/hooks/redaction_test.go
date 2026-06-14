package hooks

import (
	"strings"
	"testing"
)

func TestHookPayloadRedactsRunShellAuthorizationBearer(t *testing.T) {
	command := `curl -H "Authorization: Bearer sk-live-secret-123456" https://example.test/v1`

	data := capturedHookData(t, map[string]any{
		"tool": "run_shell",
		"args": map[string]any{
			"command": command,
		},
	})

	args := data["args"].(map[string]any)
	redacted := args["command"].(string)
	if strings.Contains(redacted, "sk-live-secret-123456") {
		t.Fatalf("command leaked bearer token: %q", redacted)
	}
	if !strings.Contains(redacted, "Authorization: Bearer <redacted>") {
		t.Fatalf("command did not redact bearer token in place: %q", redacted)
	}
	if !strings.Contains(redacted, "curl -H") || !strings.Contains(redacted, "https://example.test/v1") {
		t.Fatalf("command lost useful context: %q", redacted)
	}
}

func TestHookPayloadRedactsWriteFileContentSecrets(t *testing.T) {
	content := strings.Join([]string{
		"OPENAI_API_KEY=sk-file-secret-123456",
		"refresh_token: ghp_file_token_123456",
		"note=token budget stays visible",
	}, "\n")

	data := capturedHookData(t, map[string]any{
		"tool": "write_file",
		"args": map[string]any{
			"path":    "config.env",
			"content": content,
		},
	})

	args := data["args"].(map[string]any)
	redacted := args["content"].(string)
	if strings.Contains(redacted, "sk-file-secret-123456") || strings.Contains(redacted, "ghp_file_token_123456") {
		t.Fatalf("content leaked secret values: %q", redacted)
	}
	if !strings.Contains(redacted, "OPENAI_API_KEY=<redacted>") {
		t.Fatalf("content did not redact api key assignment: %q", redacted)
	}
	if !strings.Contains(redacted, "refresh_token: <redacted>") {
		t.Fatalf("content did not redact token assignment: %q", redacted)
	}
	if !strings.Contains(redacted, "note=token budget stays visible") {
		t.Fatalf("content lost non-secret context: %q", redacted)
	}
}

func TestHookPayloadKeepsNonSecretStrings(t *testing.T) {
	prompt := "Discuss token budgets, bearer tokens, Authorization headers, token=demo, password=example, api_key=placeholder, secretary=alice"

	data := capturedHookData(t, map[string]any{
		"tool":   "write_file",
		"prompt": prompt,
	})

	if data["prompt"] != prompt {
		t.Fatalf("non-secret prompt was over-redacted: %q", data["prompt"])
	}
}

func capturedHookData(t *testing.T, data map[string]any) map[string]any {
	t.Helper()
	provider := &captureProvider{}
	manager := NewManager([]Provider{provider}, nil)
	decision := manager.Emit("before_tool_call", "run", data)
	if !decision.OK {
		t.Fatalf("decision = %#v", decision)
	}
	if provider.event.HookData == nil {
		t.Fatal("provider did not receive hook data")
	}
	return provider.event.HookData
}

type captureProvider struct {
	event Event
}

func (p *captureProvider) ID() string {
	return "capture"
}

func (p *captureProvider) Matches(Event) bool {
	return true
}

func (p *captureProvider) Handle(event Event) (Decision, error) {
	p.event = event
	return Allow(), nil
}
