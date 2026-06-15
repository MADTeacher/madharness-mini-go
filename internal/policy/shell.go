package policy

import (
	"fmt"
	"path/filepath"
	"strings"
)

// maxShellWrapperDepth останавливает цепочки env/sh wrappers до рекурсии.
const maxShellWrapperDepth = 6

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
	return p.ShellDecisionInDir(command, p.root)
}

// ShellDecisionInDir проверяет command с учётом cwd для файловых аргументов.
func (p *Policy) ShellDecisionInDir(command string, cwd string) Decision {
	return p.shellDecisionInDir(command, cwd, 0)
}

func (p *Policy) shellDecisionInDir(command string, cwd string, depth int) Decision {
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
	return p.shellArgvDecision(args, parsed.ControlOperators, cwd, depth)
}

func (p *Policy) shellArgvDecision(args []string, controlOperators []string, cwd string, depth int) Decision {
	if len(args) == 0 {
		return deny(CodeEmptyShellCommand, "empty shell command", false)
	}
	if !p.cfg.Data.AllowShell {
		return deny(CodeShellDisabled, "shell disabled by config", true)
	}
	if len(controlOperators) > 0 {
		return deny(CodeShellControlOperator, "shell control operators are denied", true)
	}
	if depth > maxShellWrapperDepth {
		return deny(CodeRiskyShellCommand, "too many shell command wrappers", true)
	}
	if decision, handled := p.wrapperDecision(args, cwd, depth); handled {
		return decision
	}
	if riskyCommand(args) {
		return deny(CodeRiskyShellCommand, "risky shell command denied", true)
	}
	if decision := p.shellPathArgsDecision(args, cwd); !decision.Allowed {
		return decision
	}
	return allow()
}

func (p *Policy) wrapperDecision(args []string, cwd string, depth int) (Decision, bool) {
	name := commandBase(args[0])
	if name == "env" {
		wrapped, decision, ok := envWrappedCommand(args)
		if decision.Code != "" {
			return decision, true
		}
		if !ok {
			return Decision{}, false
		}
		return p.shellArgvDecision(wrapped, nil, cwd, depth+1), true
	}
	if script, ok := shellInterpreterCommand(name, args[1:]); ok {
		decision := p.shellDecisionInDir(script, cwd, depth+1)
		if !decision.Allowed {
			return decision, true
		}
		return deny(CodeShellControlOperator, "shell interpreter wrappers are denied", true), true
	}
	return Decision{}, false
}

func envWrappedCommand(args []string) ([]string, Decision, bool) {
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			if i+1 < len(args) {
				return args[i+1:], Decision{}, true
			}
			return nil, Decision{}, false
		case arg == "-S" || strings.HasPrefix(arg, "-S") || arg == "--split-string":
			return nil, deny(CodeRiskyShellCommand, "env split-string wrapper denied", true), false
		case arg == "-" || arg == "-i" || arg == "--ignore-environment" || arg == "-0" || arg == "--null":
			continue
		case arg == "-u" || arg == "--unset" || arg == "-C" || arg == "--chdir":
			i++
			continue
		case strings.HasPrefix(arg, "-u") || strings.HasPrefix(arg, "--unset="):
			continue
		case strings.HasPrefix(arg, "-C") || strings.HasPrefix(arg, "--chdir="):
			continue
		case isEnvAssignment(arg):
			continue
		case strings.HasPrefix(arg, "-"):
			return nil, Decision{}, false
		default:
			return args[i:], Decision{}, true
		}
	}
	return nil, Decision{}, false
}

func shellInterpreterCommand(name string, args []string) (string, bool) {
	if !isShellInterpreter(name) {
		return "", false
	}
	for i, arg := range args {
		if arg == "-c" || arg == "--command" {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", true
		}
		if strings.HasPrefix(arg, "-c") && arg != "-c" {
			return strings.TrimPrefix(arg, "-c"), true
		}
	}
	return "", false
}

func isShellInterpreter(name string) bool {
	switch name {
	case "sh", "bash", "zsh", "dash", "ksh", "fish":
		return true
	default:
		return false
	}
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

func (p *Policy) shellPathArgsDecision(args []string, cwd string) Decision {
	for _, arg := range args[1:] {
		for _, candidate := range p.shellPathCandidates(cwd, arg) {
			decision := p.SafePathDecisionFrom(cwd, candidate)
			if !decision.Allowed {
				return decision.Decision
			}
		}
	}
	return allow()
}

func (p *Policy) shellPathCandidates(cwd string, arg string) []string {
	if arg == "" || arg == "--" {
		return nil
	}
	if strings.HasPrefix(arg, "--") {
		_, value, ok := strings.Cut(arg, "=")
		if !ok {
			return nil
		}
		if shellPathLike(value) || p.shellArgNamesProtected(cwd, value) {
			return []string{value}
		}
		return nil
	}
	if strings.HasPrefix(arg, "-") || isEnvAssignment(arg) {
		return nil
	}
	if shellPathLike(arg) || p.shellArgNamesProtected(cwd, arg) {
		return []string{arg}
	}
	return nil
}

func (p *Policy) shellArgNamesProtected(cwd string, raw string) bool {
	decision := p.SafePathDecisionFrom(cwd, raw)
	return !decision.Allowed && decision.Code == CodeProtectedPath
}

func shellPathLike(arg string) bool {
	if strings.Contains(arg, "://") {
		return false
	}
	if filepath.IsAbs(arg) || arg == "." || arg == ".." || arg == "~" || strings.HasPrefix(arg, "~/") {
		return true
	}
	if strings.HasPrefix(arg, ".") {
		return true
	}
	return strings.Contains(arg, "/") || strings.Contains(arg, `\`)
}

func isEnvAssignment(arg string) bool {
	key, _, ok := strings.Cut(arg, "=")
	return ok && key != "" && !strings.ContainsAny(key, `/\`)
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
