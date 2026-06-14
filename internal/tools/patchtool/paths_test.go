package patchtool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func TestTouchedPathsIncludesMoveTarget(t *testing.T) {
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := &tools.Context{Config: cfg, Policy: policy.New(cfg)}
	patch := "*** Begin Patch\n*** Update File: old.txt\n*** Move to: new.txt\n*** End Patch"

	paths, err := TouchedPaths(ctx, patch)
	if err != nil {
		t.Fatal(err)
	}

	wants := map[string]bool{
		filepath.Join(cfg.Root, "old.txt"): true,
		filepath.Join(cfg.Root, "new.txt"): true,
	}
	for _, path := range paths {
		delete(wants, path)
	}
	if len(wants) > 0 {
		t.Fatalf("missing paths = %+v; got %v", wants, paths)
	}
}

func TestTouchedPathsRejectsSymlinkDirectoryOutsideWorkspace(t *testing.T) {
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, t.TempDir(), filepath.Join(cfg.Root, "out"))
	ctx := &tools.Context{Config: cfg, Policy: policy.New(cfg)}
	patch := "*** Begin Patch\n*** Add File: out/new.txt\n+secret\n*** End Patch"

	_, err = TouchedPaths(ctx, patch)

	if err == nil || !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("err = %v", err)
	}
}

func mustSymlink(t *testing.T, oldname string, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlink is not available: %v", err)
	}
}
