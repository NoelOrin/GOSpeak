package main

// Startup tracing for the desktop host.
//
// Set GOSPEAK_STARTUP_DEBUG=1 to print a timeline of the launch path
// (process start → app ready → loopback proxy → window → first frontend
// request → React mounted) to stderr. Off by default; the cost is one env
// lookup per mark.
//
// GOSPEAK_STARTUP_EXIT_MS=<ms> quits the app that long after launch, which
// together with the flag above makes an unattended launch-time measurement
// possible (`GOSPEAK_STARTUP_DEBUG=1 GOSPEAK_STARTUP_EXIT_MS=8000 ./gospeak-desktop`).

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/egoist/mygo"
)

// startupT0 is set as early as Go package initialization allows: it misses the
// runtime/dyld part of the launch, but covers everything this package controls.
var startupT0 = time.Now()

func startupDebug() bool { return os.Getenv("GOSPEAK_STARTUP_DEBUG") == "1" }

// startupMark logs elapsed time since process start. No-op unless tracing is on.
func startupMark(format string, args ...any) {
	if !startupDebug() {
		return
	}
	log.Printf("[startup] +%-8s %s",
		time.Since(startupT0).Round(100*time.Microsecond),
		fmt.Sprintf(format, args...))
}

// startupProbeFrontend polls the page until React has mounted something under
// #app, giving an end-to-end number for "the window shows the app" that the Go
// side otherwise cannot observe (mygo exposes no load/paint event). It also
// reports the page's own clock (performance.now at mount), which isolates the
// frontend's boot cost from WebKit process-start variance.
func startupProbeFrontend(w *mygo.Window) {
	if !startupDebug() || w == nil {
		return
	}
	const probe = `(() => {
		const el = document.getElementById('app');
		return el && el.childElementCount > 0 ? Math.round(performance.now()) : -1;
	})()`
	go func() {
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if p := w.Page(); p != nil {
				if v, err := p.Eval(probe); err == nil {
					if ms, ok := v.(float64); ok && ms >= 0 {
						startupMark("frontend mounted (#app rendered, page time %dms)", int(ms))
						return
					}
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		startupMark("frontend mount probe timed out")
	}()
}

// startupScheduleAutoQuit quits after GOSPEAK_STARTUP_EXIT_MS (measurement only).
func startupScheduleAutoQuit() {
	raw := os.Getenv("GOSPEAK_STARTUP_EXIT_MS")
	if raw == "" {
		return
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		return
	}
	time.AfterFunc(time.Duration(ms)*time.Millisecond, func() {
		startupMark("auto-quit")
		requestQuit()
	})
}
