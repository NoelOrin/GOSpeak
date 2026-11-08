//go:build !windows

package main

// isFullscreenAppInFront is only implemented on Windows (see
// overlay_fullscreen_windows.go). The macOS CoreGraphics/cgo implementation was
// removed to keep the desktop build cgo-free, so macOS reports "no": the
// overlay's auto policy simply won't auto-show on fullscreen games there, and
// the manual toggle / hotkey still work.
func isFullscreenAppInFront() bool { return false }
