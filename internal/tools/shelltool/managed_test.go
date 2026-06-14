package shelltool

import (
	"os"
	"os/exec"
	"os/signal"
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

func TestHelperShellTool(t *testing.T) {
	if os.Getenv("GO_WANT_SHELL_TOOL_HELPER") != "1" {
		return
	}
	helperArgs := shellToolHelperArgs()
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
