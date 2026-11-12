//go:build windows

package server

import "os"

// terminateSelf forcibly stops the current process. Windows has no self-SIGTERM,
// so the Agent leader-lock guard exits immediately instead of a graceful shutdown.
func terminateSelf() {
	os.Exit(1)
}
