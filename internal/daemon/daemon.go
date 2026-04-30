package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/RileighLuvsCatz/retrace/db"
	"github.com/RileighLuvsCatz/retrace/internal/collector"
	"github.com/RileighLuvsCatz/retrace/internal/window"
)

const (
	DefaultDBPath       = "trace.db"
	DefaultPIDPath      = "retrace.pid"
	DefaultLogPath      = "retrace-daemon.log"
	DefaultPollInterval = collector.DefaultPollInterval
)

// Config controls daemon process and collection settings.
type Config struct {
	DBPath       string
	PIDPath      string
	LogPath      string
	PollInterval time.Duration
}

// DefaultConfig returns local Phase 2 defaults.
func DefaultConfig() Config {
	return Config{
		DBPath:       DefaultDBPath,
		PIDPath:      DefaultPIDPath,
		LogPath:      DefaultLogPath,
		PollInterval: DefaultPollInterval,
	}
}

// Status reports whether a PID-file daemon appears to be running.
func Status(config Config) (int, bool, error) {
	pid, err := readPID(config.PIDPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, false, nil
		}

		return 0, false, err
	}

	running := processRunning(pid)
	if !running {
		_ = os.Remove(config.PIDPath)
	}

	return pid, running, nil
}

// Start launches a background daemon process and writes its PID file.
func Start(config Config) error {
	if pid, running, err := Status(config); err != nil {
		return err
	} else if running {
		return fmt.Errorf("daemon already running with pid %d", pid)
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}

	logFile, err := os.OpenFile(config.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open daemon log: %w", err)
	}
	defer logFile.Close()

	command := exec.Command(
		executable,
		"daemon",
		"--db", config.DBPath,
		"--pid-file", config.PIDPath,
		"--log-file", config.LogPath,
		"--poll-interval", config.PollInterval.String(),
		"run",
	)
	command.Stdout = logFile
	command.Stderr = logFile
	configureBackgroundProcess(command)

	if err := command.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	if err := writePID(config.PIDPath, command.Process.Pid); err != nil {
		_ = command.Process.Kill()
		return err
	}

	return command.Process.Release()
}

// Stop terminates the background daemon process and removes its PID file.
func Stop(config Config) error {
	pid, err := readPID(config.PIDPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	if !processRunning(pid) {
		_ = os.Remove(config.PIDPath)
		return nil
	}

	if err := terminateProcess(pid); err != nil {
		return err
	}

	return os.Remove(config.PIDPath)
}

// RunForeground runs collection in the current process until interrupted.
func RunForeground(ctx context.Context, config Config) error {
	if err := writePID(config.PIDPath, os.Getpid()); err != nil {
		return err
	}
	defer os.Remove(config.PIDPath)

	ctx, stop := NotifyContext(ctx)
	defer stop()

	store, err := db.Open(ctx, config.DBPath)
	if err != nil {
		return err
	}
	defer store.Close()

	return collector.Run(ctx, store, window.NewProvider(), collector.Config{
		PollInterval: config.PollInterval,
	})
}

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("read pid file: %w", err)
	}
	if pid <= 0 {
		return 0, fmt.Errorf("read pid file: invalid pid %d", pid)
	}

	return pid, nil
}

func writePID(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o644)
}
