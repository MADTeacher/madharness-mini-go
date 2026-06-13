package instructions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

func TestLoadProjectUsesRootAgentsMD(t *testing.T) {
	cfg := testInstructionsConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "AGENTS.md"), []byte("Use project tests.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	text, err := LoadProject(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if text != "Use project tests." {
		t.Fatalf("text = %q", text)
	}
}

func TestLoadProjectIgnoresMissingEmptyOverrideAndGlobal(t *testing.T) {
	cfg := testInstructionsConfig(t)
	if text, err := LoadProject(cfg); err != nil || text != "" {
		t.Fatalf("missing text=%q err=%v", text, err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, "AGENTS.md"), []byte("  \n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, "AGENTS.override.md"), []byte("Override"), 0o644); err != nil {
		t.Fatal(err)
	}
	if text, err := LoadProject(cfg); err != nil || text != "" {
		t.Fatalf("empty text=%q err=%v", text, err)
	}
}

func TestLoadProjectIgnoresNestedAgentsMD(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "services", "payments")
	if err := os.MkdirAll(filepath.Join(active, ".madharness-mini"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, active, map[string]any{"workspace_root": "../..", "allow_shell": true})
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Root rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "services", "AGENTS.md"), []byte("Service rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, "AGENTS.md"), []byte("Payment rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.New(active)
	if err != nil {
		t.Fatal(err)
	}

	text, err := LoadProject(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if text != "Root rules" {
		t.Fatalf("text = %q", text)
	}
}

func TestLoadProjectLimitsBytes(t *testing.T) {
	cfg := testInstructionsConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "AGENTS.md"), []byte(strings.Repeat("a", ProjectDocMaxBytes+10)), 0o644); err != nil {
		t.Fatal(err)
	}

	text, err := LoadProject(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len([]byte(text)) != ProjectDocMaxBytes {
		t.Fatalf("len = %d", len([]byte(text)))
	}
}

func testInstructionsConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT", "")
	t.Setenv("MADHARNESS_MINI_MAX_IMAGE_BYTES", "")
	t.Setenv("MADHARNESS_MINI_IMAGE_DETAIL", "")
	root := t.TempDir()
	writeConfig(t, root, map[string]any{"workspace_root": ".", "allow_shell": true})
	cfg, err := config.New(root)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func writeConfig(t *testing.T, root string, data map[string]any) {
	t.Helper()
	dir := filepath.Join(root, config.StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
