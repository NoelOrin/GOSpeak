package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func setupGuildHandlerTestDB(t *testing.T) (*gorm.DB, *service.GuildService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Guild{}, &model.GuildMember{}, &model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	guildRepo := repository.NewGuildRepository(db)
	guildSvc := service.NewGuildService(guildRepo)
	return db, guildSvc
}

func setupGuildHandlerRouter(t *testing.T, guildSvc *service.GuildService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// permSvc can be nil for tests that don't use permissions
	h := NewGuildHandler(guildSvc, nil)

	// Mock JWT auth middleware: inject context keys matching production JWTAuth()
	r.Use(func(c *gin.Context) {
		userUUID := c.GetHeader("X-User-UUID")
		if userUUID == "" {
			userUUID = "default-user"
		}
		c.Set("user_uuid", userUUID)
		c.Set("username", userUUID)    // username = user_uuid in test fixtures
		c.Set("role", c.GetHeader("X-User-Role"))
		c.Set("auth_type", "jwt")
		c.Set("permissions", []string{})
		c.Next()
	})

	rg := r.Group("/api/v1/guild")
	rg.POST("/create", h.Create)
	rg.POST("/get", h.Get)
	rg.POST("/list", h.List)
	rg.POST("/list-public", h.ListPublic)
	rg.POST("/my-guilds", h.MyGuilds)
	rg.POST("/join", h.Join)
	rg.POST("/leave", h.Leave)
	rg.POST("/kick", h.Kick)
	rg.POST("/update", h.Update)
	rg.POST("/delete", h.Delete)
	rg.POST("/members", h.Members)

	return r
}

func postGuildJSON(t *testing.T, router *gin.Engine, path, body string, headers ...map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, h := range headers {
		for k, v := range h {
			req.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func parseGuildResp(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}
	return resp
}

func intCode(v interface{}) int {
	f, ok := v.(float64)
	if ok {
		return int(f)
	}
	return 0
}

func TestGuildHandler_Create_InvalidJSON(t *testing.T) {
	_, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	w := postGuildJSON(t, router, "/api/v1/guild/create", "not-json", map[string]string{"X-User-UUID": "owner-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("expected code 2001, got %d", code)
	}
}

func TestGuildHandler_Create_MissingName(t *testing.T) {
	_, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	w := postGuildJSON(t, router, "/api/v1/guild/create", `{}`, map[string]string{"X-User-UUID": "owner-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("expected code 2001, got %d", code)
	}
}

func TestGuildHandler_Create_Success(t *testing.T) {
	_, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	w := postGuildJSON(t, router, "/api/v1/guild/create", `{"name":"Test Guild"}`, map[string]string{"X-User-UUID": "owner-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object")
	}
	if data["name"] != "Test Guild" {
		t.Fatalf("expected name 'Test Guild', got %v", data["name"])
	}
	if data["owner_uuid"] != "owner-1" {
		t.Fatalf("expected owner_uuid 'owner-1', got %v", data["owner_uuid"])
	}
}

func TestGuildHandler_Get_NotFound(t *testing.T) {
	_, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	w := postGuildJSON(t, router, "/api/v1/guild/get", `{"uuid":"nonexistent-uuid"}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 3001 {
		t.Fatalf("expected code 3001, got %d", code)
	}
}

func TestGuildHandler_Get_Success(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	// seed guild
	g := &model.Guild{Name: "Test Guild", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}

	w := postGuildJSON(t, router, "/api/v1/guild/get", `{"uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}

func TestGuildHandler_Join_InvalidCode(t *testing.T) {
	_, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	w := postGuildJSON(t, router, "/api/v1/guild/join", `{}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("expected code 2001, got %d", code)
	}
}

func TestGuildHandler_Join_NotFound(t *testing.T) {
	_, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	w := postGuildJSON(t, router, "/api/v1/guild/join", `{"invite_code":"BADCODE"}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 3001 {
		t.Fatalf("expected code 3001, got %d", code)
	}
}

func TestGuildHandler_Leave_Success(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	// seed guild + owner + member
	g := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}
	db.Create(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "member-1", RoleName: "member"})

	w := postGuildJSON(t, router, "/api/v1/guild/leave", `{"uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "member-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}

func TestGuildHandler_ListPagination(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	for i := 0; i < 3; i++ {
			g := &model.Guild{Name: fmt.Sprintf("Guild-%d", i), OwnerUUID: "owner-1"}
		if err := db.Create(g).Error; err != nil {
			t.Fatalf("seed guild: %v", err)
		}
	}

	w := postGuildJSON(t, router, "/api/v1/guild/list", `{"page":1,"page_size":2}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object")
	}
	total := intCode(data["total"])
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
}

func TestGuildHandler_Delete_NotOwner(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	g := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}

	// non-owner tries to delete
	w := postGuildJSON(t, router, "/api/v1/guild/delete", `{"uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "not-owner"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected code 1013 (FORBIDDEN), got %d", code)
	}
}

func TestGuildHandler_Delete_OwnerSuccess(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	g := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}

	w := postGuildJSON(t, router, "/api/v1/guild/delete", `{"uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "owner-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}

func TestGuildHandler_Kick_NotAdmin(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	g := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}
	db.Create(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "member-1", RoleName: "member"})
	db.Create(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "member-2", RoleName: "member"})

	// member tries to kick another member (should fail - not admin/owner)
	w := postGuildJSON(t, router, "/api/v1/guild/kick", `{"guild_uuid":"`+g.UUID+`","user_uuid":"member-2"}`, map[string]string{"X-User-UUID": "member-1"})
	resp := parseGuildResp(t, w.Body.String())
	// Expect FORBIDDEN (1013) since member-1 is not owner/admin
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected code 1013 (FORBIDDEN), got %d", code)
	}
}

func TestGuildHandler_Members_Success(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	g := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}
	db.Create(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "owner-1", RoleName: "owner"})

	w := postGuildJSON(t, router, "/api/v1/guild/members", `{"guild_uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}
