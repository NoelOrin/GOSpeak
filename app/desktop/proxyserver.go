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
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

//go:embed all:webui/dist
var embeddedDist embed.FS

type proxyServer struct {
	mu     sync.RWMutex
	target *url.URL

	dist      fs.FS
	mainProxy *httputil.ReverseProxy
	// transport is shared by the main and worker proxies so idle connections to
	// the backend (and to SFU nodes) are reused across both.
	transport *http.Transport

	// assets caches the embedded dist in memory: a page reload re-reads nothing
	// from disk, and gzip is computed once per file (see assetCache).
	assets *assetCache

	workerMu    sync.Mutex
	workerProxy map[string]*httputil.ReverseProxy
}

// proxyTransport is the connection pool both reverse proxies share.
//
// The default Transport only keeps 2 idle connections per host, which makes the
// page's burst of parallel API calls (and the overlay's polling) pay for a new
// TCP/TLS handshake every few requests; it also honours HTTP(S)_PROXY, which on
// a developer machine can hijack loopback→backend traffic that the user never
// meant to route through a proxy. Neither is what a desktop client wants.
func proxyTransport() *http.Transport {
	return &http.Transport{
		Proxy: nil, // talk to the configured backend directly, never via env proxy
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

// handler wires the loopback routes: proxied API/WS prefixes first, static
// frontend last (it owns "/" and the SPA fallback).
func (s *proxyServer) handler() http.Handler {
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
	return mux
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
		assets:      newAssetCache(),
		transport:   proxyTransport(),
		workerProxy: map[string]*httputil.ReverseProxy{},
	}
	s.mainProxy = &httputil.ReverseProxy{
		Director:       s.directMain,
		ModifyResponse: fixCookies,
		Transport:      s.transport,
		ErrorHandler:   proxyErrorHandler,
	}

	listener, port, err := listenLoopback(proxyPortDir())
	if err != nil {
		return nil, "", err
	}
	srv := &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		_ = srv.Serve(listener)
	}()
	// Pre-compress the frontend while the window is still being created, so the
	// first page load never waits on gzip. With build-time .gz sidecars (the
	// packaged app) this only warms the in-memory cache.
	go func() {
		s.assets.warm(s.dist)
		startupMark("asset cache warm")
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
		Transport:      s.transport,
		ErrorHandler:   proxyErrorHandler,
	}
	s.workerProxy[key] = p
	return p
}

// proxyErrorHandler answers an unreachable backend with the API's unified error
// envelope instead of ReverseProxy's bare 502 text, so the page's API client
// can tell "server is down" apart from a server-side error and show its own
// message (code 6002 = sfu/proxy transport error).
func proxyErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusBadGateway)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": 6002,
		"msg":  "backend unreachable",
		"data": nil,
	})
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
// /overlay or /login work on deep links. A missing URL *with* an extension is a
// real 404 — answering it with index.html would hand an HTML page to a <script>
// tag and turn a build mistake into a confusing MIME error.
func (s *proxyServer) serveDist(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		startupMark("first page request from the WebView")
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." || !fs.ValidPath(name) {
		s.serveIndex(w, r)
		return
	}
	// Compression sidecars written by the build are not public URLs.
	if strings.HasSuffix(name, ".gz") {
		http.NotFound(w, r)
		return
	}
	if s.serveAsset(w, r, name) == nil {
		return
	}
	// An SPA route has no extension and falls back to index.html; a missing
	// asset stays a 404.
	if path.Ext(name) == "" {
		s.serveIndex(w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *proxyServer) serveIndex(w http.ResponseWriter, r *http.Request) {
	if err := s.serveAsset(w, r, "index.html"); err != nil {
		http.Error(w, "frontend not built", http.StatusInternalServerError)
	}
}

// serveAsset writes one embedded file from the in-memory cache. It returns the
// error from loading the asset so callers can fall back (SPA routes) or 404.
//
// Headers set here:
//   - Cache-Control: hashed build output (assets/, chunks/, entries/) is
//     content-addressed, so it can be cached forever; index.html must always be
//     revalidated or an app update would keep loading stale chunk URLs;
//   - ETag + ServeContent: a reload revalidates with a 304 instead of
//     re-downloading ~2 MB of JS;
//   - Content-Encoding: gzip for text assets when the page accepts it (skipped
//     for Range requests, which address the identity encoding).
func (s *proxyServer) serveAsset(w http.ResponseWriter, r *http.Request, name string) error {
	a, err := s.assets.get(s.dist, name)
	if err != nil {
		return err
	}

	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", cacheControlFor(name))

	etag := a.etag
	body := a.body
	enc := "identity"
	if a.gz != nil && r.Header.Get("Range") == "" && acceptsGzip(r.Header.Get("Accept-Encoding")) {
		h.Set("Content-Encoding", "gzip")
		h.Set("Vary", "Accept-Encoding")
		etag = strings.TrimSuffix(a.etag, `"`) + `-gz"`
		body = a.gz
		enc = "gzip"
	}
	h.Set("ETag", etag)
	startupMark("serve %s (%d B %s)", name, len(body), enc)

	// ServeContent keeps Range, If-Range, If-None-Match and If-Modified-Since
	// handling (plus HEAD) off our hands.
	http.ServeContent(w, r, name, a.modTime, bytes.NewReader(body))
	return nil
}

// cacheControlFor picks the caching policy for a dist path.
func cacheControlFor(name string) string {
	switch {
	case name == "index.html":
		// Revalidate on every load: it references the hashed chunk names.
		return "no-cache"
	case strings.HasPrefix(name, "assets/"),
		strings.HasPrefix(name, "chunks/"),
		strings.HasPrefix(name, "entries/"):
		return "public, max-age=31536000, immutable"
	default:
		// Unhashed files (favicons, presets/): short cache, revalidate soon.
		return "public, max-age=3600"
	}
}

// acceptsGzip reports whether an Accept-Encoding header value allows gzip.
func acceptsGzip(v string) bool {
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if i := strings.IndexByte(part, ';'); i >= 0 {
			part = strings.TrimSpace(part[:i])
		}
		if strings.EqualFold(part, "gzip") {
			return true
		}
	}
	return false
}

