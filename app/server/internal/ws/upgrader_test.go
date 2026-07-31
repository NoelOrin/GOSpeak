package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractToken_Header(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "Bearer test-token-123")

	token, fromSubprotocol := extractToken(r)
	if token != "test-token-123" || fromSubprotocol {
		t.Fatalf("expected header token, got token=%q fromSubprotocol=%v", token, fromSubprotocol)
	}
}

func TestExtractToken_Cookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: "cookie-token-456"})

	token, fromSubprotocol := extractToken(r)
	if token != "cookie-token-456" || fromSubprotocol {
		t.Fatalf("expected cookie token, got token=%q fromSubprotocol=%v", token, fromSubprotocol)
	}
}

func TestExtractToken_QueryRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws?token=query-token-789", nil)

	token, _ := extractToken(r)
	if token != "" {
		t.Fatalf("query token must not be accepted, got %q", token)
	}
}

func TestExtractToken_Subprotocol(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "gospeak, ws-ticket-123")

	token, fromSubprotocol := extractToken(r)
	if token != "ws-ticket-123" || !fromSubprotocol {
		t.Fatalf("expected subprotocol ticket, got token=%q fromSubprotocol=%v", token, fromSubprotocol)
	}
}

func TestExtractToken_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)

	token, _ := extractToken(r)
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
}

func TestExtractToken_PlainHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "raw-token-no-bearer")

	token, _ := extractToken(r)
	if token != "raw-token-no-bearer" {
		t.Fatalf("expected 'raw-token-no-bearer', got %q", token)
	}
}

func TestExtractToken_BearerPriority(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "Bearer header-token")
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: "cookie-token"})

	token, _ := extractToken(r)
	if token != "header-token" {
		t.Fatalf("expected 'header-token' from Authorization, got %q", token)
	}
}
