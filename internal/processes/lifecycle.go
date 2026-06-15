package processes

import (
	"os"
	"os/exec"
	"strings"
	"time"
)

// safeInheritedEnv оставляет subprocess только базовое системное окружение.
var safeInheritedEnv = map[string]bool{
	"ComSpec":     true,
	"HOME":        true,
	"LANG":        true,
	"LC_ALL":      true,
	"PATH":        true,
	"PATHEXT":     true,
	"Path":        true,
	"SystemRoot":  true,
	"TEMP":        true,
	"TMP":         true,
	"TMPDIR":      true,
	"USER":        true,
	"USERPROFILE": true,
	"WINDIR":      true,
}

// PrepareCommand изолирует subprocess так, чтобы lifecycle мог чистить потомков.
func PrepareCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.Env = processEnv(cmd.Env)
	prepareCommand(cmd)
}

// StopCommand мягко останавливает command и затем чистит оставшуюся process tree.
func StopCommand(cmd *exec.Cmd, done <-chan struct{}, timeout time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if timeout <= 0 {
		timeout = defaultStopWait
	}
	interruptCommand(cmd, os.Interrupt)
	if waitDone(done, timeout) {
		killCommand(cmd)
		return
	}
	killCommand(cmd)
	_ = waitDone(done, timeout)
}

// CleanupCommand добивает оставшихся потомков после обычного выхода родителя.
func CleanupCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	killCommand(cmd)
}

func waitDone(done <-chan struct{}, timeout time.Duration) bool {
	if done == nil {
		return false
	}
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func processEnv(explicit []string) []string {
	env := explicit
	if env == nil {
		env = os.Environ()
	}
	out := []string{}
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok || strings.HasPrefix(key, "MADHARNESS_MINI_") || !safeInheritedEnv[key] {
			continue
		}
		out = append(out, key+"="+value)
	}
	return out
}
