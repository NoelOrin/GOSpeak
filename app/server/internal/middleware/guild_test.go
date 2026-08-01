package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupGuildMiddlewareRouter(checker func(uuid, userUUID string) bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetGuildChecker(checker)

	r.Use(func(c *gin.Context) {
		userUUID := c.GetHeader("X-User-UUID")
		if userUUID != "" {
			c.Set("user_uuid", userUUID)
		}
		c.Next()
	})

	r.POST("/test", RequireGuildMember(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}

func setupGuildMiddlewareIfProvidedRouter(checker func(uuid, userUUID string) bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetGuildChecker(checker)

	r.Use(func(c *gin.Context) {
		userUUID := c.GetHeader("X-User-UUID")
		if userUUID != "" {
			c.Set("user_uuid", userUUID)
		}
		c.Next()
	})

	r.POST("/test", RequireGuildMemberIfProvided(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}

func TestRequireGuildMemberIfProvided_SkipsWithoutGuild(t *testing.T) {
	r := setupGuildMiddlewareIfProvidedRouter(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetGuildChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"lobby"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without guild_uuid, got %d", w.Code)
	}
}

func TestRequireGuildMemberIfProvided_RejectsNonMember(t *testing.T) {
	r := setupGuildMiddlewareIfProvidedRouter(func(uuid, userUUID string) bool { return false })
	t.Cleanup(func() { SetGuildChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"guild_uuid":"guild-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-member, got %d", w.Code)
	}
}

func TestRequireGuildMemberIfProvided_AllowsMember(t *testing.T) {
	r := setupGuildMiddlewareIfProvidedRouter(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetGuildChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"guild_uuid":"guild-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for member, got %d", w.Code)
	}
}

func TestRequireGuildMember_NoUUID(t *testing.T) {
	r := setupGuildMiddlewareRouter(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetGuildChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (no guild_uuid), got %d", w.Code)
	}
}

func TestRequireGuildMember_NotMember(t *testing.T) {
	r := setupGuildMiddlewareRouter(func(uuid, userUUID string) bool { return false })
	t.Cleanup(func() { SetGuildChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"guild_uuid":"guild-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 (not a member), got %d", w.Code)
	}
}

func TestRequireGuildMember_Success(t *testing.T) {
	r := setupGuildMiddlewareRouter(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetGuildChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"guild_uuid":"guild-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireGuildMember_UUIDBodyFallback(t *testing.T) {
	r := setupGuildMiddlewareRouter(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetGuildChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"uuid":"guild-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with uuid fallback, got %d", w.Code)
	}
}

func TestRequireGuildMember_PreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetGuildChecker(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetGuildChecker(nil) })
	r.Use(func(c *gin.Context) {
		c.Set("user_uuid", "user-1")
		c.Next()
	})
	r.POST("/test", RequireGuildMember(), func(c *gin.Context) {
		var body struct {
			UUID string `json:"uuid"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.UUID != "guild-1" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "body not readable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"uuid":"guild-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected body preserved for handler, got %d", w.Code)
	}
}

func TestRequireGuildMember_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetGuildChecker(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetGuildChecker(nil) })

	// No auth middleware — user_uuid not set in context
	r.POST("/test", RequireGuildMember(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"guild_uuid":"guild-1"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (no auth in ctx), got %d", w.Code)
	}
}

func TestIsGuildMember_EmptyGuildAllows(t *testing.T) {
	SetGuildChecker(func(guildUUID, userUUID string) bool { return false })
	t.Cleanup(func() { SetGuildChecker(nil) })

	if !IsGuildMember("", "user-1") {
		t.Fatal("platform rooms must not require guild membership")
	}
}

func TestIsGuildMember_NilCheckerRejects(t *testing.T) {
	SetGuildChecker(nil)
	t.Cleanup(func() { SetGuildChecker(nil) })

	if IsGuildMember("guild-a", "user-1") {
		t.Fatal("nil checker must fail closed for guild-scoped requests")
	}
}

func TestIsGuildMember_DelegatesToChecker(t *testing.T) {
	SetGuildChecker(func(guildUUID, userUUID string) bool {
		return guildUUID == "guild-a" && userUUID == "user-1"
	})
	t.Cleanup(func() { SetGuildChecker(nil) })

	if IsGuildMember("guild-a", "other") {
		t.Fatal("expected non-member to be rejected")
	}
	if !IsGuildMember("guild-a", "user-1") {
		t.Fatal("expected member to be allowed")
	}
}
