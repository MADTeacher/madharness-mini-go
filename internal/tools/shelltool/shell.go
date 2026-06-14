// Package shelltool запускает разрешённую subprocess-команду в workspace.
package shelltool

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/approval"
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
			"cwd":     tools.StrParam(".", "Workspace-relative directory to run from; use a skill root only for documented bundled scripts.", false),
		}, []string{"command"}),
		Handler: runShell,
		Effect:  tools.EffectShell,
	}
}

func runShell(ctx *tools.Context, args map[string]any) tools.Observation {
	command := tools.StringArg(args, "command", "")
	decision := ctx.Policy.ShellDecision(command)
	if !decision.Allowed {
		if !decision.Escalatable {
			return tools.Fail("run_shell", decision.Reason, map[string]any{"command": command})
		}
		approvalDecision := ctx.ApprovalForTool(approval.Request{
			Tool:    "run_shell",
			Action:  "run_shell",
			Subject: command,
			Code:    decision.Code,
			Reason:  decision.Reason,
			Details: map[string]any{"command": command},
		})
		if !approvalDecision.Approved {
			return tools.Fail("run_shell", decision.Reason, map[string]any{
				"command":         command,
				"approval_source": approvalDecision.Source,
			})
		}
	}
	cwdRaw := tools.StringArg(args, "cwd", ".")
	cwd, err := ctx.SafePathForTool("run_shell", cwdRaw, "shell_cwd")
	if err != nil {
		return tools.Fail("run_shell", err.Error(), map[string]any{"command": command})
	}
	info, err := os.Stat(cwd)
	if err != nil || !info.IsDir() {
		return tools.Fail("run_shell", "cwd is not a directory: "+cwdRaw, map[string]any{"command": command})
	}
	argv, shellMode, err := commandArgv(command, decision.Code)
	if err != nil {
		return tools.Fail("run_shell", "invalid shell command: "+err.Error(), map[string]any{"command": command})
	}
	if ctx.Trace != nil && ctx.ResourceTracker != nil {
		if event := ctx.ResourceTracker.ResourceEvent(cwd); event != nil {
			_ = ctx.Trace.Write("skill_resource_used", mergeEvent(event, map[string]any{"tool": "run_shell"}))
		}
	}
	release := ctx.LockWorkspaceExclusive("run_shell")
	defer release()
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, argv[0], argv[1:]...)
	cmd.Dir = cwd
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
	cwdDisplay := "."
	if rel, err := filepath.Rel(ctx.Config.Root, cwd); err == nil && rel != "." {
		cwdDisplay = filepath.ToSlash(rel)
	}
	return tools.OK("run_shell", "exit code "+itoa(returncode), map[string]any{
		"command":    command,
		"cwd":        cwdDisplay,
		"shell":      shellMode,
		"returncode": returncode,
		"stdout":     tools.Clipped(stdout.String(), tools.MaxOutput),
		"stderr":     tools.Clipped(stderr.String(), tools.MaxOutput),
	})
}

func commandArgv(command string, decisionCode string) ([]string, bool, error) {
	if decisionCode == policy.CodeShellControlOperator || hasShellControlOperator(command) {
		if runtime.GOOS == "windows" {
			return []string{"cmd", "/C", command}, true, nil
		}
		return []string{"/bin/sh", "-c", command}, true, nil
	}
	argv, err := policy.SplitCommand(command)
	return argv, false, err
}

func hasShellControlOperator(command string) bool {
	for _, token := range []string{"|", ">", "<", "&&", "||", ";"} {
		if strings.Contains(command, token) {
			return true
		}
	}
	return false
}

func itoa(value int) string {
	return fmtInt(value)
}

const description = `Run one allowed command in the workspace.

Use this for tests, builds, safe repository inspection, and documented skill
scripts. The command runs from the workspace root by default, or from a
workspace-relative cwd such as a skill root when cwd is provided, and times out
after 60 seconds. It must be a single command: shell control operators such as
|, >, <, &&, ||, and ; are denied, and risky commands such as sudo, curl, wget,
ssh, scp, chmod 777, mkfs, dd, and rm -rf are blocked by policy unless an
approval mode or YOLO mode explicitly allows the single action. Approved
commands with shell control operators run through the platform shell.
Do not use run_shell to edit files; use apply_patch for precise edits and
write_file only for deliberate full rewrites.`

func mergeEvent(base map[string]any, extra map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}
