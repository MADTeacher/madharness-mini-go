package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

func TestDiscoverReadsFrontmatterAndNativeOverridesAgents(t *testing.T) {
	cfg := testConfig(t)
	writeSkill(t, filepath.Join(cfg.Root, ".agents", "skills", "docs-writer"), "docs-writer", "Агентская версия.")
	writeSkill(t, filepath.Join(cfg.Root, ".madharness_mini", "skills", "docs-writer"), "docs-writer", "Нативная версия.")

	index := Discover(cfg)

	if strings.Join(index.Names(), ",") != "docs-writer" {
		t.Fatalf("names = %v", index.Names())
	}
	skill := index.Skills["docs-writer"]
	if skill.Source != "native" || skill.Description != "Нативная версия." {
		t.Fatalf("skill = %+v", skill)
	}
	if skill.Metadata["author"] != "test" {
		t.Fatalf("metadata = %+v", skill.Metadata)
	}
	if strings.Join(skill.AllowedTools, " ") != "read_file run_shell" {
		t.Fatalf("allowed tools = %v", skill.AllowedTools)
	}
	if !hasDiagnostic(index.Diagnostics, "shadowed") {
		t.Fatalf("diagnostics = %+v", index.Diagnostics)
	}
}

func TestDiscoverParsesFoldedDescription(t *testing.T) {
	cfg := testConfig(t)
	root := filepath.Join(cfg.Root, ".agents", "skills", "folded")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "---\nname: folded\ndescription: >-\n  Первая строка\n  вторая строка.\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	index := Discover(cfg)

	if got := index.Skills["folded"].Description; got != "Первая строка вторая строка." {
		t.Fatalf("description = %q", got)
	}
}

func TestDiscoverSkipsInvalidSkill(t *testing.T) {
	cfg := testConfig(t)
	root := filepath.Join(cfg.Root, ".agents", "skills", "broken")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: broken\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	index := Discover(cfg)

	if len(index.Skills) != 0 || !hasDiagnostic(index.Diagnostics, "missing required description") {
		t.Fatalf("index = %+v", index)
	}
}

func TestListResourcesRejectsSymlinkEscape(t *testing.T) {
	cfg := testConfig(t)
	root := filepath.Join(cfg.Root, ".agents", "skills", "docs-writer")
	writeSkill(t, root, "docs-writer", "Пишет документацию.")
	outside := filepath.Join(cfg.Root, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	refDir := filepath.Join(root, "references")
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(refDir, "outside.txt")); err != nil {
		t.Fatal(err)
	}

	resources := ListResources(Discover(cfg).Skills["docs-writer"], cfg.Root)

	if len(resources) != 0 {
		t.Fatalf("resources = %+v", resources)
	}
}

func TestFindExplicitSelectionSupportsMarkersAndPhrases(t *testing.T) {
	selection := FindExplicitSelection(
		"@skill:docs-writer @skill/docs-writer используй навык test-skill $missing-skill",
		map[string]bool{"docs-writer": true, "test-skill": true},
	)

	if strings.Join(selection.Names, ",") != "docs-writer,test-skill" {
		t.Fatalf("names = %v", selection.Names)
	}
	if strings.Join(selection.Unknown, ",") != "missing-skill" {
		t.Fatalf("unknown = %v", selection.Unknown)
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func writeSkill(t *testing.T, root string, name string, description string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	text := strings.Join([]string{
		"---",
		"name: " + name,
		"description: " + description,
		"license: MIT",
		"compatibility: Requires only local files",
		"metadata:",
		"  author: test",
		"allowed-tools: read_file run_shell",
		"---",
		"",
		"# Docs Writer",
		"",
		"Пиши коротко и проверяемо.",
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasDiagnostic(diagnostics []Diagnostic, needle string) bool {
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, needle) {
			return true
		}
	}
	return false
}
