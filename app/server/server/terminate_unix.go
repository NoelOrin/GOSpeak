//go:build unix

package server

import (
	"os"
	"syscall"
)

// terminateSelf requests a graceful shutdown of the current process by forwarding
// SIGTERM, which the server signal handler turns into a clean exit. Used by the
// Agent leader-lock guard to avoid split-brain when the lock is lost.
func terminateSelf() {
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}
