package processes

import (
	"os"
	"os/exec"
	"time"
)

// PrepareCommand изолирует subprocess так, чтобы lifecycle мог чистить потомков.
func PrepareCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
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
