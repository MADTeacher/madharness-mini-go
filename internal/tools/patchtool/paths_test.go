package patchtool

import (
	"path/filepath"
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
