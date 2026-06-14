//go:build unix

package imagetool

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestReadImageRejectsSpecialFile(t *testing.T) {
	cfg := testImageToolConfig(t)
	path := filepath.Join(cfg.Root, "pipe.png")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}

	obs := readImage(testImageToolContext(cfg), map[string]any{"path": "pipe.png"})

	if obs["ok"] != false || !strings.Contains(obs["summary"].(string), "not a file") {
		t.Fatalf("obs = %+v", obs)
	}
}
