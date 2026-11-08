package main

import (
	"context"
)

// UpdateInfo mirrors packages/desktop createUpdater.
type UpdateInfo struct {
	Version string `json:"version"`
	Notes   string `json:"notes,omitempty"`
}

// Updater drives signed auto-updates.
//
// Real auto-update needs a signed release channel (e.g. Sparkle on macOS /
// WebView2-era installer on Windows) wired at packaging time, so Check always
// reports "up to date" and Install returns a clear error until then.
type Updater struct{}

func (Updater) Check(ctx context.Context) *UpdateInfo { return nil }

func (Updater) Install(ctx context.Context) error {
	return unsupportedError("auto-update")
}
