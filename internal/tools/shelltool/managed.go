package shelltool

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/approval"
	"github.com/MADTeacher/madharness-mini-go/internal/processes"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

const (
	defaultReadyTimeoutSeconds = 10
	defaultStopTimeoutSeconds  = 5
)

// Specs возвращает обычный run_shell и managed shell tools.
func Specs() []tools.Spec {
	return []tools.Spec{
		Spec(),
		startShellSpec(),
		shellStatusSpec(),
		stopShellSpec(),
	}
}

func startShellSpec() tools.Spec {
	return tools.Spec{
		Name:        "start_shell",
		Description: startShellDescription,
		Parameters: tools.Obj(map[string]any{
			"command":               tools.StrParam("", "Single safe command with arguments to start as a managed long-running process.", true),
			"cwd":                   tools.StrParam(".", "Workspace-relative directory to run from.", false),
			"name":                  tools.StrParam("", "Optional unique name for this managed process.", false),
			"ready_pattern":         tools.StrParam("", "Optional regular expression matched against stdout and stderr to mark the process ready.", false),
			"ready_timeout_seconds": tools.IntParam(defaultReadyTimeoutSeconds),
		}, []string{"command"}),
		Handler: startShell,
		Effect:  tools.EffectShell,
	}
}

func shellStatusSpec() tools.Spec {
	return tools.Spec{
		Name:        "shell_status",
		Description: shellStatusDescription,
		Parameters: tools.Obj(map[string]any{
			"process_id": tools.StrParam("", "Managed process id such as proc-1. Omit with name to list all processes.", false),
			"name":       tools.StrParam("", "Optional managed process name. Omit with process_id to list all processes.", false),
		}, nil),
		Handler: shellStatus,
		Effect:  tools.EffectShell,
	}
}

func stopShellSpec() tools.Spec {
	return tools.Spec{
		Name:        "stop_shell",
		Description: stopShellDescription,
		Parameters: tools.Obj(map[string]any{
			"process_id":      tools.StrParam("", "Managed process id such as proc-1.", false),
			"name":            tools.StrParam("", "Optional managed process name.", false),
			"timeout_seconds": tools.IntParam(defaultStopTimeoutSeconds),
		}, nil),
		Handler: stopShell,
		Effect:  tools.EffectShell,
	}
}

func startShell(ctx *tools.Context, args map[string]any) tools.Observation {
	if ctx.Processes == nil {
		return tools.Fail("start_shell", "process manager is not configured")
	}
	command := tools.StringArg(args, "command", "")
	cwdRaw := tools.StringArg(args, "cwd", ".")
	cwd, err := ctx.SafePathForTool("start_shell", cwdRaw, "shell_cwd")
	if err != nil {
		return tools.Fail("start_shell", err.Error(), map[string]any{"command": command})
	}
	info, err := os.Stat(cwd)
	if err != nil || !info.IsDir() {
		return tools.Fail("start_shell", "cwd is not a directory: "+cwdRaw, map[string]any{"command": command})
	}
	decision := ctx.Policy.ShellDecisionInDir(command, cwd)
	if !decision.Allowed {
		if !decision.Escalatable {
			return tools.Fail("start_shell", decision.Reason, map[string]any{"command": command})
		}
		approvalDecision := ctx.ApprovalForTool(approval.Request{
			Tool:    "start_shell",
			Action:  "start_shell",
			Subject: command,
			Code:    decision.Code,
			Reason:  decision.Reason,
			Details: map[string]any{"command": command},
		})
		if !approvalDecision.Approved {
			return tools.Fail("start_shell", decision.Reason, map[string]any{
				"command":         command,
				"approval_source": approvalDecision.Source,
			})
		}
	}
	argv, shellMode, err := commandArgv(command, decision.Code)
	if err != nil {
		return tools.Fail("start_shell", "invalid shell command: "+err.Error(), map[string]any{"command": command})
	}
	cwdDisplay := cwdForObservation(ctx, cwd)
	release := ctx.LockWorkspaceExclusive("start_shell")
	defer release()
	status, err := ctx.Processes.Start(processes.StartOptions{
		Command:      command,
		Argv:         argv,
		CWD:          cwd,
		CWDDisplay:   cwdDisplay,
		Name:         tools.StringArg(args, "name", ""),
		Shell:        shellMode,
		ReadyPattern: tools.StringArg(args, "ready_pattern", ""),
		ReadyTimeout: time.Duration(tools.IntArg(args, "ready_timeout_seconds", defaultReadyTimeoutSeconds)) * time.Second,
		Trace:        ctx.Trace,
	})
	data := statusData(status)
	data["command"] = command
	if err != nil {
		if errors.Is(err, processes.ErrExitedBeforeReady) {
			return tools.Fail("start_shell", "process exited before readiness", data)
		}
		if errors.Is(err, processes.ErrReadyTimeout) {
			return tools.Fail("start_shell", "process readiness timeout", data)
		}
		return tools.Fail("start_shell", err.Error(), data)
	}
	return tools.OK("start_shell", startSummary(status), data)
}

