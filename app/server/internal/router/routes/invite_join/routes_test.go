package invite_join

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRoutes_Registered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	pub := r.Group("/api/v1/invite")
	RegisterPublic(pub, nil)
	prot := r.Group("/api/v1/invite")
	RegisterProtected(prot, nil)
	admin := r.Group("/api/v1/domain")
	RegisterDomainAdmin(admin, nil)

	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/invite/register"},
		{http.MethodPost, "/api/v1/invite/join"},
		{http.MethodPost, "/api/v1/invite/my-request"},
		{http.MethodPost, "/api/v1/invite/cancel-request"},
		{http.MethodPost, "/api/v1/domain/join-requests/list"},
		{http.MethodPost, "/api/v1/domain/join-requests/review"},
		{http.MethodPost, "/api/v1/domain/join-approval/config"},
	}
	registered := map[string]bool{}
	for _, route := range r.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, want := range cases {
		if !registered[want.method+" "+want.path] {
			t.Fatalf("route %s %s is not registered", want.method, want.path)
		}
	}
}

func TestRoutes_RegisterIsRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterPublic(r.Group("/api/v1/invite"), nil)
	// 20 次以内放行；第 21 次命中 1017。handler 为 nil 会在放行后 panic，
	// 因此只断言「限流生效时返回 1017 而不是 500」。
	for i := 0; i < 21; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/invite/register", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.1.2.3:1234"
		func() {
			defer func() { _ = recover() }()
			r.ServeHTTP(rec, req)
		}()
		if i == 20 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("21st request status = %d, want 429", rec.Code)
		}
	}
}
