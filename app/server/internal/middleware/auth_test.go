package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

type testPermissionChecker struct {
	allowed bool
}

func (c testPermissionChecker) HasPermission(roleName, permCode string) bool {
	return c.allowed
}

func TestRequireOwnerOrPermissionRejectsNonStringOwnerWithoutPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetPermissionChecker(testPermissionChecker{allowed: false})
	t.Cleanup(func() { SetPermissionChecker(nil) })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("username", "alice")
		c.Set("owner", 123)
	})
	r.GET("/resource", RequireOwnerOrPermission("owner", "resource:update"), func(c *gin.Context) {
		pkg.Success(c, nil)
	})

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	r.ServeHTTP(w, req)

	var resp pkg.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Code != pkg.FORBIDDEN {
		t.Fatalf("expected FORBIDDEN, got %d", resp.Code)
	}
}

func TestRequireOwnerOrPermissionAllowsMatchingOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetPermissionChecker(testPermissionChecker{allowed: false})
	t.Cleanup(func() { SetPermissionChecker(nil) })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("username", "alice")
		c.Set("owner", "alice")
	})
	r.GET("/resource", RequireOwnerOrPermission("owner", "resource:update"), func(c *gin.Context) {
		pkg.Success(c, nil)
	})

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	r.ServeHTTP(w, req)

	var resp pkg.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected SUCCESS, got %d", resp.Code)
	}
}

func TestPermissionGranted_PrefersClaimsPermissions(t *testing.T) {
	checker := testPermissionChecker{allowed: false}
	claims := &pkg.Claims{Role: "user", Permissions: []string{"signal:kick"}}
	if !PermissionGranted(claims, claims.Role, "signal:kick", checker) {
		t.Fatal("bot/token permissions should grant signal:kick")
	}
	if PermissionGranted(claims, claims.Role, "mute:manage", checker) {
		t.Fatal("missing claim permission should deny")
	}
}

func TestPermissionGranted_FallsBackToRoleChecker(t *testing.T) {
	checker := testPermissionChecker{allowed: true}
	if !PermissionGranted(nil, "admin", "signal:kick", checker) {
		t.Fatal("role checker fallback should grant")
	}
	deny := testPermissionChecker{allowed: false}
	if PermissionGranted(&pkg.Claims{Role: "user"}, "user", "signal:kick", deny) {
		t.Fatal("role without permission should deny")
	}
}

func TestVerifyToken_AcceptsAccessAndBot(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, code := VerifyToken(access); code != pkg.SUCCESS {
		t.Fatalf("expected access token accepted, got code=%d", code)
	}

	bot, err := pkg.GenerateBotToken("bot", "Bot", "uuid-bot", "bot", 1, []string{"signal:kick"}, false)
	if err != nil {
		t.Fatalf("GenerateBotToken: %v", err)
	}
	if _, code := VerifyToken(bot); code != pkg.SUCCESS {
		t.Fatalf("expected bot token accepted, got code=%d", code)
	}
}

func TestVerifyToken_RejectsMissingUserUUID(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, code := VerifyToken(access); code != pkg.TOKEN_WRONG {
		t.Fatalf("expected access token rejected without user_uuid, got code=%d", code)
	}

	bot, err := pkg.GenerateBotToken("bot", "Bot", "", "bot", 1, []string{"signal:kick"}, false)
	if err != nil {
		t.Fatalf("GenerateBotToken: %v", err)
	}
	if _, code := VerifyToken(bot); code != pkg.TOKEN_WRONG {
		t.Fatalf("expected bot token rejected without user_uuid, got code=%d", code)
	}

	ticket, err := pkg.GenerateWSTicket("alice", "Alice", "", "user", 1)
	if err != nil {
		t.Fatalf("GenerateWSTicket: %v", err)
	}
	if _, code := VerifyWSTicket(ticket); code != pkg.TOKEN_WRONG {
		t.Fatalf("expected ws ticket rejected without user_uuid, got code=%d", code)
	}
}

func TestVerifyToken_RejectsRefreshAndWSTicket(t *testing.T) {
	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	ticket, err := pkg.GenerateWSTicket("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateWSTicket: %v", err)
	}

	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "refresh", token: refresh},
		{name: "ws-ticket", token: ticket},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims, code := VerifyToken(tc.token)
			if code != pkg.TOKEN_WRONG {
				t.Fatalf("expected TOKEN_WRONG, got code=%d", code)
			}
			if claims != nil {
				t.Fatalf("expected nil claims, got %#v", claims)
			}
		})
	}
}

func TestVerifyWSTicket_AcceptsWSTicket(t *testing.T) {
	ticket, err := pkg.GenerateWSTicket("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateWSTicket: %v", err)
	}
	claims, code := VerifyWSTicket(ticket)
	if code != pkg.SUCCESS {
		t.Fatalf("expected ws ticket accepted, got code=%d", code)
	}
	if claims == nil || claims.Username != "alice" {
		t.Fatalf("expected alice claims, got %#v", claims)
	}
}

func TestVerifyWSTicket_RejectsNonWSTicket(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	bot, err := pkg.GenerateBotToken("bot", "Bot", "uuid-bot", "bot", 1, []string{"signal:kick"}, false)
	if err != nil {
		t.Fatalf("GenerateBotToken: %v", err)
	}
	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "access", token: access},
		{name: "bot", token: bot},
		{name: "refresh", token: refresh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims, code := VerifyWSTicket(tc.token)
			if code != pkg.TOKEN_WRONG {
				t.Fatalf("expected TOKEN_WRONG, got code=%d", code)
			}
			if claims != nil {
				t.Fatalf("expected nil claims, got %#v", claims)
			}
		})
	}
}

func TestCORS_AddsVary(t *testing.T) {
	router := gin.New()
	router.Use(CORS([]string{"https://app.example.com"}))
	router.GET("/x", func(c *gin.Context) { c.String(200, "ok") })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://app.example.com")
	router.ServeHTTP(rec, req)
	if rec.Header().Get("Vary") != "Origin" {
		t.Fatal("expected Vary: Origin")
	}
}
