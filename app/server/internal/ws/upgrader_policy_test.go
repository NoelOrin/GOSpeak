package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/pkg"
)

// /ws 由根 http.Handler 直连 Upgrader，握手阶段必须补回 Gin 的 BanCheck 等价判决。
func TestUpgrader_ServeHTTP_ConnectPolicyRejectsBannedUser(t *testing.T) {
	token, err := pkg.GenerateToken("banned-user", "Banned", "banned-uuid", "ban", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	called := false
	u := NewUpgrader(UpgraderConfig{
		AuthCookieName: accessCookieName,
		ConnectPolicy: func(claims *pkg.Claims) (pkg.ErrCode, string) {
			called = true
			if claims.Role != "ban" {
				t.Errorf("policy received unexpected role %q", claims.Role)
			}
			return pkg.USER_BANNED, "user has been banned"
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: token})
	rec := httptest.NewRecorder()
	u.ServeHTTP(rec, req)

	if !called {
		t.Fatal("ConnectPolicy must run after VerifyToken and before Accept")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("banned handshake must be 403, got %d", rec.Code)
	}
}

func TestPolicyHTTPStatus(t *testing.T) {
	if got := policyHTTPStatus(pkg.USER_BANNED); got != http.StatusForbidden {
		t.Errorf("USER_BANNED -> %d, want 403", got)
	}
	if got := policyHTTPStatus(pkg.FORBIDDEN); got != http.StatusForbidden {
		t.Errorf("FORBIDDEN -> %d, want 403", got)
	}
	if got := policyHTTPStatus(pkg.INTERNAL_ERROR); got != http.StatusInternalServerError {
		t.Errorf("INTERNAL_ERROR -> %d, want 500", got)
	}
}
