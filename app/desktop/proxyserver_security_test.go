package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func securityTestDist() fstest.MapFS {
	return fstest.MapFS{
		"index.html": {
			Data:    []byte("<!doctype html><title>GOSpeak</title>"),
			ModTime: time.Now(),
		},
		"assets/index.abc123.css": {
			Data:    []byte(strings.Repeat("body{color:#fff}", 200)),
			ModTime: time.Now(),
		},
		"chunks/app.abc123.js.map": {
			Data:    []byte(strings.Repeat(`{"version":3,"mappings":"AAAA"}`, 60)),
			ModTime: time.Now(),
		},
	}
}

func newSecurityTestServer(allow func() []string) *proxyServer {
	return &proxyServer{
		dist:               securityTestDist(),
		assets:             newAssetCache(),
		transport:          proxyTransport(),
		workerProxy:        map[string]*httputil.ReverseProxy{},
		allowedWorkerHosts: allow,
	}
}

func newSecurityMainProxy(s *proxyServer) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director:       s.directMain,
		ModifyResponse: s.modifyMainResponse,
		Transport:      s.transport,
		ErrorHandler:   proxyErrorHandler,
	}
}

func serveWorker(t *testing.T, s *proxyServer, target, host string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/wsworker?target="+url.QueryEscape(target), nil)
	if host != "" {
		req.Host = host
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.handleWorker(rec, req)
	return rec
}

func serveSecurityAsset(s *proxyServer, target string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.serveDist(rec, req)
	return rec
}

func TestWorkerFailsClosedWithoutAllowlist(t *testing.T) {
	s := newSecurityTestServer(nil)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("fail-open: request reached the upstream")
	}))
	defer upstream.Close()

	if rec := serveWorker(t, s, upstream.URL, "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (fail-closed)", rec.Code)
	}
}

func TestWorkerRejectsSSRFTargets(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("SSRF target was reached")
	}))
	defer upstream.Close()

	s := newSecurityTestServer(func() []string {
		return []string{"worker.internal:443"}
	})

	targets := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1:6379/",
		"http://localhost:2375/containers/json",
		"ws://127.0.0.1:6379/",
		"https://metadata.google.internal/computeMetadata/v1/",
		upstream.URL,
	}
	for _, target := range targets {
		if rec := serveWorker(t, s, target, "", nil); rec.Code != http.StatusForbidden {
			t.Errorf("target %q: status = %d, want 403", target, rec.Code)
		}
	}
}

func TestWorkerRejectsUnsupportedSchemeAndUserinfo(t *testing.T) {
	s := newSecurityTestServer(func() []string {
		return []string{"worker.internal:443"}
	})
	for _, target := range []string{
		"file:///etc/passwd",
		"gopher://worker.internal:443/",
		"http://user:pass@worker.internal:443/",
	} {
		if rec := serveWorker(t, s, target, "", nil); rec.Code != http.StatusBadRequest {
			t.Errorf("target %q: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestWorkerAllowsInjectedHost(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "worker-ok")
	}))
	defer upstream.Close()

	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	s := newSecurityTestServer(func() []string { return []string{u.Host} })

	rec := serveWorker(t, s, upstream.URL, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "worker-ok" {
		t.Fatalf("body = %q, want worker-ok", rec.Body.String())
	}
}

func TestWorkerAllowlistPinsThePort(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("wrong-port target was reached")
	}))
	defer upstream.Close()

	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	_, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	other := "1"
	if port == other {
		other = "2"
	}
	s := newSecurityTestServer(func() []string { return []string{"127.0.0.1:" + other} })

	if rec := serveWorker(t, s, upstream.URL, "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for unpinned port", rec.Code)
	}
}

