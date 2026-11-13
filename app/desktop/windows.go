package main

import (
	"net/url"
	"os"

	"github.com/egoist/mygo"
)

// devWindowURL is used only when developing with `mygo dev` plus
// GOSPEAK_DESKTOP_DEV=1: the window loads the Vite dev server directly (HMR),
// whose built-in proxy keeps /api and /ws same-origin. Keep in sync with
// devUrl in mygo.config.ts.
const devWindowURL = "http://localhost:3000"

func isDevMode() bool {
	return os.Getenv("GOSPEAK_DESKTOP_DEV") == "1"
}

// appPageBase returns where the web frontend is loaded from:
//   - dev:  the Vite dev server (same-origin API via its proxy);
//   - release: the loopback proxy (embedded dist + API/WS reverse proxy),
//     so the page never issues a cross-origin request.
func appPageBase() string {
	if isDevMode() {
		return devWindowURL
	}
	return proxyLoopbackURL
}

var mainWindow *mygo.Window

// originOf returns "scheme://host" origins for PageOptions.TrustedOrigins so a
// page loaded from an http URL may call the bound Go services.
func originOf(baseURL string) []string {
	if baseURL == "" {
		return nil
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return nil
	}
	return []string{u.Scheme + "://" + u.Host}
}

// titleBarHeight is the height of the title bar the page draws under the
// window controls: the same 28px as the web build's Header (px-2 py-0.5 with
// a py-1 text-xs button), so both builds look alike. Keep in sync with the
// fallback in app/web/src/components/titleBar/appTitleBar.tsx.
const titleBarHeight = 28

// appTitle is the native window title: the taskbar, Mission Control and the
// window switcher show the app name only, whatever route the page is on.
const appTitle = "GOSpeak"

func openMainWindow() {
	base := appPageBase()
	mainWindow = mygo.NewWindow(mygo.WindowOptions{
		Title: appTitle,
		URL:   base + "/",
		// Hides the title bar but keeps the window controls (traffic lights on
		// macOS, the Windows 11 buttons, GTK's title buttons on Linux) over the
		// page, which draws the rest of the bar. The page keeps clear of them
		// with the --mygo-titlebar-* CSS variables and drags the window by
		// --app-region: drag.
		TitleBarStyle:  mygo.TitleBarHidden,
		TitleBarHeight: titleBarHeight,
		// macOS: centers the traffic lights in the title bar
		// (button height 14 + 2*7 = 28).
		TrafficLightPosition: &mygo.Point{X: 20, Y: 7},
		// Fills the window until the page paints, in the page's --background,
		// so loading does not flash another color.
		BackgroundColor: "light-dark(#f8f8f8, #17181b)",
		// Lets the loopback page call bound Go methods (server config, overlay,
		// capabilities). Verify the field name against the mygo version in use.
		Page: mygo.PageOptions{TrustedOrigins: originOf(base)},
		// First-launch size; StateKey's remembered bounds (position, size,
		// maximized, full screen) override it and fall back to a visible
		// position when the saved display is gone.
		Width:     1200,
		Height:    800,
		MinWidth:  960,
		MinHeight: 640,
		StateKey:  "main",
	})
	// The page's <title> no longer renames the native window.
	if p := mainWindow.Page(); p != nil {
		p.OnPageTitleUpdated(func(e *mygo.TitleEvent) { e.PreventDefault() })
	}
}

// reloadAllWindows reloads the pages in place after a base-URL switch. The
// page origin is the loopback proxy both before and after, so a reload — not a
// window rebuild — is all that is needed. The overlay draws native UI rather
// than a page, so it has nothing to reload.
func reloadAllWindows() {
	if mainWindow != nil {
		if p := mainWindow.Page(); p != nil {
			_ = p.LoadURL(appPageBase() + "/")
		}
	}
}
