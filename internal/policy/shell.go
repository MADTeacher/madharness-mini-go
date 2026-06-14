package policy

import (
	"fmt"
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
	args, err := SplitCommand(command)
	if err != nil {
		return deny(CodeInvalidShellCommand, fmt.Sprintf("invalid shell command: %v", err), false)
	}
	if len(args) == 0 {
		return deny(CodeEmptyShellCommand, "empty shell command", false)
	}
	if !p.cfg.Data.AllowShell {
		return deny(CodeShellDisabled, "shell disabled by config", true)
	}
	for _, token := range []string{"|", ">", "<", "&&", "||", ";"} {
		if strings.Contains(command, token) {
			return deny(CodeShellControlOperator, "shell control operators are denied", true)
		}
	}
	lowered := strings.ToLower(command)
	denied := []string{"rm -rf", "sudo", "curl ", "wget ", "ssh ", "scp ", "chmod 777", "mkfs", " dd "}
	for _, fragment := range denied {
		if strings.Contains(" "+lowered+" ", fragment) {
			return deny(CodeRiskyShellCommand, "risky shell command denied", true)
		}
	}
	return allow()
}
