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
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.Room{}, &model.User{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domainRepo := repository.NewDomainRepository(db)
	domainRoleRepo := repository.NewDomainRoleRepository(db)
	domainSvc := service.NewDomainService(domainRepo, domainRoleRepo)
	return db, domainSvc
}

func setupDomainHandlerRouter(t *testing.T, domainSvc *service.DomainService) *gin.Engine {
	return setupDomainHandlerRouterWithPermSvc(t, domainSvc, nil)
}

func setupDomainHandlerRouterWithPermSvc(t *testing.T, domainSvc *service.DomainService, permSvc *service.PermissionService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := NewDomainHandler(domainSvc, permSvc)

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
	rg.POST("/preview", h.Preview)
	rg.POST("/leave", h.Leave)
	rg.POST("/kick", h.Kick)
	rg.POST("/update", h.Update)
	rg.POST("/reset-invite", h.ResetInvite)
	rg.POST("/delete", h.Delete)
	rg.POST("/members", h.Members)
	rg.POST("/roles/list", h.ListRoles)
	rg.POST("/roles/create", h.CreateRole)
	rg.POST("/roles/update", h.UpdateRole)
	rg.POST("/roles/delete", h.DeleteRole)
	rg.POST("/members/update-role", h.UpdateMemberRole)
	rg.POST("/my-permissions", h.MyPermissions)

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

func TestDomainHandler_MyDomains_BatchDetails(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g1 := &model.Domain{Name: "G1", OwnerUUID: "owner-1"}
	if err := db.Create(g1).Error; err != nil {
		t.Fatalf("seed domain g1: %v", err)
	}
	g2 := &model.Domain{Name: "G2", OwnerUUID: "owner-2"}
	if err := db.Create(g2).Error; err != nil {
		t.Fatalf("seed domain g2: %v", err)
	}

	members := []model.DomainMember{
		{DomainUUID: g1.UUID, UserUUID: "owner-1", RoleName: "owner"},
		{DomainUUID: g1.UUID, UserUUID: "member-1", RoleName: "member"},
		{DomainUUID: g1.UUID, UserUUID: "member-2", RoleName: "member"},
		{DomainUUID: g2.UUID, UserUUID: "owner-2", RoleName: "owner"},
		{DomainUUID: g2.UUID, UserUUID: "member-1", RoleName: "member"},
	}
	for i := range members {
		if err := db.Create(&members[i]).Error; err != nil {
			t.Fatalf("seed member: %v", err)
		}
	}

	rooms := []model.Room{
		{Name: "g1-room-1", DomainUUID: g1.UUID},
		{Name: "g1-room-2", DomainUUID: g1.UUID},
		{Name: "g2-room-1", DomainUUID: g2.UUID},
	}
	for i := range rooms {
		if err := db.Create(&rooms[i]).Error; err != nil {
			t.Fatalf("seed room: %v", err)
		}
	}

	w := postDomainJSON(t, router, "/api/v1/domain/my-domains", `{}`, map[string]string{"X-User-UUID": "member-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	rows, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data array, got %#v", resp["data"])
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 domain details, got %d", len(rows))
	}

	byUUID := make(map[string]map[string]interface{}, len(rows))
	for _, row := range rows {
		detail, ok := row.(map[string]interface{})
		if !ok {
			t.Fatalf("expected detail object, got %#v", row)
		}
		uuid, _ := detail["uuid"].(string)
		byUUID[uuid] = detail
	}

	g1Detail, ok := byUUID[g1.UUID]
	if !ok {
		t.Fatalf("expected detail for %s, got %v", g1.UUID, byUUID)
	}
	if g1Detail["name"] != "G1" {
		t.Fatalf("expected g1 name G1, got %v", g1Detail["name"])
	}
	if intCode(g1Detail["member_count"]) != 3 {
		t.Fatalf("expected g1 member_count 3, got %v", g1Detail["member_count"])
	}
	if intCode(g1Detail["room_count"]) != 2 {
		t.Fatalf("expected g1 room_count 2, got %v", g1Detail["room_count"])
	}

	g2Detail, ok := byUUID[g2.UUID]
	if !ok {
		t.Fatalf("expected detail for %s, got %v", g2.UUID, byUUID)
	}
	if intCode(g2Detail["member_count"]) != 2 || intCode(g2Detail["room_count"]) != 1 {
		t.Fatalf("expected g2 member_count 2 room_count 1, got %v/%v", g2Detail["member_count"], g2Detail["room_count"])
	}
}

func TestDomainHandler_RoleManagement_OwnerSuccess(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	domain := &model.Domain{Name: "Role API", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "member-1", RoleName: model.DomainRoleMember,
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	router := setupDomainHandlerRouter(t, domainSvc)

	resp := postDomainJSON(t, router, "/api/v1/domain/roles/list",
		`{"domain_uuid":"`+domain.UUID+`"}`,
		map[string]string{"X-User-UUID": "owner-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("list roles: got %d, body %s", resp.Code, resp.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"].(float64) != 0 {
		t.Fatalf("expected code 0, got %v", body["code"])
	}

	resp = postDomainJSON(t, router, "/api/v1/domain/roles/create",
		`{"domain_uuid":"`+domain.UUID+`","name":"moderator","permissions":["room:read"]}`,
		map[string]string{"X-User-UUID": "owner-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("create role: got %d, body %s", resp.Code, resp.Body.String())
	}

	resp = postDomainJSON(t, router, "/api/v1/domain/members/update-role",
		`{"domain_uuid":"`+domain.UUID+`","user_uuid":"member-1","role_name":"moderator"}`,
		map[string]string{"X-User-UUID": "owner-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("update member role: got %d, body %s", resp.Code, resp.Body.String())
	}
}

func TestDomainHandler_RoleManagement_NonManagerForbidden(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	domain := &model.Domain{Name: "Role Denied", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "member-1", RoleName: model.DomainRoleMember,
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	router := setupDomainHandlerRouter(t, domainSvc)

	resp := postDomainJSON(t, router, "/api/v1/domain/roles/create",
		`{"domain_uuid":"`+domain.UUID+`","name":"hacker","permissions":[]}`,
		map[string]string{"X-User-UUID": "member-1"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("unexpected http status %d, body %s", resp.Code, resp.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"].(float64) != 1013 {
		t.Fatalf("expected FORBIDDEN 1013, got %v", body["code"])
	}
}

func TestDomainHandler_MyPermissions(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	domain := &model.Domain{Name: "My Perms", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "owner-1", RoleName: model.DomainRoleOwner,
	}).Error; err != nil {
		t.Fatalf("seed owner member: %v", err)
	}
	router := setupDomainHandlerRouter(t, domainSvc)

	resp := postDomainJSON(t, router, "/api/v1/domain/my-permissions",
		`{"domain_uuid":"`+domain.UUID+`"}`,
		map[string]string{"X-User-UUID": "owner-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("my permissions: got %d, body %s", resp.Code, resp.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"].(float64) != 0 {
		t.Fatalf("expected code 0, got %v", body["code"])
	}
}

func TestDomainHandler_Update_AllowsDomainRolePermission(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	domain := &model.Domain{Name: "Update Role", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "manager-1", RoleName: "manager",
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := repository.NewDomainRoleRepository(db).CreateRoleWithPermissions(
		&model.DomainRole{DomainUUID: domain.UUID, Name: "manager"},
		[]string{model.PermDomainManage},
	); err != nil {
		t.Fatalf("create role: %v", err)
	}
	router := setupDomainHandlerRouter(t, domainSvc)

	resp := postDomainJSON(t, router, "/api/v1/domain/update",
		`{"domain_uuid":"`+domain.UUID+`","name":"Renamed"}`,
		map[string]string{"X-User-UUID": "manager-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", resp.Code, resp.Body.String())
	}
}

func TestDomainHandler_Update_AllowsGlobalPermissionFallback(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	domain := &model.Domain{Name: "Global Fallback", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "member-1", RoleName: model.DomainRoleMember,
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}

	permRepo := repository.NewPermissionRepository(db)
	if err := db.AutoMigrate(&model.Permission{}, &model.RolePermission{}); err != nil {
		t.Fatalf("migrate permissions: %v", err)
	}
	for _, perm := range model.DefaultPermissions {
		if err := permRepo.CreateIfNotExists(&perm); err != nil {
			t.Fatalf("seed permission %s: %v", perm.Code, err)
		}
	}
	if err := permRepo.SeedRolePermissionsIfEmpty("platform-admin", []string{model.PermDomainManage}); err != nil {
		t.Fatalf("seed role permission: %v", err)
	}
	permSvc := service.NewPermissionService(permRepo)
	if err := permSvc.LoadCache(); err != nil {
		t.Fatalf("load permission cache: %v", err)
	}

	router := setupDomainHandlerRouterWithPermSvc(t, domainSvc, permSvc)
	resp := postDomainJSON(t, router, "/api/v1/domain/update",
		`{"domain_uuid":"`+domain.UUID+`","name":"Renamed"}`,
		map[string]string{"X-User-UUID": "member-1", "X-User-Role": "platform-admin"})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", resp.Code, resp.Body.String())
	}
}

func TestDomainHandler_RoleManagement_AdminPermissionsAreFixed(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	domain := &model.Domain{Name: "Fixed Roles", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "owner-1", RoleName: model.DomainRoleOwner,
	}).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	router := setupDomainHandlerRouter(t, domainSvc)

	for _, roleName := range []string{model.DomainRoleAdmin, model.DomainRoleOwner} {
		resp := postDomainJSON(t, router, "/api/v1/domain/roles/update",
			`{"domain_uuid":"`+domain.UUID+`","role_name":"`+roleName+`","permissions":["room:read"]}`,
			map[string]string{"X-User-UUID": "owner-1"})
		if resp.Code != http.StatusForbidden {
			t.Fatalf("update %s: expected 403, got %d body %s", roleName, resp.Code, resp.Body.String())
		}
	}
}

func TestDomainHandler_ResetInvite_OwnerSuccess(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g := &model.Domain{Name: "Reset Domain", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	oldCode := g.InviteCode
	if oldCode == "" {
		t.Fatal("expected seeded invite_code")
	}

	w := postDomainJSON(t, router, "/api/v1/domain/reset-invite",
		`{"domain_uuid":"`+g.UUID+`"}`,
		map[string]string{"X-User-UUID": "owner-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object")
	}
	newCode, _ := data["invite_code"].(string)
	if newCode == "" || newCode == oldCode {
		t.Fatalf("expected new invite_code, got %q (old %q)", newCode, oldCode)
	}

	// 旧邀请码应立即失效
	pw := postDomainJSON(t, router, "/api/v1/domain/preview",
		`{"invite_code":"`+oldCode+`"}`,
		map[string]string{"X-User-UUID": "user-1"})
	presp := parseDomainResp(t, pw.Body.String())
	if intCode(presp["code"]) == 0 {
		t.Fatalf("expected old invite_code to be invalid, got code 0")
	}
	_ = domainSvc
}

func TestDomainHandler_ResetInvite_MemberForbidden(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g := &model.Domain{Name: "Forbidden Domain", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	member := &model.DomainMember{DomainUUID: g.UUID, UserUUID: "member-2", RoleName: "member"}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}

	w := postDomainJSON(t, router, "/api/v1/domain/reset-invite",
		`{"domain_uuid":"`+g.UUID+`"}`,
		map[string]string{"X-User-UUID": "member-2"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected FORBIDDEN (1013), got %d: %s", code, resp["msg"])
	}
	_ = domainSvc
}
