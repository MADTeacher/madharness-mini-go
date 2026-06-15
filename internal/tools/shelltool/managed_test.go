package shelltool

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/processes"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func TestStartShellReportsReadyTimeout(t *testing.T) {
	cfg := testShellToolConfig(t)
	manager := processes.NewManager()
	ctx := &tools.Context{
		Config:    cfg,
		Policy:    policy.New(cfg),
		Processes: manager,
	}
	defer manager.CloseAll(nil)

	obs := startShell(ctx, map[string]any{
		"command":               helperShellToolCommand(t, "sleep"),
		"ready_pattern":         "never-ready",
		"ready_timeout_seconds": 1,
	})

	if obs["ok"] != false || !strings.Contains(obs["summary"].(string), "readiness timeout") {
		t.Fatalf("obs = %+v", obs)
	}
	if obs["running"] != false || obs["ready"] != false {
		t.Fatalf("process should be stopped after timeout: %+v", obs)
	}
}

func TestRunShellDeniesProtectedAndOutsideFileArguments(t *testing.T) {
	cfg := testShellToolConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, ".env"), []byte("SECRET=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := &tools.Context{
		Config: cfg,
		Policy: policy.New(cfg),
	}

	protected := runShell(ctx, map[string]any{"command": "cat .env"})
	if protected["ok"] != false || !strings.Contains(protected["summary"].(string), "protected path") {
		t.Fatalf("protected obs = %+v", protected)
	}
	outside := runShell(ctx, map[string]any{"command": "cat ../outside.txt"})
	if outside["ok"] != false || !strings.Contains(outside["summary"].(string), "outside workspace") {
		t.Fatalf("outside obs = %+v", outside)
	}
}

func TestRunShellDeniesWrapperBypassCommands(t *testing.T) {
	cfg := testShellToolConfig(t)
	ctx := &tools.Context{
		Config: cfg,
		Policy: policy.New(cfg),
	}

	riskyWrapper := runShell(ctx, map[string]any{"command": "env curl https://example.com"})
	if riskyWrapper["ok"] != false || !strings.Contains(riskyWrapper["summary"].(string), "risky shell command") {
		t.Fatalf("riskyWrapper obs = %+v", riskyWrapper)
	}
	shellWrapper := runShell(ctx, map[string]any{"command": `sh -c "echo ok"`})
	if shellWrapper["ok"] != false || !strings.Contains(shellWrapper["summary"].(string), "shell interpreter wrappers") {
		t.Fatalf("shellWrapper obs = %+v", shellWrapper)
	}
}

func TestRunShellEnvDoesNotPrintAPIKeys(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("env command fixture is POSIX-specific")
	}
	cfg := testShellToolConfig(t)
	ctx := &tools.Context{
		Config: cfg,
		Policy: policy.New(cfg),
	}
	t.Setenv("MADHARNESS_MINI_API_KEY", "madharness-secret")
	t.Setenv("OPENAI_API_KEY", "openai-secret")

	obs := runShell(ctx, map[string]any{"command": "env"})

	if obs["ok"] != true {
		t.Fatalf("obs = %+v", obs)
	}
	stdout := obs["stdout"].(string)
	if strings.Contains(stdout, "MADHARNESS_MINI_API_KEY") || strings.Contains(stdout, "OPENAI_API_KEY") {
		t.Fatalf("api key env leaked in stdout: %q", stdout)
	}
	if strings.Contains(stdout, "madharness-secret") || strings.Contains(stdout, "openai-secret") {
		t.Fatalf("api key value leaked in stdout: %q", stdout)
	}
}

func TestStartShellDoesNotInheritAPIKeys(t *testing.T) {
	cfg := testShellToolConfig(t)
	manager := processes.NewManager()
	ctx := &tools.Context{
		Config:    cfg,
		Policy:    policy.New(cfg),
		Processes: manager,
	}
	defer manager.CloseAll(nil)
	t.Setenv("MADHARNESS_MINI_API_KEY", "madharness-secret")
	t.Setenv("OPENAI_API_KEY", "openai-secret")

	obs := startShell(ctx, map[string]any{
		"command":       helperShellToolCommand(t, "print-env-sleep"),
		"ready_pattern": "env-helper-ready",
	})

	if obs["ok"] != true {
		t.Fatalf("obs = %+v", obs)
	}
	stdout := obs["stdout"].(string)
	if strings.Contains(stdout, "MADHARNESS_MINI_API_KEY") || strings.Contains(stdout, "OPENAI_API_KEY") {
		t.Fatalf("api key env leaked in stdout: %q", stdout)
	}
	if strings.Contains(stdout, "madharness-secret") || strings.Contains(stdout, "openai-secret") {
		t.Fatalf("api key value leaked in stdout: %q", stdout)
	}
}

func TestHelperShellTool(t *testing.T) {
	helperArgs := shellToolHelperArgs()
	if os.Getenv("GO_WANT_SHELL_TOOL_HELPER") != "1" && len(helperArgs) == 0 {
		return
	}
	if len(helperArgs) == 0 {
		t.Fatal("missing helper mode")
	}
	mode := helperArgs[0]
	switch mode {
	case "sleep":
		os.Stdout.WriteString("helper sleeping\n")
		time.Sleep(30 * time.Second)
	case "spawn-child":
		marker := helperArgs[1]
		cmd := helperShellToolProcess(marker, "child-ignore-interrupt")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(marker, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Stdout.WriteString("child spawned\n")
	case "child-ignore-interrupt":
		ignoreShellToolInterrupt()
		time.Sleep(30 * time.Second)
	case "print-env-sleep":
		printLeakedAPIKeys()
		os.Stdout.WriteString("env-helper-ready\n")
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}

func testShellToolConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	t.Setenv("MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT", "")
	t.Setenv("MADHARNESS_MINI_MAX_IMAGE_BYTES", "")
	t.Setenv("MADHARNESS_MINI_IMAGE_DETAIL", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func helperShellToolCommand(t *testing.T, args ...string) string {
	t.Helper()
	t.Setenv("GO_WANT_SHELL_TOOL_HELPER", "1")
	command := shellQuoteForTest(os.Args[0]) + " -test.run=TestHelperShellTool --"
	for _, arg := range args {
		command += " " + shellQuoteForTest(arg)
	}
	return command
}

func helperShellToolProcess(marker string, mode string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperShellTool", "--", mode, marker)
	cmd.Env = append(os.Environ(), "GO_WANT_SHELL_TOOL_HELPER=1")
	return cmd
}

func shellToolHelperArgs() []string {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			return os.Args[i+1:]
		}
	}
	return nil
}

func shellQuoteForTest(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func ignoreShellToolInterrupt() {
	signal.Ignore(os.Interrupt)
}

func printLeakedAPIKeys() {
	if value := os.Getenv("MADHARNESS_MINI_API_KEY"); value != "" {
		os.Stdout.WriteString("MADHARNESS_MINI_API_KEY=" + value + "\n")
	}
	if value := os.Getenv("OPENAI_API_KEY"); value != "" {
		os.Stdout.WriteString("OPENAI_API_KEY=" + value + "\n")
	}
}
