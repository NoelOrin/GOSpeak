package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireAgentFence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	calls := 0
	r.Use(RequireAgentFence(func() error {
		calls++
		return errors.New("lost")
	}))
	r.POST("/write", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.GET("/read", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/write", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("write without fence got %d, want 403", rec.Code)
	}
	if calls != 1 {
		t.Fatalf("fence calls = %d, want 1", calls)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/read", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("read should skip fence, got %d", rec.Code)
	}
	if calls != 1 {
		t.Fatalf("read must not call fence, calls = %d", calls)
	}
}
