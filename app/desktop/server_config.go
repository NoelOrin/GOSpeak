package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
)

// ServerConfig exposes the backend base URL the desktop client connects to.
//
// The base URL is persisted to the app's user-data directory so it survives
// restarts and can be changed either before login (login-screen field) or from
// the settings UI. JSON tags must stay in sync with the TS side
// (packages/desktop/src/capabilities/server.ts and app/web/src/config/server.ts).
type ServerConfig struct{}

type serverConfigFile struct {
	BaseURL string `json:"baseUrl"`
}

var (
	serverConfigMu sync.Mutex
	serverBaseURL  string
)

const serverConfigFileName = "server-config.json"

// loadServerConfig reads the persisted base URL (if any) from user data. Safe to
// call once during startup; failures are non-fatal (base URL stays "").
func loadServerConfig() {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(dir, serverConfigFileName))
	if err != nil {
		return // no config yet
	}
	var cfg serverConfigFile
	if json.Unmarshal(data, &cfg) == nil {
		serverConfigMu.Lock()
		serverBaseURL = strings.TrimSpace(cfg.BaseURL)
		serverConfigMu.Unlock()
	}
}

// currentBaseURL returns the in-memory configured base URL ("" when unset).
func currentBaseURL() string {
	serverConfigMu.Lock()
	defer serverConfigMu.Unlock()
	return serverBaseURL
}

// GetBaseURL returns the configured backend base URL ("" when unset).
func (ServerConfig) GetBaseURL() string {
	return currentBaseURL()
}

// SetBaseURL persists a new backend base URL and immediately connects to it:
// the loopback proxy target is switched and the windows reload, so the page
// keeps a single same-origin view of the new server. An empty value clears the
// stored URL (proxy then answers API calls with 503 until configured).
func (ServerConfig) SetBaseURL(newURL string) {
	newURL = strings.TrimSpace(newURL)
	previous := currentBaseURL()

	// Persist to user data.
	dir, derr := mygo.App.Path(mygo.PathUserData)
	if derr == nil {
		serverConfigMu.Lock()
		serverBaseURL = newURL
		encoded, merr := json.MarshalIndent(serverConfigFile{BaseURL: newURL}, "", "  ")
		serverConfigMu.Unlock()
		if merr == nil {
			if mkErr := os.MkdirAll(dir, 0o755); mkErr == nil {
				_ = os.WriteFile(filepath.Join(dir, serverConfigFileName), encoded, 0o644)
			}
		}
	} else {
		// Cannot resolve user data; keep in memory only.
		serverConfigMu.Lock()
		serverBaseURL = newURL
		serverConfigMu.Unlock()
	}

	// Switching servers must not carry the previous server's session over:
	// the cookies live on the loopback origin, and the page keeps the cached
	// user plus the session expiry in localStorage. The overlay's avatars come
	// from the old server too, so their cache goes with it.
	if newURL != previous {
		_ = mygo.Cookies.Clear()
		clearPageSessionCache()
		overlayAvatars.clear()
	}

	// Apply live: swap proxy target, then reload the pages so they re-resolve
	// their session against the (new) server. Same origin before and after —
	// no CORS involved anywhere.
	if loopbackProxy != nil {
		loopbackProxy.setProxyTarget(newURL)
	}
	reloadAllWindows()
}

// clearPageSessionCache drops the cached user and session expiry the web app
// keeps in localStorage. They are keyed by the loopback origin, which stays the
// same across servers, so they have to be cleared explicitly.
func clearPageSessionCache() {
	if mainWindow == nil {
		return
	}
	p := mainWindow.Page()
	if p == nil {
		return
	}
	_, _ = p.Eval(`(() => {
		try {
			localStorage.removeItem("user");
			localStorage.removeItem("gospeak_session_expires_at");
		} catch {}
	})()`)
}
