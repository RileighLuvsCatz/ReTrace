//go:build !linux && !windows

package window

import (
	"context"
	"fmt"
	"runtime"
)

// NewProvider returns an unsupported active-window provider.
func NewProvider() Provider {
	return unsupportedProvider{}
}

type unsupportedProvider struct{}

func (unsupportedProvider) CurrentWindow(context.Context) (Info, error) {
	return Info{}, fmt.Errorf("active window tracking is not supported on %s", runtime.GOOS)
}
