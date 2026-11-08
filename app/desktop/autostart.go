package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// AutoStart controls whether GOSpeak launches on OS login.
//
// macOS uses a System Events login item pointing at the running executable
// (works for both `go build` binaries and packaged .app bundles). Other
// platforms report a clear "not implemented" error until wired (Linux:
// ~/.config/autostart; Windows: HKCU Run key).
type AutoStart struct{}

func autostartAppName() string {
	exe, err := os.Executable()
	if err != nil {
		return "GOSpeak"
	}
	return strings.TrimSuffix(baseName(exe), ".exe")
}

func baseName(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

func runOscript(script string) (string, error) {
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (AutoStart) IsEnabled(ctx context.Context) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	out, err := runOscript(
		`tell application "System Events" to get the name of every login item`,
	)
	if err != nil {
		return false
	}
	for _, name := range strings.Split(out, ",") {
		if strings.TrimSpace(name) == autostartAppName() {
			return true
		}
	}
	return false
}

func (AutoStart) Enable(ctx context.Context) error {
	if runtime.GOOS != "darwin" {
		return unsupportedError("auto-start")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	name := autostartAppName()
	// Replace any existing item so the path stays current.
	_, _ = runOscript(fmt.Sprintf(
		`tell application "System Events" to delete login item %q`, name))
	_, err = runOscript(fmt.Sprintf(
		`tell application "System Events" to make login item at end with properties {path:%q, hidden:false}`, exe))
	return err
}

func (AutoStart) Disable(ctx context.Context) error {
	if runtime.GOOS != "darwin" {
		return unsupportedError("auto-start")
	}
	_, err := runOscript(fmt.Sprintf(
		`tell application "System Events" to delete login item %q`, autostartAppName()))
	return err
}
