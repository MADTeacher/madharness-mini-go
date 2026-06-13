package policy

import (
	"testing"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

func TestPolicyDeniesOutsideWorkspace(t *testing.T) {
	p := New(testPolicyConfig(t))
	path, err := p.SafePath("../outside.txt")
	if err == nil || path != "" {
		t.Fatalf("path=%q err=%v", path, err)
	}
}

func TestPolicyDeniesProtectedPaths(t *testing.T) {
	p := New(testPolicyConfig(t))
	path, err := p.SafePath(".git/config")
	if err == nil || path != "" {
		t.Fatalf("path=%q err=%v", path, err)
	}
}

func TestShellPolicyDeniesRiskyCommands(t *testing.T) {
	p := New(testPolicyConfig(t))
	if ok, _ := p.ShellAllowed("rm -rf ."); ok {
		t.Fatal("rm -rf should be denied")
	}
	if ok, _ := p.ShellAllowed("curl https://example.com"); ok {
		t.Fatal("curl should be denied")
	}
	if ok, reason := p.ShellAllowed("go test ./..."); !ok {
		t.Fatalf("go test denied: %s", reason)
	}
}

func TestSplitCommandKeepsQuotedArg(t *testing.T) {
	args, err := SplitCommand(`printf "hello world"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[1] != "hello world" {
		t.Fatalf("args = %#v", args)
	}
}

func testPolicyConfig(t *testing.T) *config.Config {
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