func TestWorkerRejectsCrossOriginUpgrade(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "worker-ok")
	}))
	defer upstream.Close()

	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	s := newSecurityTestServer(func() []string { return []string{u.Host} })
	const loopbackHost = "127.0.0.1:7777"

	cases := []struct {
		origin string
		want   int
	}{
		{"http://evil.example", http.StatusForbidden},
		{"https://127.0.0.1:7777", http.StatusForbidden},
		{"http://127.0.0.1:9999", http.StatusForbidden},
		{"not a url", http.StatusForbidden},
		{"http://127.0.0.1:7777", http.StatusOK},
	}
	for _, tc := range cases {
		rec := serveWorker(t, s, upstream.URL, loopbackHost, map[string]string{"Origin": tc.origin})
		if rec.Code != tc.want {
			t.Errorf("Origin %q: status = %d, want %d", tc.origin, rec.Code, tc.want)
		}
	}
}

func TestWorkerStripsSetCookie(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "attacker", Path: "/"})
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	s := newSecurityTestServer(func() []string { return []string{u.Host} })

	rec := serveWorker(t, s, upstream.URL, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("worker response leaked Set-Cookie: %v", got)
	}
}

func TestWorkerPreservesTargetQueryAndDropsInboundTarget(t *testing.T) {
	var gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	s := newSecurityTestServer(func() []string { return []string{u.Host} })

	rec := serveWorker(t, s, upstream.URL+"/ws?token=abc&room=1", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("worker query %q: %v", gotQuery, err)
	}
	if got := q.Get("token"); got != "abc" {
		t.Fatalf("token = %q, want abc (worker query was dropped)", got)
	}
	if got := q.Get("room"); got != "1" {
		t.Fatalf("room = %q, want 1", got)
	}
	if q.Has("target") {
		t.Fatalf("inbound target parameter was forwarded: %q", gotQuery)
	}
}

func TestWorkerKeepsOtherInboundQueryParams(t *testing.T) {
	var gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	s := newSecurityTestServer(func() []string { return []string{u.Host} })

	req := httptest.NewRequest(http.MethodGet,
		"/wsworker?target="+url.QueryEscape(upstream.URL+"/ws")+"&trace=1", nil)
	rec := httptest.NewRecorder()
	s.handleWorker(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("worker query %q: %v", gotQuery, err)
	}
	if q.Get("trace") != "1" {
		t.Fatalf("inbound trace param was dropped: %q", gotQuery)
	}
	if q.Has("target") {
		t.Fatalf("inbound target parameter was forwarded: %q", gotQuery)
	}
}

func TestWorkerProxyCacheIsBounded(t *testing.T) {
	s := newSecurityTestServer(func() []string { return []string{"worker.internal:443"} })

	for i := 0; i < maxWorkerProxies*2; i++ {
		s.workerFor(&url.URL{
			Scheme: "http",
			Host:   "worker.internal:443",
			Path:   "/ws/" + strconv.Itoa(i),
		})
	}
	if len(s.workerProxy) > maxWorkerProxies {
		t.Fatalf("workerProxy grew to %d, want <= %d", len(s.workerProxy), maxWorkerProxies)
	}
	if len(s.workerOrder) != len(s.workerProxy) {
		t.Fatalf("workerOrder (%d) out of sync with cache (%d)", len(s.workerOrder), len(s.workerProxy))
	}
}

func TestWorkerProxyCacheReusesEntry(t *testing.T) {
	s := newSecurityTestServer(func() []string { return []string{"worker.internal:443"} })
	u := &url.URL{Scheme: "http", Host: "worker.internal:443", Path: "/ws"}

	first := s.workerFor(u)
	second := s.workerFor(u)
	if first != second {
		t.Fatal("worker proxy was rebuilt for the same target")
	}
	if len(s.workerProxy) != 1 || len(s.workerOrder) != 1 {
		t.Fatalf("cache = %d entries, order = %d, want 1/1", len(s.workerProxy), len(s.workerOrder))
	}
}

