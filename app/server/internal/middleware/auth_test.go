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
