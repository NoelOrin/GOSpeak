package main

// Local loopback reverse proxy — the heart of "desktop ships its own web UI".
//
// The desktop window loads http://127.0.0.1:<port>/ served by THIS process, so
// the page and its API/WS calls share one origin (no CORS, cookies work
// normally). This server:
//   - serves the embedded web frontend (webui/dist, SPA history fallback);
//   - reverse-proxies /api/v1, /uploads, /livekit and /ws to the configured
//     backend base URL (server-to-server, browser same-origin policy does not
//     apply);
//   - rewrites Set-Cookie (strips Secure/Domain) so backend session cookies
//     stick to the loopback origin;
//   - forwards /wsworker?target=<ws-url> to SFU/signaling worker nodes,
//     carrying the session cookie the page sends (same origin).
//
// Changing the base URL simply swaps the proxy target — the page never sees a
// cross-origin request.

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"sync"
)

//go:embed all:webui/dist
var embeddedDist embed.FS

type proxyServer struct {
	mu     sync.RWMutex
	target *url.URL

	dist      fs.FS
	mainProxy *httputil.ReverseProxy

	workerMu    sync.Mutex
	workerProxy map[string]*httputil.ReverseProxy
}

// startProxyServer listens on the persisted loopback port (see proxyport.go)
// and returns the server plus its "http://127.0.0.1:<port>" base URL.
func startProxyServer() (*proxyServer, string, error) {
	sub, err := fs.Sub(embeddedDist, "webui/dist")
	if err != nil {
		return nil, "", fmt.Errorf("open embedded dist: %w", err)
	}

	s := &proxyServer{
		dist:        sub,
		workerProxy: map[string]*httputil.ReverseProxy{},
	}
	s.mainProxy = &httputil.ReverseProxy{
		Director:       s.directMain,
		ModifyResponse: fixCookies,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/", s.handleProxied)
	mux.HandleFunc("/uploads/", s.handleProxied)
	// LiveKit 信令同源反代：后端下发的 serverUrl 是 ws://127.0.0.1:<port>/livekit，
	// 由这里转给用户配置的后端（后端再转发到内部 LiveKit）。
	mux.HandleFunc("/livekit/", s.handleProxied)
	mux.HandleFunc("/rtc/v1/", s.handleProxied)
	// Signal socket (page resolves ws://127.0.0.1:<port>/ws as same origin).
	mux.HandleFunc("/ws", s.handleProxied)
	// SFU / signaling worker nodes reached through us with the session cookie.
	mux.HandleFunc("/wsworker", s.handleWorker)
	mux.HandleFunc("/", s.serveDist)

	listener, port, err := listenLoopback(proxyPortDir())
	if err != nil {
		return nil, "", err
	}
	go func() {
		_ = http.Serve(listener, mux)
	}()
	return s, fmt.Sprintf("http://127.0.0.1:%d", port), nil
}

// setProxyTarget points the proxy at a new backend base URL ("" = unset).
func (s *proxyServer) setProxyTarget(baseURL string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(baseURL) == "" {
		s.target = nil
		return
	}
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Host == "" {
		s.target = nil
		return
	}
	s.target = u
}

func (s *proxyServer) directMain(req *http.Request) {
	s.mu.RLock()
	t := s.target
	s.mu.RUnlock()
	if t == nil {
		return // handled by handleProxied with a 503 before reaching here
	}
	// Tell the backend which origin the page actually uses (the loopback one),
	// so serverUrl comes back as ws://127.0.0.1:<port>/livekit and stays
	// same-origin instead of pointing the window straight at the remote host.
	if req.Header.Get("X-Forwarded-Host") == "" {
		req.Header.Set("X-Forwarded-Host", req.Host)
	}
	if req.Header.Get("X-Forwarded-Proto") == "" {
		proto := "http"
		if req.TLS != nil {
			proto = "https"
		}
		req.Header.Set("X-Forwarded-Proto", proto)
	}
	req.URL.Scheme = t.Scheme
	req.URL.Host = t.Host
	req.Host = t.Host
	if t.Path != "" && t.Path != "/" {
		// Base URL may carry a path prefix; keep it in front.
		req.URL.Path = strings.TrimSuffix(t.Path, "/") + req.URL.Path
	}
}

// handleProxied forwards API/upload/RTC/WS traffic to the configured backend.
func (s *proxyServer) handleProxied(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	t := s.target
	s.mu.RUnlock()
	if t == nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 6001,
			"msg":  "server base url not configured",
			"data": nil,
		})
		return
	}
	s.mainProxy.ServeHTTP(w, r)
}