func TestGzipETagIsWellFormedAndRevalidates(t *testing.T) {
	s := newSecurityTestServer(nil)
	rec := serveSecurityAsset(s, "/assets/index.abc123.css", map[string]string{"Accept-Encoding": "gzip"})
	etag := rec.Header().Get("ETag")
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Fatalf("gzip ETag %q is not a quoted string", etag)
	}
	if !strings.HasSuffix(etag, `-gz"`) {
		t.Fatalf("gzip ETag %q does not carry the -gz suffix", etag)
	}

	revalidated := serveSecurityAsset(s, "/assets/index.abc123.css", map[string]string{
		"Accept-Encoding": "gzip",
		"If-None-Match":   etag,
	})
	if revalidated.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", revalidated.Code)
	}
	if revalidated.Body.Len() != 0 {
		t.Fatalf("304 carried %d bytes", revalidated.Body.Len())
	}
}

func TestGzipAndIdentityETagsDiffer(t *testing.T) {
	s := newSecurityTestServer(nil)
	gz := serveSecurityAsset(s, "/assets/index.abc123.css", map[string]string{"Accept-Encoding": "gzip"})
	plain := serveSecurityAsset(s, "/assets/index.abc123.css", nil)

	if gz.Header().Get("ETag") == plain.Header().Get("ETag") {
		t.Fatal("gzip and identity representations share an ETag")
	}
}

func TestVaryIsSetForBothEncodings(t *testing.T) {
	s := newSecurityTestServer(nil)
	for _, header := range []map[string]string{nil, {"Accept-Encoding": "gzip"}} {
		rec := serveSecurityAsset(s, "/assets/index.abc123.css", header)
		if got := rec.Header().Get("Vary"); got != "Accept-Encoding" {
			t.Fatalf("Vary = %q, want Accept-Encoding", got)
		}
	}
}

func TestGzipContentTypeComesFromThePath(t *testing.T) {
	s := newSecurityTestServer(nil)
	rec := serveSecurityAsset(s, "/assets/index.abc123.css", map[string]string{"Accept-Encoding": "gzip"})
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Fatalf("Content-Type = %q, want text/css", got)
	}
}

func TestGzipSourceMapKeepsJSONContentType(t *testing.T) {
	s := newSecurityTestServer(nil)
	rec := serveSecurityAsset(s, "/chunks/app.abc123.js.map", map[string]string{"Accept-Encoding": "gzip"})
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

func TestDirectMainOverridesClientForwardedHeaders(t *testing.T) {
	var host, proto string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host = r.Header.Get("X-Forwarded-Host")
		proto = r.Header.Get("X-Forwarded-Proto")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	s := newSecurityTestServer(nil)
	s.setProxyTarget(upstream.URL)
	s.mainProxy = newSecurityMainProxy(s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Host = "127.0.0.1:7777"
	req.Header.Set("X-Forwarded-Host", "evil.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	s.handleProxied(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if host != "127.0.0.1:7777" {
		t.Fatalf("X-Forwarded-Host = %q, want the loopback host", host)
	}
	if proto != "http" {
		t.Fatalf("X-Forwarded-Proto = %q, want http", proto)
	}
}

func TestWSEndpointResponseAllowlistsWorkerHost(t *testing.T) {
	const workerURL = "wss://worker.example:7443/ws"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{"url": workerURL},
		})
	}))
	defer upstream.Close()

	s := newSecurityTestServer(nil)
	s.workerHosts = &workerHostAllowlist{}
	s.allowedWorkerHosts = func() []string { return s.workerHosts.list() }
	s.setProxyTarget(upstream.URL)
	s.mainProxy = newSecurityMainProxy(s)

	rec := httptest.NewRecorder()
	s.handleProxied(rec, httptest.NewRequest(http.MethodGet, "/api/v1/signal/ws-endpoint", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (the proxy consumed the body)", err)
	}
	if body.Data.URL != workerURL {
		t.Fatalf("url = %q, page lost the ws-endpoint body", body.Data.URL)
	}

	if hosts := s.workerHosts.list(); len(hosts) != 1 || hosts[0] != "worker.example:7443" {
		t.Fatalf("allowlist = %v, want [worker.example:7443]", hosts)
	}
	if !s.workerHostAllowed("https", "worker.example:7443") {
		t.Fatal("advertised worker host was not allowed")
	}
	if s.workerHostAllowed("https", "worker.example:9999") {
		t.Fatal("allowlist leaked to another port")
	}
}

