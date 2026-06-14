package policy

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ShellAllowed проверяет, можно ли выполнить command как один subprocess.
func (p *Policy) ShellAllowed(command string) (bool, string) {
	decision := p.ShellDecision(command)
	if decision.Allowed {
		return true, ""
	}
	return false, decision.Reason
}

// ShellDecision проверяет команду и помечает policy-отказы, которые можно спросить.
func (p *Policy) ShellDecision(command string) Decision {
	if strings.TrimSpace(command) == "" {
		return deny(CodeEmptyShellCommand, "empty shell command", false)
	}
	parsed, err := ParseCommand(command)
	if err != nil {
		return deny(CodeInvalidShellCommand, fmt.Sprintf("invalid shell command: %v", err), false)
	}
	args := parsed.Args
	if len(args) == 0 {
		return deny(CodeEmptyShellCommand, "empty shell command", false)
	}
	if !p.cfg.Data.AllowShell {
		return deny(CodeShellDisabled, "shell disabled by config", true)
	}
	if len(parsed.ControlOperators) > 0 {
		return deny(CodeShellControlOperator, "shell control operators are denied", true)
	}
	if riskyCommand(args) {
		return deny(CodeRiskyShellCommand, "risky shell command denied", true)
	}
	return allow()
}

func riskyCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	name := commandBase(args[0])
	switch {
	case name == "sudo" || name == "curl" || name == "wget" || name == "ssh" || name == "scp" || name == "dd":
		return true
	case name == "mkfs" || strings.HasPrefix(name, "mkfs."):
		return true
	case name == "rm":
		return rmHasRecursiveForce(args[1:])
	case name == "chmod":
		return chmodSetsWorldWritable(args[1:])
	default:
		return false
	}
}

func commandBase(command string) string {
	base := filepath.Base(command)
	if base == "." || base == string(filepath.Separator) {
		base = command
	}
	return strings.ToLower(base)
}

func rmHasRecursiveForce(args []string) bool {
	recursive := false
	force := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--recursive" {
			recursive = true
			continue
		}
		if arg == "--force" {
			force = true
			continue
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			continue
		}
		for _, flag := range strings.TrimLeft(arg, "-") {
			if flag == 'r' || flag == 'R' {
				recursive = true
			}
			if flag == 'f' {
				force = true
			}
		}
	}
	return recursive && force
}

func chmodSetsWorldWritable(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		mode := strings.TrimLeft(arg, "0")
		if mode == "777" {
			return true
		}
	}
	return false
}
