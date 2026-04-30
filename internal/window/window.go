package window

import "context"

// Info describes the currently focused application window.
type Info struct {
	App   string
	Title string
}

// Provider reads the currently focused application window.
type Provider interface {
	CurrentWindow(ctx context.Context) (Info, error)
}
