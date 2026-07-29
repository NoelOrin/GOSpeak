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

func TestRequireGuildMember_NoUUID(t *testing.T) {
	r := setupGuildMiddlewareRouter(func(uuid, userUUID string) bool { return true })

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

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"guild_uuid":"guild-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireGuildMember_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetGuildChecker(func(uuid, userUUID string) bool { return true })

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

func init() {
	SetGuildChecker(nil)
}
