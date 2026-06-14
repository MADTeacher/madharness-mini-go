package policy

const (
	// CodePathOutsideWorkspace не эскалируется: harness не выходит за workspace.
	CodePathOutsideWorkspace = "path_outside_workspace"
	// CodeProtectedPath можно эскалировать для model-invoked tools.
	CodeProtectedPath = "protected_path"
	// CodeInvalidShellCommand не эскалируется: команда не может быть разобрана.
	CodeInvalidShellCommand = "invalid_shell_command"
	// CodeEmptyShellCommand не эскалируется: запускать нечего.
	CodeEmptyShellCommand = "empty_shell_command"
	// CodeShellDisabled можно эскалировать явным согласием пользователя.
	CodeShellDisabled = "shell_disabled"
	// CodeRiskyShellCommand можно эскалировать явным согласием пользователя.
	CodeRiskyShellCommand = "risky_shell_command"
	// CodeShellControlOperator можно эскалировать, после чего команда пойдёт через shell.
	CodeShellControlOperator = "shell_control_operator"
)

// Decision описывает результат policy-проверки без потери причины отказа.
type Decision struct {
	Allowed     bool
	Escalatable bool
	Code        string
	Reason      string
}

func allow() Decision {
	return Decision{Allowed: true}
}

func deny(code string, reason string, escalatable bool) Decision {
	return Decision{Allowed: false, Escalatable: escalatable, Code: code, Reason: reason}
}
