package main

import (
	"sync/atomic"

	"github.com/egoist/mygo"
)

// quitting is set when the user picked "quit" (tray menu, Cmd+Q, app menu).
// Until then, closing the main window hides it and the process keeps running
// in the tray.
var quitting atomic.Bool

// showMainWindow brings the main window back from the tray, restoring it when
// minimized, and gives it focus.
func showMainWindow() {
	if mainWindow == nil {
		return
	}
	mainWindow.Show()
	mainWindow.Restore()
	mainWindow.Focus()
}

// hideMainWindow hides the main window without closing it, so the process
// (and its socket, media session and tray icon) stays alive.
func hideMainWindow() {
	if mainWindow != nil {
		mainWindow.Hide()
	}
}

// trayMenu builds the tray menu: open the window, hide it, or quit for real.
func trayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{
			Label: "打开 GOSpeak",
			Click: func(*mygo.MenuItem, *mygo.Window) { showMainWindow() },
		},
		{
			Label: "隐藏到托盘",
			Click: func(*mygo.MenuItem, *mygo.Window) { hideMainWindow() },
		},
		{
			Label: "锁定/解锁覆盖层位置",
			Click: func(*mygo.MenuItem, *mygo.Window) { toggleOverlayLock() },
		},
		mygo.Separator(),
		{
			Label: "退出",
			Click: func(*mygo.MenuItem, *mygo.Window) { requestQuit() },
		},
	})
}

// initLifecycle wires the "close hides, tray keeps running" behaviour and the
// entry points that bring the window back.
func initLifecycle() {
	// Closing the last window must not quit: the tray keeps the app alive.
	mygo.App.OnWindowAllClosed(func() {})

	// Overlay configuration writes are debounced (overlay.go); quitting must
	// flush a change made right before the user closed the app.
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { flushOverlayConfig() })

	if mainWindow != nil {
		mainWindow.OnClose(func(e *mygo.CloseEvent) {
			if quitting.Load() {
				return
			}
			e.PreventDefault()
			hideMainWindow()
		})
	}

	// Dock icon click (macOS) reopens the hidden window.
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			showMainWindow()
		}
	})

	// A second launch focuses the running instance instead of starting a new
	// process.
	mygo.App.OnSecondInstance(func([]string, string) { showMainWindow() })
}

// requestQuit lets the close handler through and quits the app.
func requestQuit() {
	quitting.Store(true)
	mygo.App.Quit()
}
