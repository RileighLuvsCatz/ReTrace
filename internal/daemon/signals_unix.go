//go:build !windows

package daemon

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// NotifyContext returns a context canceled by interrupt or terminate signals.
func NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