func shellStatus(ctx *tools.Context, args map[string]any) tools.Observation {
	if ctx.Processes == nil {
		return tools.Fail("shell_status", "process manager is not configured")
	}
	statuses, err := ctx.Processes.Status(
		tools.StringArg(args, "process_id", ""),
		tools.StringArg(args, "name", ""),
	)
	if err != nil {
		return tools.Fail("shell_status", err.Error())
	}
	if len(statuses) == 1 && (tools.StringArg(args, "process_id", "") != "" || tools.StringArg(args, "name", "") != "") {
		return tools.OK("shell_status", statusSummary(statuses[0]), statusData(statuses[0]))
	}
	items := []map[string]any{}
	for _, status := range statuses {
		items = append(items, statusData(status))
	}
	return tools.OK("shell_status", "managed processes: "+itoa(len(items)), map[string]any{"processes": items})
}

func stopShell(ctx *tools.Context, args map[string]any) tools.Observation {
	if ctx.Processes == nil {
		return tools.Fail("stop_shell", "process manager is not configured")
	}
	release := ctx.LockWorkspaceExclusive("stop_shell")
	defer release()
	status, err := ctx.Processes.Stop(processes.StopOptions{
		ProcessID: tools.StringArg(args, "process_id", ""),
		Name:      tools.StringArg(args, "name", ""),
		Timeout:   time.Duration(tools.IntArg(args, "timeout_seconds", defaultStopTimeoutSeconds)) * time.Second,
		Reason:    "stop_shell",
		Trace:     ctx.Trace,
	})
	if err != nil {
		return tools.Fail("stop_shell", err.Error())
	}
	return tools.OK("stop_shell", "stopped "+status.ProcessID, statusData(status))
}

func cwdForObservation(ctx *tools.Context, cwd string) string {
	if ctx == nil || ctx.Config == nil {
		return filepath.ToSlash(cwd)
	}
	if rel, err := filepath.Rel(ctx.Config.Root, cwd); err == nil {
		if rel == "." {
			return "."
		}
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(cwd)
}

func statusData(status processes.Status) map[string]any {
	data := map[string]any{
		"process_id": status.ProcessID,
		"name":       status.Name,
		"command":    status.Command,
		"cwd":        status.CWD,
		"pid":        status.PID,
		"shell":      status.Shell,
		"running":    status.Running,
		"ready":      status.Ready,
		"stdout":     tools.Clipped(status.Stdout, tools.MaxOutput),
		"stderr":     tools.Clipped(status.Stderr, tools.MaxOutput),
	}
	if status.ExitCode != nil {
		data["exit_code"] = *status.ExitCode
	} else {
		data["exit_code"] = nil
	}
	return data
}

func startSummary(status processes.Status) string {
	if status.Ready {
		return "started " + status.ProcessID + " and matched readiness"
	}
	return "started " + status.ProcessID
}

func statusSummary(status processes.Status) string {
	if status.Running {
		return status.ProcessID + " is running"
	}
	return status.ProcessID + " exited"
}

const startShellDescription = `Start one allowed command as a managed long-running process.

Use this for development servers, frontend clients, watchers, or other commands
that should keep running while the agent continues testing. The process belongs
only to the current run and is cleaned up automatically when the run exits.
Use ready_pattern when the command prints a clear startup message.`

const shellStatusDescription = `Inspect managed shell processes from the current run.

Call without process_id or name to list all known processes. Call with
process_id or name to inspect output, readiness, running state, and exit code.`

const stopShellDescription = `Stop a managed shell process from the current run.

The harness first sends a soft interrupt and then kills the process if it does
not exit before timeout_seconds.`
