package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/agentcontext"
)

func TestRuntimeActivatesSkillOnceAndListsResources(t *testing.T) {
	cfg := testConfig(t)
	root := filepath.Join(cfg.Root, ".agents", "skills", "docs-writer")
	writeSkill(t, root, "docs-writer", "Пишет документацию.")
	if err := os.MkdirAll(filepath.Join(root, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "references", "STYLE.md"), []byte("Style"), 0o644); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(cfg, Discover(cfg))

	obs := runtime.Activate("docs-writer", "test")

	if obs["ok"] != true || obs["already_active"] != false {
		t.Fatalf("obs = %+v", obs)
	}
	fragments, ok := obs["_context_fragments"].([]agentcontext.Fragment)
	if !ok || len(fragments) != 1 {
		t.Fatalf("fragments = %#v", obs["_context_fragments"])
	}
	if !strings.Contains(fragments[0].Text, "# Active Agent Skill: docs-writer") ||
		!strings.Contains(fragments[0].Text, "workspace path: `.agents/skills/docs-writer/references/STYLE.md`") {
		t.Fatalf("fragment = %s", fragments[0].Text)
	}

	again := runtime.Activate("docs-writer", "test")
	if again["ok"] != true || again["already_active"] != true {
		t.Fatalf("again = %+v", again)
	}
	if _, exists := again["_context_fragments"]; exists {
		t.Fatalf("repeated activation should not add fragments: %+v", again)
	}
}

func TestResourceEventTracksActiveSkillFilesAndRoot(t *testing.T) {
	cfg := testConfig(t)
	root := filepath.Join(cfg.Root, ".agents", "skills", "docs-writer")
	writeSkill(t, root, "docs-writer", "Пишет документацию.")
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "scripts", "check.sh")
	if err := os.WriteFile(script, []byte("echo ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(cfg, Discover(cfg))
	_ = runtime.Activate("docs-writer", "test")

	fileEvent := runtime.ResourceEvent(script)
	rootEvent := runtime.ResourceEvent(root)
	skillFileEvent := runtime.ResourceEvent(filepath.Join(root, "SKILL.md"))

	if fileEvent["path"] != "scripts/check.sh" || rootEvent["path"] != "." {
		t.Fatalf("events = %+v %+v", fileEvent, rootEvent)
	}
	if skillFileEvent != nil {
		t.Fatalf("SKILL.md should not be resource-used event: %+v", skillFileEvent)
	}
}
