//go:build !windows

package daemon

import (
	"fmt"
	"os/exec"
	"syscall"
)

func configureBackgroundProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func processRunning(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func terminateProcess(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("stop daemon: %w", err)
	}

	return nil
}
