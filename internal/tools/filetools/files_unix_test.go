//go:build unix

package filetools

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestReadFileRejectsSpecialFile(t *testing.T) {
	cfg := testFileToolsConfig(t)
	path := filepath.Join(cfg.Root, "pipe")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}

	obs := readFile(testFileToolsContext(cfg), map[string]any{"path": "pipe"})

	if obs["ok"] != false || !strings.Contains(obs["summary"].(string), "not a file") {
		t.Fatalf("obs = %+v", obs)
	}
}
