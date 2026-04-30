//go:build windows

package daemon

import (
	"context"
	"os"
	"os/signal"
)

// NotifyContext returns a context canceled by interrupt signals.
func NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt)
}
