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

func setupGuildPreviewRouter(t *testing.T, guildSvc *service.GuildService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewGuildHandler(guildSvc, nil)
	r.Use(func(c *gin.Context) {
		c.Set("user_uuid", c.GetHeader("X-User-UUID"))
		c.Next()
	})
	rg := r.Group("/api/v1/guild")
	rg.POST("/preview", h.Preview)
	rg.POST("/join", h.Join)
	return r
}

func postGuildPreview(t *testing.T, router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestGuildHandler_Preview_Success(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildPreviewRouter(t, guildSvc)

	guild := &model.Guild{Name: "Preview Guild", OwnerUUID: "owner-1"}
	if err := db.Create(guild).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}

	w := postGuildPreview(t, router, "/api/v1/guild/preview", `{"invite_code":"`+guild.InviteCode+`"}`)
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected preview data")
	}
	if data["name"] != "Preview Guild" {
		t.Fatalf("expected preview name 'Preview Guild', got %v", data["name"])
	}
}

func TestGuildHandler_Preview_NotFound(t *testing.T) {
	_, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildPreviewRouter(t, guildSvc)

	w := postGuildPreview(t, router, "/api/v1/guild/preview", `{"invite_code":"BADCODE"}`)
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 3001 {
		t.Fatalf("expected code 3001, got %d", code)
	}
}
