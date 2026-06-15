package patchtool

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func TestApplyPatchAcceptsCRLFFinalNewline(t *testing.T) {
	ctx := newPatchTestContext(t)
	patch := "*** Begin Patch\r\n" +
		"*** Add File: out.txt\r\n" +
		"+hello\r\n" +
		"*** End Patch\r\n"

	obs := applyPatch(ctx, map[string]any{"patch": patch})

	if ok, _ := obs["ok"].(bool); !ok {
		t.Fatalf("applyPatch failed: %v", obs["summary"])
	}
	assertFileContent(t, filepath.Join(ctx.Config.Root, "out.txt"), "hello\n")
}

func TestApplyPatchFailedMultiFilePatchLeavesFilesUnchanged(t *testing.T) {
	ctx := newPatchTestContext(t)
	originalPath := filepath.Join(ctx.Config.Root, "original.txt")
	if err := os.WriteFile(originalPath, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "*** Begin Patch\n" +
		"*** Update File: original.txt\n" +
		"@@\n" +
		"-before\n" +
		"+after\n" +
		"*** Add File: scratch\n" +
		"+temporary\n" +
		"*** Add File: scratch/child.txt\n" +
		"+nested\n" +
		"*** End Patch"

	obs := applyPatch(ctx, map[string]any{"patch": patch})

	if ok, _ := obs["ok"].(bool); ok {
		t.Fatalf("applyPatch unexpectedly succeeded: %v", obs)
	}
	assertFileContent(t, originalPath, "before\n")
	if _, err := os.Stat(filepath.Join(ctx.Config.Root, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("scratch side effect exists after failed patch: %v", err)
	}
}

func newPatchTestContext(t *testing.T) *tools.Context {
	t.Helper()
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &tools.Context{Config: cfg, Policy: policy.New(cfg)}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, string(got), want)
	}
}
