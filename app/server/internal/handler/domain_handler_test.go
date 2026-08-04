package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDomainHandlerTestDB(t *testing.T) (*gorm.DB, *service.DomainService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.Room{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domainRepo := repository.NewDomainRepository(db)
	domainSvc := service.NewDomainService(domainRepo)
	return db, domainSvc
}

func setupDomainHandlerRouter(t *testing.T, domainSvc *service.DomainService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// permSvc can be nil for tests that don't use permissions
	h := NewDomainHandler(domainSvc, nil)

	// Mock JWT auth middleware: inject context keys matching production JWTAuth()
	r.Use(func(c *gin.Context) {
		userUUID := c.GetHeader("X-User-UUID")
		if userUUID == "" {
			userUUID = "default-user"
		}
		c.Set("user_uuid", userUUID)
		c.Set("username", userUUID) // username = user_uuid in test fixtures
		c.Set("role", c.GetHeader("X-User-Role"))
		c.Set("auth_type", "jwt")
		c.Set("permissions", []string{})
		domainUUID := c.Query("domain_uuid")
		if domainUUID == "" {
			raw, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			var body struct {
				DomainUUID string `json:"domain_uuid"`
			}
			_ = json.Unmarshal(raw, &body)
			domainUUID = body.DomainUUID
		}
		if domainUUID != "" {
			c.Set("domain_uuid", domainUUID)
		}
		c.Next()
	})

	rg := r.Group("/api/v1/domain")
	rg.POST("/create", h.Create)
	rg.POST("/get", h.Get)
	rg.GET("/get", h.Get)
	rg.POST("/list", h.List)
	rg.POST("/list-public", h.ListPublic)
	rg.POST("/my-domains", h.MyDomains)
	rg.POST("/join", h.Join)
	rg.POST("/leave", h.Leave)
	rg.POST("/kick", h.Kick)
	rg.POST("/update", h.Update)
	rg.POST("/delete", h.Delete)
	rg.POST("/members", h.Members)

	return r
}

func postDomainJSON(t *testing.T, router *gin.Engine, path, body string, headers ...map[string]string) *httptest.ResponseRecorder {
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

func parseDomainResp(t *testing.T, body string) map[string]interface{} {
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

func TestDomainHandler_Create_InvalidJSON(t *testing.T) {
	_, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	w := postDomainJSON(t, router, "/api/v1/domain/create", "not-json", map[string]string{"X-User-UUID": "owner-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("expected code 2001, got %d", code)
	}
}

func TestDomainHandler_Create_MissingName(t *testing.T) {
	_, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	w := postDomainJSON(t, router, "/api/v1/domain/create", `{}`, map[string]string{"X-User-UUID": "owner-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("expected code 2001, got %d", code)
	}
}

func TestDomainHandler_Create_Success(t *testing.T) {
	_, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	w := postDomainJSON(t, router, "/api/v1/domain/create", `{"name":"Test Domain"}`, map[string]string{"X-User-UUID": "owner-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object")
	}
	if data["name"] != "Test Domain" {
		t.Fatalf("expected name 'Test Domain', got %v", data["name"])
	}
	if data["owner_uuid"] != "owner-1" {
		t.Fatalf("expected owner_uuid 'owner-1', got %v", data["owner_uuid"])
	}
}

func TestDomainHandler_Get_NotFound(t *testing.T) {
	_, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	w := postDomainJSON(t, router, "/api/v1/domain/get", `{"domain_uuid":"nonexistent-uuid"}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 3001 {
		t.Fatalf("expected code 3001, got %d", code)
	}
}

func TestDomainHandler_Get_Success(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	// seed domain
	g := &model.Domain{Name: "Test Domain", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	w := postDomainJSON(t, router, "/api/v1/domain/get", `{"domain_uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}

func TestDomainHandler_Get_UsesContextDomainUUID(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g := &model.Domain{Name: "Query Domain", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	// query 与 body 不一致时，必须以 middleware 解析的 query/context 为准。
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domain/get?domain_uuid="+g.UUID, strings.NewReader(`{"domain_uuid":"body-uuid"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-UUID", "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object")
	}
	if data["uuid"] != g.UUID {
		t.Fatalf("expected domain %q, got %v", g.UUID, data["uuid"])
	}
}

func TestDomainHandler_Join_InvalidCode(t *testing.T) {
	_, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	w := postDomainJSON(t, router, "/api/v1/domain/join", `{}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("expected code 2001, got %d", code)
	}
}

func TestDomainHandler_Join_NotFound(t *testing.T) {
	_, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	w := postDomainJSON(t, router, "/api/v1/domain/join", `{"invite_code":"BADCODE"}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 3001 {
		t.Fatalf("expected code 3001, got %d", code)
	}
}

func TestDomainHandler_Leave_Success(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	// seed domain + owner + member
	g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	db.Create(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "member-1", RoleName: "member"})

	w := postDomainJSON(t, router, "/api/v1/domain/leave", `{"domain_uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "member-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}

func TestDomainHandler_ListPagination(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	for i := 0; i < 3; i++ {
		g := &model.Domain{Name: fmt.Sprintf("Domain-%d", i), OwnerUUID: "owner-1"}
		if err := db.Create(g).Error; err != nil {
			t.Fatalf("seed domain: %v", err)
		}
	}

	w := postDomainJSON(t, router, "/api/v1/domain/list", `{"page":1,"page_size":2}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseDomainResp(t, w.Body.String())
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

func TestDomainHandler_Delete_NotOwner(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	// non-owner tries to delete
	w := postDomainJSON(t, router, "/api/v1/domain/delete", `{"domain_uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "not-owner"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected code 1013 (FORBIDDEN), got %d", code)
	}
}

func TestDomainHandler_Delete_OwnerSuccess(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	w := postDomainJSON(t, router, "/api/v1/domain/delete", `{"domain_uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "owner-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}

func TestDomainHandler_Kick_NotAdmin(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	db.Create(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "member-1", RoleName: "member"})
	db.Create(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "member-2", RoleName: "member"})

	// member tries to kick another member (should fail - not admin/owner)
	w := postDomainJSON(t, router, "/api/v1/domain/kick", `{"domain_uuid":"`+g.UUID+`","user_uuid":"member-2"}`, map[string]string{"X-User-UUID": "member-1"})
	resp := parseDomainResp(t, w.Body.String())
	// Expect FORBIDDEN (1013) since member-1 is not owner/admin
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected code 1013 (FORBIDDEN), got %d", code)
	}
}

func TestDomainHandler_Members_Success(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := db.Create(&model.User{UUID: "owner-1", Name: "OwnerName", DisplayName: "域主"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "owner-1", RoleName: "owner"}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}

	w := postDomainJSON(t, router, "/api/v1/domain/members", `{"domain_uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "user-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data object, got %#v", resp["data"])
	}
	members, ok := data["members"].([]interface{})
	if !ok || len(members) != 1 {
		t.Fatalf("expected 1 member in response, got %#v", data)
	}
	first := members[0].(map[string]interface{})
	if first["name"] != "OwnerName" {
		t.Fatalf("expected member name OwnerName, got %v", first["name"])
	}
}
