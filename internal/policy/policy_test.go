package policy

import (
	"os"
	"path/filepath"
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

func TestDefaultProtectedPathsCoverControlPlane(t *testing.T) {
	p := New(testPolicyConfig(t))
	for _, path := range []string{
		"AGENTS.md",
		".codex/config.toml",
		".madharness-mini/config.json",
		".madharness-mini/hooks.json",
		".madharness-mini/mcp.json",
	} {
		decision := p.SafePathDecision(path)
		if decision.Allowed || !decision.Escalatable || decision.Code != CodeProtectedPath {
			t.Fatalf("%s decision = %#v", path, decision)
		}
	}
	decision := p.SafePathDecision(".madharness-mini/traces/trace-id/trace-id.jsonl")
	if !decision.Allowed {
		t.Fatalf("trace path should remain writable by harness: %#v", decision)
	}
	decision = p.SafePathDecision("app/config.json")
	if !decision.Allowed {
		t.Fatalf("ordinary config.json should not be protected by control-plane default: %#v", decision)
	}
}

func TestPathDecisionKeepsOutsideWorkspaceNonEscalatable(t *testing.T) {
	p := New(testPolicyConfig(t))
	decision := p.SafePathDecision("../outside.txt")
	if decision.Allowed || decision.Escalatable || decision.Code != CodePathOutsideWorkspace {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestPolicyDeniesSymlinkToOutsideWorkspace(t *testing.T) {
	cfg := testPolicyConfig(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(cfg.Root, "linked.txt"))

	decision := New(cfg).SafePathDecision("linked.txt")

	if decision.Allowed || decision.Escalatable || decision.Code != CodePathOutsideWorkspace {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestPolicyDeniesWriteThroughSymlinkDirectoryToOutsideWorkspace(t *testing.T) {
	cfg := testPolicyConfig(t)
	outside := t.TempDir()
	mustSymlink(t, outside, filepath.Join(cfg.Root, "out"))

	decision := New(cfg).SafePathDecision("out/new.txt")

	if decision.Allowed || decision.Escalatable || decision.Code != CodePathOutsideWorkspace {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestPolicyMarksSymlinkToProtectedPathEscalatable(t *testing.T) {
	cfg := testPolicyConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, ".env"), []byte("SECRET=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, ".env", filepath.Join(cfg.Root, "env-link"))

	decision := New(cfg).SafePathDecision("env-link")

	if decision.Allowed || !decision.Escalatable || decision.Code != CodeProtectedPath {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestPolicyDeniesSymlinkCWDOutsideWorkspace(t *testing.T) {
	cfg := testPolicyConfig(t)
	mustSymlink(t, t.TempDir(), filepath.Join(cfg.Root, "cwd-link"))

	decision := New(cfg).SafePathDecision("cwd-link")

	if decision.Allowed || decision.Escalatable || decision.Code != CodePathOutsideWorkspace {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestShellPolicyDeniesRiskyCommands(t *testing.T) {
	p := New(testPolicyConfig(t))
	risky := []string{
		"rm -rf .",
		"rm   -r   -f .",
		"rm\t-fr\t.",
		"/usr/bin/curl https://example.com",
		"wget\thttps://example.com",
		"ssh example.com",
		"scp a b",
		"sudo go test",
		"dd if=/dev/zero of=file",
		"mkfs.ext4 /dev/disk1",
		"chmod\t777 file",
	}
	for _, command := range risky {
		if ok, _ := p.ShellAllowed(command); ok {
			t.Fatalf("%q should be denied", command)
		}
	}
	if ok, reason := p.ShellAllowed("go test ./..."); !ok {
		t.Fatalf("go test denied: %s", reason)
	}
	if ok, reason := p.ShellAllowed("echo curl https://example.com"); !ok {
		t.Fatalf("echo with curl text denied: %s", reason)
	}
	if ok, reason := p.ShellAllowed("mycurl https://example.com"); !ok {
		t.Fatalf("basename-like command denied: %s", reason)
	}
	if ok, reason := p.ShellAllowed("rm file.txt"); !ok {
		t.Fatalf("non-recursive rm denied: %s", reason)
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
	background := p.ShellDecision("pwd &")
	if background.Allowed || !background.Escalatable || background.Code != CodeShellControlOperator {
		t.Fatalf("background = %#v", background)
	}
	invalid := p.ShellDecision(`printf "unterminated`)
	if invalid.Allowed || invalid.Escalatable || invalid.Code != CodeInvalidShellCommand {
		t.Fatalf("invalid = %#v", invalid)
	}
}

func TestShellControlOperatorsIgnoreQuotedAndEscapedText(t *testing.T) {
	p := New(testPolicyConfig(t))
	allowed := []string{
		`printf "a && b"`,
		`printf 'a;b'`,
		`printf a\&b`,
		`printf "a | b"`,
	}
	for _, command := range allowed {
		decision := p.ShellDecision(command)
		if !decision.Allowed {
			t.Fatalf("%q denied: %#v", command, decision)
		}
	}
	denied := []string{
		"printf ok&",
		"printf ok && printf again",
		"printf ok || printf fallback",
		"printf ok; printf again",
		"printf ok > out.txt",
		"printf ok < in.txt",
		"printf ok | cat",
	}
	for _, command := range denied {
		decision := p.ShellDecision(command)
		if decision.Allowed || decision.Code != CodeShellControlOperator {
			t.Fatalf("%q decision = %#v", command, decision)
		}
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

func TestSplitCommandKeepsEmptyQuotedArg(t *testing.T) {
	args, err := SplitCommand(`printf "" tail`)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 3 || args[1] != "" || args[2] != "tail" {
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

func mustSymlink(t *testing.T, oldname string, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlink is not available: %v", err)
	}
}
