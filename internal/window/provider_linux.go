//go:build linux

package window

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// NewProvider returns the Linux active-window provider.
func NewProvider() Provider {
	return XDoToolProvider{}
}

// XDoToolProvider reads active window information through xdotool.
type XDoToolProvider struct{}

// CurrentWindow returns the active window title and owning process command.
func (XDoToolProvider) CurrentWindow(ctx context.Context) (Info, error) {
	windowID, err := runTrimmed(ctx, "xdotool", "getactivewindow")
	if err != nil {
		return Info{}, fmt.Errorf("get active window: %w", err)
	}

	title, err := runTrimmed(ctx, "xdotool", "getwindowname", windowID)
	if err != nil {
		return Info{}, fmt.Errorf("get active window title: %w", err)
	}

	pid, err := runTrimmed(ctx, "xdotool", "getwindowpid", windowID)
	if err != nil {
		return Info{}, fmt.Errorf("get active window pid: %w", err)
	}

	app, err := runTrimmed(ctx, "ps", "-p", pid, "-o", "comm=")
	if err != nil || app == "" {
		app = "pid:" + pid
	}

	return Info{App: app, Title: title}, nil
}

func runTrimmed(ctx context.Context, name string, args ...string) (string, error) {
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}
