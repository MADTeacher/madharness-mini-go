//go:build unix

package instructions

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestLoadProjectRejectsSpecialFile(t *testing.T) {
	cfg := testInstructionsConfig(t)
	path := filepath.Join(cfg.Root, "AGENTS.md")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}

	text, err := LoadProject(cfg)

	if err == nil || text != "" || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("text=%q err=%v", text, err)
	}
}
