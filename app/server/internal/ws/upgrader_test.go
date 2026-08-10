package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/pkg"
)

func TestAccessTokenFromCookie(t *testing.T) {
	upgrader := NewUpgrader(UpgraderConfig{})
	r := httptest.NewRequest("GET", "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: "access-token-123"})

	if got := upgrader.accessTokenFromCookie(r); got != "access-token-123" {
		t.Fatalf("expected cookie access token, got %q", got)
	}
}

func TestAccessTokenFromCookie_CustomName(t *testing.T) {
	upgrader := NewUpgrader(UpgraderConfig{AuthCookieName: "gospeak_auth"})
	r := httptest.NewRequest("GET", "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_auth", Value: "access-token-456"})

	if got := upgrader.accessTokenFromCookie(r); got != "access-token-456" {
		t.Fatalf("expected custom cookie access token, got %q", got)
	}
}

func TestAccessTokenFromCookie_Empty(t *testing.T) {
	upgrader := NewUpgrader(UpgraderConfig{})
	r := httptest.NewRequest("GET", "/ws", nil)

	if got := upgrader.accessTokenFromCookie(r); got != "" {
		t.Fatalf("expected empty token, got %q", got)
	}
}

func TestAccessTokenFromCookie_RefreshCookieIgnored(t *testing.T) {
	upgrader := NewUpgrader(UpgraderConfig{})
	r := httptest.NewRequest("GET", "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_refresh_token", Value: "refresh-token"})

	if got := upgrader.accessTokenFromCookie(r); got != "" {
		t.Fatalf("refresh cookie must not be used for WS auth, got %q", got)
	}
}

func TestUpgrader_ServeHTTP_RejectsMissingCookie(t *testing.T) {
	upgrader := NewUpgrader(UpgraderConfig{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)

	upgrader.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without cookie, got %d", w.Code)
	}
}

func TestUpgrader_ServeHTTP_RejectsHeaderAccessToken(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	upgrader := NewUpgrader(UpgraderConfig{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Authorization", "Bearer "+access)

	upgrader.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for header access token, got %d", w.Code)
	}
}

func TestUpgrader_ServeHTTP_RejectsSubprotocolAccessToken(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	upgrader := NewUpgrader(UpgraderConfig{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "gospeak, "+access)

	upgrader.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for subprotocol access token, got %d", w.Code)
	}
}

func TestUpgrader_ServeHTTP_RejectsRefreshTokenCookie(t *testing.T) {
	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	upgrader := NewUpgrader(UpgraderConfig{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: refresh})

	upgrader.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for refresh token cookie, got %d", w.Code)
	}
}

func TestUpgrader_ServeHTTP_RejectsBotTokenCookie(t *testing.T) {
	bot, err := pkg.GenerateBotToken("bot", "Bot", "uuid-bot", "bot", 1, []string{"signal:kick"}, false)
	if err != nil {
		t.Fatalf("GenerateBotToken: %v", err)
	}

	upgrader := NewUpgrader(UpgraderConfig{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: bot})

	upgrader.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bot token cookie, got %d", w.Code)
	}
}
