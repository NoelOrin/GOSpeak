package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/pkg"
)

func TestExtractToken_Subprotocol(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "gospeak, ws-ticket-123")

	token, fromSubprotocol := extractToken(r)
	if token != "ws-ticket-123" || !fromSubprotocol {
		t.Fatalf("expected subprotocol ticket, got token=%q fromSubprotocol=%v", token, fromSubprotocol)
	}
}

func TestExtractToken_HeaderRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "Bearer test-token-123")

	token, fromSubprotocol := extractToken(r)
	if token != "" || fromSubprotocol {
		t.Fatalf("expected header token rejected, got token=%q fromSubprotocol=%v", token, fromSubprotocol)
	}
}

func TestExtractToken_CookieRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: "cookie-token-456"})

	token, fromSubprotocol := extractToken(r)
	if token != "" || fromSubprotocol {
		t.Fatalf("expected cookie token rejected, got token=%q fromSubprotocol=%v", token, fromSubprotocol)
	}
}

func TestExtractToken_QueryRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws?token=query-token-789", nil)

	token, _ := extractToken(r)
	if token != "" {
		t.Fatalf("query token must not be accepted, got %q", token)
	}
}

func TestExtractToken_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)

	token, _ := extractToken(r)
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
}

func TestExtractToken_PlainHeaderRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "raw-token-no-bearer")

	token, _ := extractToken(r)
	if token != "" {
		t.Fatalf("expected plain header token rejected, got %q", token)
	}
}

func TestExtractToken_BearerAndCookieRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "Bearer header-token")
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: "cookie-token"})

	token, _ := extractToken(r)
	if token != "" {
		t.Fatalf("expected header/cookie tokens rejected, got %q", token)
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

func TestUpgrader_ServeHTTP_RejectsCookieAccessToken(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	upgrader := NewUpgrader(UpgraderConfig{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: access})

	upgrader.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for cookie access token, got %d", w.Code)
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
