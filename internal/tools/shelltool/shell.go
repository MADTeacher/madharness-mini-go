// Package shelltool запускает разрешённую subprocess-команду в workspace.
package shelltool

import (
	"bytes"
	"context"
	"os/exec"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/policy"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// Spec возвращает run_shell tool.
func Spec() tools.Spec {
	return tools.Spec{
		Name:        "run_shell",
		Description: description,
		Parameters: tools.Obj(map[string]any{
			"command": tools.StrParam("", "Single safe command with arguments, run from the workspace root; no shell control operators and no file-editing scripts.", true),
		}, []string{"command"}),
		Handler: runShell,
	}
}

func runShell(ctx *tools.Context, args map[string]any) tools.Observation {
	command := tools.StringArg(args, "command", "")
	allowed, reason := ctx.Policy.ShellAllowed(command)
	if !allowed {
		return tools.Fail("run_shell", reason, map[string]any{"command": command})
	}
	argv, err := policy.SplitCommand(command)
	if err != nil {
		return tools.Fail("run_shell", "invalid shell command: "+err.Error(), map[string]any{"command": command})
	}
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, argv[0], argv[1:]...)
	cmd.Dir = ctx.Config.Root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	returncode := -1
	if cmd.ProcessState != nil {
		returncode = cmd.ProcessState.ExitCode()
	}
	if timeoutCtx.Err() == context.DeadlineExceeded {
		returncode = -1
	}
	if err != nil && cmd.ProcessState == nil {
		return tools.Fail("run_shell", err.Error(), map[string]any{"command": command})
	}
	return tools.OK("run_shell", "exit code "+itoa(returncode), map[string]any{
		"command":    command,
		"returncode": returncode,
		"stdout":     tools.Clipped(stdout.String(), tools.MaxOutput),
		"stderr":     tools.Clipped(stderr.String(), tools.MaxOutput),
	})
}

func itoa(value int) string {
	return fmtInt(value)
}

const description = `Run one allowed command in the workspace.

Use this for tests, builds, and safe repository inspection. The command runs
with cwd set to the workspace root and times out after 60 seconds. It must be a
single command: shell control operators such as |, >, <, &&, ||, and ; are
denied, and risky commands such as sudo, curl, wget, ssh, scp, chmod 777, mkfs,
dd, and rm -rf are blocked by policy. Do not use run_shell to edit files; use
apply_patch for precise edits and write_file only for deliberate full rewrites.`
