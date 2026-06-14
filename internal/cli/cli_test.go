package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCommandCreatesConfigWithAPIKey(t *testing.T) {
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := Main([]string{"init", "--base-url", "https://kodikrouter.ru/api/v1", "--model", "deepseek/deepseek-v4-flash", "--api-key", "secret", "--no-prompt"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	raw, err := os.ReadFile(filepath.Join(root, ".madharness-mini", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data["api_key"] != "secret" || !strings.Contains(out.String(), "Настройка записана") {
		t.Fatalf("data=%v out=%s", data, out.String())
	}
}

func TestSkillsCommandsListShowValidate(t *testing.T) {
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	skillRoot := filepath.Join(root, ".agents", "skills", "docs-writer")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "---\nname: docs-writer\ndescription: Пишет документацию.\nmetadata:\n  author: test\n---\nПиши коротко.\n"
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer

	code := Main([]string{"skills", "list"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "docs-writer") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	code = Main([]string{"skills", "show", "docs-writer"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "instructions:") || !strings.Contains(out.String(), "author: test") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	code = Main([]string{"skills", "validate"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "OK") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
}

func TestSubagentsCommandsListShowValidate(t *testing.T) {
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	t.Setenv("MADHARNESS_MINI_ORCHESTRATION_ENABLED", "")
	t.Setenv("MADHARNESS_MINI_ORCHESTRATION_MODE", "")
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	subagentRoot := filepath.Join(root, ".madharness-mini", "subagents")
	if err := os.MkdirAll(subagentRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "---\nname: test-writer\ndescription: Пишет тесты.\nprofile: writable\ntools: [\"list_files\", \"read_file\"]\nmetadata:\n  owner: test\n---\nПиши коротко.\n"
	if err := os.WriteFile(filepath.Join(subagentRoot, "test-writer.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer

	code := Main([]string{"subagents", "list"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "test-writer") || !strings.Contains(out.String(), "planner") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	code = Main([]string{"subagents", "show", "test-writer"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "prompt:") || !strings.Contains(out.String(), "owner: test") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	code = Main([]string{"subagents", "validate"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "OK") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
}

func TestSelectedOrchestrationMode(t *testing.T) {
	mode, err := selectedOrchestrationMode("", false, false, true)
	if err != nil || mode != "required" {
		t.Fatalf("mode=%q err=%v", mode, err)
	}
	mode, err = selectedOrchestrationMode("requested", false, false, false)
	if err != nil || mode != "requested" {
		t.Fatalf("mode=%q err=%v", mode, err)
	}
	if _, err := selectedOrchestrationMode("requested", true, false, false); err == nil {
		t.Fatal("expected conflicting orchestration flags error")
	}
}
