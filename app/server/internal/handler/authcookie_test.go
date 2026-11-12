package handler

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/config"
)

// 反代终结 TLS 后转发到应用是明文 HTTP：auto 模式必须认 X-Forwarded-Proto，
// 否则 Cookie 缺少 Secure 标记，HTTPS 站点会被浏览器判为不安全连接。
func TestAuthCookieSecureHonorsForwardedProto(t *testing.T) {
	cfg := NewAuthCookieConfig(&config.Config{CookieSecure: "auto"})

	cases := []struct {
		name  string
		proto string
		tls   bool
		want  bool
	}{
		{"forwarded https", "https", false, true},
		{"forwarded https list", "https, http", false, true},
		{"forwarded http", "http", false, false},
		{"no header plain", "", false, false},
		{"direct tls", "", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://voice.example.com/api/v1/auth/login", nil)
			if tc.proto != "" {
				r.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if got := cfg.secure(r); got != tc.want {
				t.Fatalf("secure() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAuthCookieSecureExplicitOverride(t *testing.T) {
	always := NewAuthCookieConfig(&config.Config{CookieSecure: "true"})
	never := NewAuthCookieConfig(&config.Config{CookieSecure: "false"})
	r := httptest.NewRequest(http.MethodPost, "http://voice.example.com/", nil)

	if !always.secure(r) {
		t.Fatal("COOKIE_SECURE=true must always set Secure")
	}
	if never.secure(r) {
		t.Fatal("COOKIE_SECURE=false must never set Secure")
	}
}
