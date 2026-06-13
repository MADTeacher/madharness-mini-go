package policy

import (
	"fmt"
	"strings"
)

// ShellAllowed проверяет, можно ли выполнить command как один subprocess.
func (p *Policy) ShellAllowed(command string) (bool, string) {
	if !p.cfg.Data.AllowShell {
		return false, "shell disabled by config"
	}
	lowered := strings.ToLower(command)
	denied := []string{"rm -rf", "sudo", "curl ", "wget ", "ssh ", "scp ", "chmod 777", "mkfs", " dd "}
	for _, fragment := range denied {
		if strings.Contains(" "+lowered+" ", fragment) {
			return false, "risky shell command denied"
		}
	}
	args, err := SplitCommand(command)
	if err != nil {
		return false, fmt.Sprintf("invalid shell command: %v", err)
	}
	if len(args) == 0 {
		return false, "empty shell command"
	}
	for _, token := range []string{"|", ">", "<", "&&", "||", ";"} {
		if strings.Contains(command, token) {
			return false, "shell control operators are denied"
		}
	}
	return true, ""
}