// compressibleExtensions are the embedded asset types worth gzipping: text and
// wasm. Images, fonts and icons are already compressed containers.
var compressibleExtensions = map[string]bool{
	".js":   true,
	".mjs":  true,
	".css":  true,
	".html": true,
	".json": true,
	".svg":  true,
	".txt":  true,
	".xml":  true,
	".map":  true,
	".wasm": true,
}

// staticAsset is one file from the embedded dist, held in memory together with
// its gzip encoding and ETag so a page load is a memcpy, not a disk read.
type staticAsset struct {
	body    []byte
	gz      []byte // nil when not compressible / not smaller
	etag    string
	modTime time.Time
}

// assetCache is the read-through cache in front of the embedded dist.
//
// The whole frontend is ~2 MB, so keeping it (plus gzip) resident costs a few
// MB and removes every asset read from the reload path. Loads are serialized:
// they are rare (once per file per process) and this keeps the background
// warm-up from duplicating work the first page load is already doing.
type assetCache struct {
	mu    sync.Mutex
	items map[string]*staticAsset
}

func newAssetCache() *assetCache {
	return &assetCache{items: map[string]*staticAsset{}}
}

// get returns the cached asset, loading and compressing it on first use.
func (c *assetCache) get(dist fs.FS, name string) (*staticAsset, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a, ok := c.items[name]; ok {
		return a, nil
	}

	f, err := dist.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, fs.ErrNotExist
	}
	body, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(body)
	a := &staticAsset{
		body:    body,
		etag:    `"` + hex.EncodeToString(sum[:8]) + `"`,
		modTime: stat.ModTime(),
	}
	if compressibleExtensions[strings.ToLower(path.Ext(name))] && len(body) >= 512 {
		// Prefer the .gz sidecar the desktop build writes next to each asset
		// (see mygo.config.ts): then a launch compresses nothing at all. The
		// runtime fallback keeps `go run`, `mygo dev` and tests working from a
		// plain dist.
		if gz, err := fs.ReadFile(dist, name+".gz"); err == nil && len(gz) > 0 && len(gz) < len(body) {
			a.gz = gz
		} else if gz, err := gzipBytes(body); err == nil && len(gz) < len(body) {
			a.gz = gz
		}
	}
	c.items[name] = a
	return a, nil
}

// warm pre-loads and pre-compresses every compressible asset, so the first page
// load after launch hits a fully warm cache. Runs in the background; failures
// are irrelevant (the asset is simply loaded on demand later).
func (c *assetCache) warm(dist fs.FS) {
	_ = fs.WalkDir(dist, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(p, ".gz") {
			// .gz sidecars are read as part of their asset, not on their own.
			return nil
		}
		if compressibleExtensions[strings.ToLower(path.Ext(p))] {
			_, _ = c.get(dist, p)
		}
		return nil
	})
}

// gzipBytes compresses b at the default level: the balance point for one-off
// work on a few hundred KB of JS (level 9 buys a couple of percent for several
// times the CPU).
func gzipBytes(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(len(b) / 3)
	zw, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(b); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
