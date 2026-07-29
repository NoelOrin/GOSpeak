package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractToken_Header(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "Bearer test-token-123")

	token := extractToken(r)
	if token != "test-token-123" {
		t.Fatalf("expected 'test-token-123', got %q", token)
	}
}

func TestExtractToken_Cookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: "cookie-token-456"})

	token := extractToken(r)
	if token != "cookie-token-456" {
		t.Fatalf("expected 'cookie-token-456', got %q", token)
	}
}

func TestExtractToken_Query(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws?token=query-token-789", nil)

	token := extractToken(r)
	if token != "query-token-789" {
		t.Fatalf("expected 'query-token-789', got %q", token)
	}
}

func TestExtractToken_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)

	token := extractToken(r)
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
}

// TestExtractToken_PlainHeader verifies that a raw Authorization header
// without Bearer prefix is returned as-is (intentional: supports custom auth schemes).
func TestExtractToken_PlainHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "raw-token-no-bearer")

	token := extractToken(r)
	if token != "raw-token-no-bearer" {
		t.Fatalf("expected 'raw-token-no-bearer', got %q", token)
	}
}

func TestExtractToken_BearerPriority(t *testing.T) {
	// Authorization header should take priority over cookie
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "Bearer header-token")
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: "cookie-token"})

	token := extractToken(r)
	if token != "header-token" {
		t.Fatalf("expected 'header-token' from Authorization, got %q", token)
	}
}
