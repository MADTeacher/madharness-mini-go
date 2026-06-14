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

func TestPathDecisionMarksProtectedPathEscalatable(t *testing.T) {
	p := New(testPolicyConfig(t))
	decision := p.SafePathDecision(".git/config")
	if decision.Allowed || !decision.Escalatable || decision.Code != CodeProtectedPath || decision.Path == "" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestPathDecisionKeepsOutsideWorkspaceNonEscalatable(t *testing.T) {
	p := New(testPolicyConfig(t))
	decision := p.SafePathDecision("../outside.txt")
	if decision.Allowed || decision.Escalatable || decision.Code != CodePathOutsideWorkspace {
		t.Fatalf("decision = %#v", decision)
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

func TestShellDecisionMarksEscalatableAndStrictDenials(t *testing.T) {
	p := New(testPolicyConfig(t))
	risky := p.ShellDecision("curl --version")
	if risky.Allowed || !risky.Escalatable || risky.Code != CodeRiskyShellCommand {
		t.Fatalf("risky = %#v", risky)
	}
	control := p.ShellDecision("pwd && pwd")
	if control.Allowed || !control.Escalatable || control.Code != CodeShellControlOperator {
		t.Fatalf("control = %#v", control)
	}
	invalid := p.ShellDecision(`printf "unterminated`)
	if invalid.Allowed || invalid.Escalatable || invalid.Code != CodeInvalidShellCommand {
		t.Fatalf("invalid = %#v", invalid)
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
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	t.Setenv("MADHARNESS_MINI_APPROVAL_MODE", "")
	t.Setenv("MADHARNESS_MINI_YOLO", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
