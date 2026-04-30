//go:build windows

package window

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW        = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcID = user32.NewProc("GetWindowThreadProcessId")
)

// NewProvider returns the Windows active-window provider.
func NewProvider() Provider {
	return User32Provider{}
}

// User32Provider reads active window information through user32.dll.
type User32Provider struct{}

// CurrentWindow returns the active window title and process ID.
func (User32Provider) CurrentWindow(ctx context.Context) (Info, error) {
	select {
	case <-ctx.Done():
		return Info{}, ctx.Err()
	default:
	}

	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return Info{}, fmt.Errorf("no foreground window")
	}

	buffer := make([]uint16, 512)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))

	var pid uint32
	procGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

	return Info{
		App:   fmt.Sprintf("pid:%d", pid),
		Title: syscall.UTF16ToString(buffer),
	}, nil
}
