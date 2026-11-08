// GOSpeak Desktop — the desktop client host.
//
// Built with MyGo (https://github.com/egoist/mygo), a pure-Go desktop
// framework. The desktop ships its own copy of the web frontend (embedded in
// webui/dist) and connects to a user-configured backend through a LOCAL
// LOOPBACK REVERSE PROXY (proxyserver.go):
//
//	window ── loads ──▶ http://127.0.0.1:<port>  (this process)
//	                     ├─ serves the embedded frontend (same origin)
//	                     └─ proxies /api /ws /uploads → configured base URL
//
// Because the page, its API calls and its WebSocket all share the loopback
// origin, there is no CORS and backend HttpOnly cookies work exactly as they
// do in a browser pointed at the server.
//
// Run with the MyGo CLI:
//
//	pnpm --filter @gospeak/desktop-app dev     # mygo dev (live reload)
//	pnpm --filter @gospeak/desktop-app build   # mygo build (packaged app)
//
// For developing against the local Vite dev server instead of the embedded
// frontend: GOSPEAK_DESKTOP_DEV=1 mygo dev
package main

import (
	"log"

	"github.com/egoist/mygo"
)

// 构建时由 CI 通过 ldflags 注入（release.yml 的 desktop 发布链）：
//
//	-ldflags="-X 'main.version=<ver>' -X 'main.commit=<sha>'"
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	// One instance only: a second launch hands its command line to the running
	// app (OnSecondInstance focuses the window) and exits.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	// Bind the desktop capability services. MyGo turns each exported method into
	// a typed async call on the generated TypeScript client. The method names
	// below map to the command surface used by packages/desktop.
	mygo.Bind(Window{})
	mygo.Bind(Tray{})
	mygo.Bind(Notification{})
	mygo.Bind(AutoStart{})
	mygo.Bind(DeepLink{})
	mygo.Bind(Dialog{})
	mygo.Bind(Updater{})
	mygo.Bind(Overlay{})
	mygo.Bind(ServerConfig{})

	// Deep links (gospeak://invite/d/<CODE>) must be wired before Run so the
	// URL the app was launched with is delivered too.
	registerDeepLink()

	mygo.App.WhenReady(func() {
		// Load the persisted backend base URL before any window opens.
		loadServerConfig()

		if !isDevMode() {
			// Release: boot the loopback proxy and load the frontend from it,
			// so API/WS stay same-origin no matter which server is configured.
			// Dev loads the Vite dev server directly (HMR); its built-in proxy
			// keeps /api and /ws same-origin, so no loopback proxy is needed.
			if _, err := ensureProxyStarted(); err != nil {
				log.Printf("[GOSpeak] loopback proxy failed to start: %v", err)
			}
		}
		openMainWindow()

		// Dock / taskbar icon when running from source (a packaged bundle
		// already carries resources/icon.png).
		applyAppIcon()

		// Floating game-overlay window: disabled by default, created hidden.
		// Wires config persistence, global hotkeys and the auto policy loop.
		initOverlay()

		// System tray icon (menu bar / taskbar) with open & quit menu.
		ensureTray()

		// Closing the window hides it to the tray; only "quit" ends the app.
		initLifecycle()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
