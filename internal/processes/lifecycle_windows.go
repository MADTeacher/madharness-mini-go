//go:build windows

package processes

import (
	"os"
	"os/exec"
)

func prepareCommand(cmd *exec.Cmd) {
	_ = cmd
}

func interruptCommand(cmd *exec.Cmd, signal os.Signal) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(signal)
}

func killCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
