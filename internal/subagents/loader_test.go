package subagents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

func TestDiscoverReadsBuiltinAndProjectSubagents(t *testing.T) {
	cfg := testConfig(t)
	writeTestSubagent(t, cfg.Root, "test-writer", "writable", []string{"list_files", "read_file", "search_code", "write_file"}, false)

	index := Discover(cfg)

	if _, ok := index.Subagents["planner"]; !ok {
		t.Fatal("planner builtin was not discovered")
	}
	subagent, ok := index.Subagents["test-writer"]
	if !ok {
		t.Fatal("project subagent was not discovered")
	}
	if subagent.Source != "project" || strings.Join(subagent.Tools, ",") != "list_files,read_file,search_code,write_file" {
		t.Fatalf("subagent = %+v", subagent)
	}
}

func TestProjectSubagentCannotShadowBuiltinWithoutOverride(t *testing.T) {
	cfg := testConfig(t)
	writeTestSubagent(t, cfg.Root, "planner", "writable", []string{"list_files"}, false)

	index := Discover(cfg)

	if index.Subagents["planner"].Source != "builtin" {
		t.Fatalf("planner source = %s", index.Subagents["planner"].Source)
	}
	if !hasDiagnostic(index.Diagnostics, "warning", "without override") {
		t.Fatalf("diagnostics = %+v", index.Diagnostics)
	}
}

func TestDiscoverRejectsInvalidSubagentFrontmatter(t *testing.T) {
	cfg := testConfig(t)
	root := filepath.Join(cfg.Root, projectSubagentsDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"bad-tools.md": strings.Join([]string{
			"---",
			"name: bad-tools",
			"description: Bad tools format.",
			"profile: writable",
			"tools: list_files read_file",
			"---",
			"body",
		}, "\n"),
		"bad-profile.md": strings.Join([]string{
			"---",
			"name: bad-profile",
			"description: Bad profile.",
			"profile: admin",
			`tools: ["list_files"]`,
			"---",
			"body",
		}, "\n"),
		"bad-delegate.md": strings.Join([]string{
			"---",
			"name: bad-delegate",
			"description: Bad delegate.",
			"profile: writable",
			`tools: ["list_files", "delegate_task"]`,
			"---",
			"body",
		}, "\n"),
		"unknown-tool.md": strings.Join([]string{
			"---",
			"name: unknown-tool",
			"description: Unknown tool.",
			"profile: writable",
			`tools: ["list_files", "no_such_tool"]`,
			"---",
			"body",
		}, "\n"),
		"duplicate.md": strings.Join([]string{
			"---",
			"name: duplicate",
			"description: Duplicate tool.",
			"profile: writable",
			`tools: ["list_files", "list_files"]`,
			"---",
			"body",
		}, "\n"),
	}
	for name, text := range cases {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	index := Discover(cfg)

	for _, name := range []string{"bad-tools", "bad-profile", "bad-delegate", "unknown-tool", "duplicate"} {
		if _, ok := index.Subagents[name]; ok {
			t.Fatalf("%s should not be loaded", name)
		}
	}
	for _, want := range []string{"JSON-style list", "invalid profile", "not exposed to subagents", "unknown subagent tool", "duplicate tool"} {
		if !hasDiagnostic(index.Diagnostics, "error", want) {
			t.Fatalf("missing diagnostic %q in %+v", want, index.Diagnostics)
		}
	}
}

func TestEffectiveToolsDowngradesButDoesNotUpgrade(t *testing.T) {
	writable := Subagent{Name: "writer", Profile: "writable", Tools: []string{"list_files", "write_file", "run_shell", "start_shell", "shell_status", "stop_shell", "ask_user"}}
	tools, err := EffectiveTools(writable, "read-only")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(tools, ",") != "list_files,ask_user" {
		t.Fatalf("tools = %v", tools)
	}
	readOnly := Subagent{Name: "reader", Profile: "read-only", Tools: []string{"list_files"}}
	if _, err := EffectiveTools(readOnly, "writable"); err == nil {
		t.Fatal("expected upgrade error")
	}
}

func TestChangedFilesFromEventsUsesOnlySuccessfulWrites(t *testing.T) {
	patch := strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: failed.txt",
		"@@",
		"-old",
		"+new",
		"*** End Patch",
	}, "\n")
	events := []map[string]any{
		{
			"event":       "tool_observation",
			"tool":        "write_file",
			"args":        map[string]any{"path": "failed-write.txt"},
			"observation": map[string]any{"ok": false},
		},
		{
			"event":       "tool_observation",
			"tool":        "apply_patch",
			"args":        map[string]any{"patch": patch},
			"observation": map[string]any{"ok": false},
		},
		{
			"event":       "tool_observation",
			"tool":        "write_file",
			"args":        map[string]any{"path": "ok.txt"},
			"observation": map[string]any{"ok": true},
		},
	}

	changed := ChangedFilesFromEvents(events)

	if strings.Join(changed, ",") != "ok.txt" {
		t.Fatalf("changed files = %v", changed)
	}
}

func writeTestSubagent(t *testing.T, root string, name string, profile string, tools []string, override bool) {
	t.Helper()
	dir := filepath.Join(root, projectSubagentsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	encodedTools, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{
		"---",
		"name: " + name,
		"description: Test subagent " + name + ".",
		"profile: " + profile,
		"tools: " + string(encodedTools),
		"max_turns: 4",
	}
	if override {
		lines = append(lines, "override: true")
	}
	lines = append(lines, "---", "", "Ты тестовый субагент.")
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	for _, key := range []string{
		"MADHARNESS_MINI_MODEL",
		"MADHARNESS_MINI_BASE_URL",
		"MADHARNESS_MINI_API_KEY",
		"MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS",
		"MADHARNESS_MINI_ORCHESTRATION_ENABLED",
		"MADHARNESS_MINI_ORCHESTRATION_MODE",
		"MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT",
		"MADHARNESS_MINI_MAX_IMAGE_BYTES",
		"MADHARNESS_MINI_IMAGE_DETAIL",
	} {
		t.Setenv(key, "")
	}
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func hasDiagnostic(diagnostics []Diagnostic, severity string, text string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == severity && strings.Contains(diagnostic.Message, text) {
			return true
		}
	}
	return false
}
