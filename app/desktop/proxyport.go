package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/egoist/mygo"
)

// The loopback proxy's port must stay stable across launches: the page's
// localStorage and session cookies are keyed by origin, which includes the
// port. A random port per launch would drop the cached user, the session
// expiry and the theme on every start.
//
// The range sits below 49152, where macOS and Windows start their ephemeral
// ports, so a fixed port here is unlikely to collide with a transient one.
const (
	proxyPortFirst = 47000
	proxyPortLast  = 47099
)

const proxyPortFileName = "proxy-port.json"

type proxyPortFile struct {
	Port int `json:"port"`
}

// loadSavedPort returns the port persisted in dir, or 0 when there is none or
// the file is unreadable/damaged.
func loadSavedPort(dir string) int {
	data, err := os.ReadFile(filepath.Join(dir, proxyPortFileName))
	if err != nil {
		return 0
	}
	var f proxyPortFile
	if json.Unmarshal(data, &f) != nil {
		return 0
	}
	if f.Port < proxyPortFirst || f.Port > proxyPortLast {
		return 0
	}
	return f.Port
}

// savePort persists port in dir. Failures are non-fatal: the next launch just
// picks a port again.
func savePort(dir string, port int) {
	if dir == "" || port == 0 {
		return
	}
	encoded, err := json.MarshalIndent(proxyPortFile{Port: port}, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, proxyPortFileName), encoded, 0o644)
}

// proxyPortDir is the user-data directory holding proxy-port.json.
func proxyPortDir() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return dir
}

// listenLoopback binds a loopback listener on the saved port when possible,
// then on each port of the range in turn, and finally on any free port.
func listenLoopback(dir string) (net.Listener, int, error) {
	saved := loadSavedPort(dir)
	candidates := make([]int, 0, proxyPortLast-proxyPortFirst+2)
	if saved != 0 {
		candidates = append(candidates, saved)
	}
	for p := proxyPortFirst; p <= proxyPortLast; p++ {
		if p == saved {
			continue
		}
		candidates = append(candidates, p)
	}
	for _, p := range candidates {
		ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
		if err == nil {
			if p != saved {
				savePort(dir, p)
			}
			return ln, p, nil
		}
	}
	// Every port in the range is taken: fall back to a system-assigned one.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("listen loopback: %w", err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return nil, 0, fmt.Errorf("listen loopback: unexpected address %T", ln.Addr())
	}
	return ln, addr.Port, nil
}
