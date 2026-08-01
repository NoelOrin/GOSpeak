package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupDomainMiddlewareRouter(checker func(uuid, userUUID string) bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetDomainChecker(checker)

	r.Use(func(c *gin.Context) {
		userUUID := c.GetHeader("X-User-UUID")
		if userUUID != "" {
			c.Set("user_uuid", userUUID)
		}
		c.Next()
	})

	r.POST("/test", RequireDomainMember(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}

func setupDomainMiddlewareIfProvidedRouter(checker func(uuid, userUUID string) bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetDomainChecker(checker)

	r.Use(func(c *gin.Context) {
		userUUID := c.GetHeader("X-User-UUID")
		if userUUID != "" {
			c.Set("user_uuid", userUUID)
		}
		c.Next()
	})

	r.POST("/test", RequireDomainMemberIfProvided(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}

func TestRequireDomainMemberIfProvided_SkipsWithoutDomain(t *testing.T) {
	r := setupDomainMiddlewareIfProvidedRouter(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetDomainChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"lobby"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without domain_uuid, got %d", w.Code)
	}
}

func TestRequireDomainMemberIfProvided_RejectsNonMember(t *testing.T) {
	r := setupDomainMiddlewareIfProvidedRouter(func(uuid, userUUID string) bool { return false })
	t.Cleanup(func() { SetDomainChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"domain_uuid":"domain-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-member, got %d", w.Code)
	}
}

func TestRequireDomainMemberIfProvided_AllowsMember(t *testing.T) {
	r := setupDomainMiddlewareIfProvidedRouter(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetDomainChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"domain_uuid":"domain-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for member, got %d", w.Code)
	}
}

func TestRequireDomainMember_NoUUID(t *testing.T) {
	r := setupDomainMiddlewareRouter(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetDomainChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (no domain_uuid), got %d", w.Code)
	}
}

func TestRequireDomainMember_NotMember(t *testing.T) {
	r := setupDomainMiddlewareRouter(func(uuid, userUUID string) bool { return false })
	t.Cleanup(func() { SetDomainChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"domain_uuid":"domain-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 (not a member), got %d", w.Code)
	}
}

func TestRequireDomainMember_Success(t *testing.T) {
	r := setupDomainMiddlewareRouter(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetDomainChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"domain_uuid":"domain-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireDomainMember_UUIDBodyIgnored(t *testing.T) {
	r := setupDomainMiddlewareRouter(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetDomainChecker(nil) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"uuid":"domain-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (uuid body no longer read), got %d", w.Code)
	}
}

func TestRequireDomainMember_PreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetDomainChecker(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetDomainChecker(nil) })
	r.Use(func(c *gin.Context) {
		c.Set("user_uuid", "user-1")
		c.Next()
	})
	r.POST("/test", RequireDomainMember(), func(c *gin.Context) {
		var body struct {
			DomainUUID string `json:"domain_uuid"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.DomainUUID != "domain-1" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "body not readable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"domain_uuid":"domain-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected body preserved for handler, got %d", w.Code)
	}
}

func TestRequireDomainMember_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetDomainChecker(func(uuid, userUUID string) bool { return true })
	t.Cleanup(func() { SetDomainChecker(nil) })

	// No auth middleware — user_uuid not set in context
	r.POST("/test", RequireDomainMember(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"domain_uuid":"domain-1"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (no auth in ctx), got %d", w.Code)
	}
}

func TestIsDomainMember_EmptyDomainAllows(t *testing.T) {
	SetDomainChecker(func(domainUUID, userUUID string) bool { return false })
	t.Cleanup(func() { SetDomainChecker(nil) })

	if !IsDomainMember("", "user-1") {
		t.Fatal("platform rooms must not require domain membership")
	}
}

func TestIsDomainMember_NilCheckerRejects(t *testing.T) {
	SetDomainChecker(nil)
	t.Cleanup(func() { SetDomainChecker(nil) })

	if IsDomainMember("domain-a", "user-1") {
		t.Fatal("nil checker must fail closed for domain-scoped requests")
	}
}

func TestIsDomainMember_DelegatesToChecker(t *testing.T) {
	SetDomainChecker(func(domainUUID, userUUID string) bool {
		return domainUUID == "domain-a" && userUUID == "user-1"
	})
	t.Cleanup(func() { SetDomainChecker(nil) })

	if IsDomainMember("domain-a", "other") {
		t.Fatal("expected non-member to be rejected")
	}
	if !IsDomainMember("domain-a", "user-1") {
		t.Fatal("expected member to be allowed")
	}
}