func TestTrustedWorkerHostsIncludesConfiguredBaseURLHost(t *testing.T) {
	s := newSecurityTestServer(nil)
	s.workerHosts = &workerHostAllowlist{}
	s.allowedWorkerHosts = s.trustedWorkerHosts
	s.setProxyTarget("https://backend.example:8443/app")

	hosts := s.trustedWorkerHosts()
	if len(hosts) != 1 || hosts[0] != "backend.example:8443" {
		t.Fatalf("allowlist = %v, want [backend.example:8443]", hosts)
	}
	if !s.workerHostAllowed("https", "backend.example:8443") {
		t.Fatal("configured base URL host was not allowed")
	}
	if s.workerHostAllowed("https", "backend.example") {
		t.Fatal("default port was allowed instead of the configured one")
	}

	s.workerHosts.allow("worker.example:7443")
	hosts = s.trustedWorkerHosts()
	if len(hosts) != 2 {
		t.Fatalf("allowlist = %v, want backend host plus advertised worker host", hosts)
	}
}

// TestProductionWiringIsFailClosed reproduces the exact allowlist wiring
// startProxyServer installs, with no backend configured, and checks that the
// review's SSRF payloads are all rejected.
func TestProductionWiringIsFailClosed(t *testing.T) {
	s := newSecurityTestServer(nil)
	s.workerHosts = &workerHostAllowlist{}
	s.allowedWorkerHosts = s.trustedWorkerHosts

	targets := []string{
		"http://169.254.169.254/latest/meta-data/iam/security-credentials/",
		"http://127.0.0.1:6379/",
		"http://localhost:2375/containers/json",
		"ws://127.0.0.1:6379/",
		"http://[::1]:2375/",
	}
	for _, target := range targets {
		if rec := serveWorker(t, s, target, "", nil); rec.Code != http.StatusForbidden {
			t.Errorf("target %q: status = %d, want 403", target, rec.Code)
		}
	}
}

func TestWorkerRejectsEverythingWhenTargetUnsetAndNoWorkerHost(t *testing.T) {
	s := newSecurityTestServer(nil)
	s.workerHosts = &workerHostAllowlist{}
	s.allowedWorkerHosts = s.trustedWorkerHosts
	s.setProxyTarget("")

	if hosts := s.trustedWorkerHosts(); len(hosts) != 0 {
		t.Fatalf("allowlist = %v, want empty", hosts)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request reached upstream with no trusted host")
	}))
	defer upstream.Close()
	if rec := serveWorker(t, s, upstream.URL, "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestSetProxyTargetResetsWorkerAllowlist(t *testing.T) {
	s := newSecurityTestServer(nil)
	s.workerHosts = &workerHostAllowlist{}
	s.workerHosts.allow("worker.example:7443")
	s.setProxyTarget("http://backend.example:8080")
	if got := s.workerHosts.list(); len(got) != 0 {
		t.Fatalf("allowlist survived a backend switch: %v", got)
	}
}

func TestNormalizedHostPort(t *testing.T) {
	cases := []struct {
		scheme, hostport, want string
		ok                     bool
	}{
		{"http", "example.com", "example.com:80", true},
		{"https", "example.com", "example.com:443", true},
		{"ws", "example.com", "example.com:80", true},
		{"wss", "example.com", "example.com:443", true},
		{"http", "Example.COM:8080", "example.com:8080", true},
		{"http", "[::1]:9000", "[::1]:9000", true},
		{"http", "[::1]", "[::1]:80", true},
		{"http", "", "", false},
		{"ftp", "example.com", "", false},
		{"http", "[::1", "", false},
	}
	for _, tc := range cases {
		got, ok := normalizedHostPort(tc.scheme, tc.hostport)
		if ok != tc.ok || got != tc.want {
			t.Errorf("normalizedHostPort(%q, %q) = %q,%v want %q,%v",
				tc.scheme, tc.hostport, got, ok, tc.want, tc.ok)
		}
	}
}
