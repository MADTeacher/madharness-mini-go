//go:build unix

package searchtool

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestSearchCodeSkipsSpecialFile(t *testing.T) {
	cfg := testSearchToolConfig(t)
	path := filepath.Join(cfg.Root, "pipe.txt")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}

	obs := searchCode(testSearchToolContext(cfg), map[string]any{"query": "NEEDLE", "glob": "*.txt"})

	if obs["ok"] != true || len(obs["results"].([]map[string]any)) != 0 {
		t.Fatalf("obs = %+v", obs)
	}
}
