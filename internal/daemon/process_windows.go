//go:build windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func configureBackgroundProcess(command *exec.Cmd) {}

func processRunning(pid int) bool {
	const processQueryLimitedInformation = 0x1000

	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)

	return true
}

func terminateProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find daemon process: %w", err)
	}

	if err := process.Kill(); err != nil {
		return fmt.Errorf("stop daemon: %w", err)
	}

	return nil
}
