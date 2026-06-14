package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

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

func traceHasHookEvent(events []map[string]any, name string, hook string) bool {
	for _, event := range events {
		if event["event"] == name && event["hook"] == hook {
			return true
		}
	}
	return false
}

func traceHasHookError(events []map[string]any, hook string, text string) bool {
	for _, event := range events {
		if event["event"] == "hook_failed" && event["hook"] == hook && strings.Contains(event["error"].(string), text) {
			return true
		}
	}
	return false
}

func manyItems(count int) []any {
	items := make([]any, 0, count)
	for index := 0; index < count; index++ {
		items = append(items, index)
	}
	return items
}