// handleWorker forwards a WebSocket connection to the node given via the
// "target" query parameter (an absolute ws/wss URL returned by ws-endpoint).
func (s *proxyServer) handleWorker(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("target")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		http.Error(w, "invalid target", http.StatusBadRequest)
		return
	}
	// Normalize to http(s) for ReverseProxy (it handles the WS upgrade).
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	case "http", "https":
	default:
		http.Error(w, "unsupported scheme", http.StatusBadRequest)
		return
	}
	u.RawQuery = "" // target's query belongs to the remote handshake, not here

	proxy := s.workerFor(u)
	proxy.ServeHTTP(w, r)
}

func (s *proxyServer) workerFor(u *url.URL) *httputil.ReverseProxy {
	key := u.Scheme + "://" + u.Host + u.Path
	s.workerMu.Lock()
	defer s.workerMu.Unlock()
	if p, ok := s.workerProxy[key]; ok {
		return p
	}
	p := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			req.URL.Path = u.Path
			req.Host = u.Host
		},
		ModifyResponse: fixCookies,
	}
	s.workerProxy[key] = p
	return p
}

// fixCookies rewrites backend Set-Cookie headers so they stick to the loopback
// origin: a Secure cookie cannot be stored over plain http, and a Domain
// attribute would break host-only scoping. SameSite is normalized to Lax (the
// page talks to us same-origin, so Lax is all that is needed).
func fixCookies(resp *http.Response) error {
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return nil
	}
	resp.Header.Del("Set-Cookie")
	for _, c := range cookies {
		c.Secure = false
		c.Domain = ""
		c.SameSite = http.SameSiteLaxMode
		resp.Header.Add("Set-Cookie", c.String())
	}
	return nil
}

// serveDist serves the embedded frontend with an SPA history fallback: any
// path without a file extension falls back to index.html so routes like
// /overlay or /login work on deep links.
func (s *proxyServer) serveDist(w http.ResponseWriter, r *http.Request) {
	name := path.Clean(r.URL.Path)
	name = strings.TrimPrefix(name, "/")
	if name == "" || name == "." {
		s.serveIndex(w)
		return
	}
	f, err := s.dist.Open(name)
	if err != nil {
		s.serveIndex(w)
		return
	}
	defer func() { _ = f.Close() }()
	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		s.serveIndex(w)
		return
	}
	seeker, ok := f.(io.ReadSeeker)
	if !ok {
		s.serveIndex(w)
		return
	}
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), seeker)
}

func (s *proxyServer) serveIndex(w http.ResponseWriter) {
	f, err := s.dist.Open("index.html")
	if err != nil {
		http.Error(w, "frontend not built", http.StatusInternalServerError)
		return
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "frontend not readable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

// package-level singleton so server_config.go can retarget the proxy.
var loopbackProxy *proxyServer

// proxyLoopbackURL is the "http://127.0.0.1:<port>" the windows load.
var proxyLoopbackURL string

// ensureProxyStarted boots the loopback proxy once and returns its base URL.
func ensureProxyStarted() (string, error) {
	if loopbackProxy != nil {
		return proxyLoopbackURL, nil
	}
	s, base, err := startProxyServer()
	if err != nil {
		return "", err
	}
	loopbackProxy = s
	proxyLoopbackURL = base
	// Apply the persisted base URL (if any) loaded by loadServerConfig.
	loopbackProxy.setProxyTarget(currentBaseURL())
	return proxyLoopbackURL, nil
}
