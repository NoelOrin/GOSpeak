package livekitproxy

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseTargetNormalizesScheme(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"ws://livekit:7880", "http://livekit:7880", false},
		{"wss://lk.example.com", "https://lk.example.com", false},
		{"http://livekit:7880/", "http://livekit:7880", false},
		{"", "", true},
		{"livekit:7880", "", true},
		{"ftp://livekit:7880", "", true},
	}
	for _, tc := range cases {
		got, err := parseTarget(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("parseTarget(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseTarget(%q) error: %v", tc.in, err)
		}
		if got.String() != tc.want {
			t.Fatalf("parseTarget(%q) = %q, want %q", tc.in, got.String(), tc.want)
		}
	}
}

func TestServerURLForRequest(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		proto   string
		fwdHost string
		tls     bool
		want    string
	}{
		{"forwarded https", "voice.example.com", "https", "", false, "wss://voice.example.com/livekit"},
		{"forwarded http", "10.0.0.5:8998", "http", "", false, "ws://10.0.0.5:8998/livekit"},
		{"forwarded https list", "voice.example.com", "https, http", "", false, "wss://voice.example.com/livekit"},
		{"forwarded host wins", "gospeak:8998", "https", "voice.example.com", false, "wss://voice.example.com/livekit"},
		{"tls fallback", "voice.example.com", "", "", true, "wss://voice.example.com/livekit"},
		{"plain fallback", "localhost:8998", "", "", false, "ws://localhost:8998/livekit"},
		{"no host", "", "https", "", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/api/v1/signal/token", nil)
			r.Host = tc.host
			if tc.proto != "" {
				r.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if tc.fwdHost != "" {
				r.Header.Set("X-Forwarded-Host", tc.fwdHost)
			}
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if got := ServerURLForRequest(r); got != tc.want {
				t.Fatalf("ServerURLForRequest() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHandlerStripsPrefixAndRewritesHost(t *testing.T) {
	var seenPath, seenHost, seenQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath, seenHost, seenQuery = r.URL.Path, r.Host, r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	h := NewHandler(func() string { return backend.URL })
	req := httptest.NewRequest(http.MethodGet, "/livekit/rtc/validate?access_token=abc", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	if seenPath != "/rtc/validate" {
		t.Fatalf("backend path = %q, want /rtc/validate", seenPath)
	}
	if seenQuery != "access_token=abc" {
		t.Fatalf("backend query = %q, want access_token=abc", seenQuery)
	}
	if !strings.HasPrefix(seenHost, "127.0.0.1:") {
		t.Fatalf("backend host = %q, want backend host", seenHost)
	}
}

func TestHandlerUnconfiguredReturns502(t *testing.T) {
	h := NewHandler(func() string { return "" })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/livekit/rtc/validate", nil))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
	if !strings.Contains(w.Body.String(), "livekit not configured") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

// 同源反代只暴露浏览器信令需要的 /rtc/*，
// LiveKit 的管理面（twirp / debug / whip）不能被顺带透传到公网。
func TestHandlerRejectsNonRtcPaths(t *testing.T) {
	called := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer backend.Close()

	h := NewHandler(func() string { return backend.URL })
	for _, path := range []string{
		"/livekit/",
		"/livekit",
		"/livekit/debug/pprof/",
		"/livekit/twirp/livekit.RoomService/ListRooms",
		"/livekit/whip/v1",
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, w.Code)
		}
	}
	if called {
		t.Fatal("backend must not be reached for non-/rtc paths")
	}
}

func TestAllowedPath(t *testing.T) {
	cases := map[string]bool{
		"/livekit/rtc/v1":       true,
		"/livekit/rtc/validate": true,
		"/livekit/rtc/":         true,
		"/livekit/":             false,
		"/livekit":              false,
		"/livekit/rtc":          false,
		"/livekit/debug/pprof":  false,
		"/api/v1/signal/token":  false,
	}
	for path, want := range cases {
		if got := allowedPath(path); got != want {
			t.Fatalf("allowedPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestCachedResolverReusesValueWithinTTL(t *testing.T) {
	calls := 0
	resolve := Cached(func() string {
		calls++
		return "ws://livekit:7880"
	}, time.Minute)

	for i := 0; i < 5; i++ {
		if got := resolve(); got != "ws://livekit:7880" {
			t.Fatalf("resolve() = %q", got)
		}
	}
	if calls != 1 {
		t.Fatalf("resolve called %d times, want 1", calls)
	}
}
