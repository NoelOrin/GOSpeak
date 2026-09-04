package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

func setupDomainPreviewRouter(t *testing.T, domainSvc *service.DomainService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewDomainHandler(domainSvc, nil)
	r.Use(func(c *gin.Context) {
		c.Set("user_uuid", c.GetHeader("X-User-UUID"))
		c.Next()
	})
	rg := r.Group("/api/v1/domain")
	rg.POST("/preview", h.Preview)
	rg.POST("/join", h.Join)
	return r
}

func postDomainPreview(t *testing.T, router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestDomainHandler_Preview_Success(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainPreviewRouter(t, domainSvc)

	domain := &model.Domain{Name: "Preview Domain", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	w := postDomainPreview(t, router, "/api/v1/domain/preview", `{"invite_code":"`+domain.InviteCode+`"}`)
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected preview data")
	}
	if data["name"] != "Preview Domain" {
		t.Fatalf("expected preview name 'Preview Domain', got %v", data["name"])
	}
}

func TestDomainHandler_Preview_NotFound(t *testing.T) {
	_, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainPreviewRouter(t, domainSvc)

	w := postDomainPreview(t, router, "/api/v1/domain/preview", `{"invite_code":"BADCODE"}`)
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 3001 {
		t.Fatalf("expected code 3001, got %d", code)
	}
}
