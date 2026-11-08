package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type guestHandlerEnv struct {
	router *gin.Engine
	db     *gorm.DB
}

func newGuestHandlerEnv(t *testing.T) *guestHandlerEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Domain{}, &model.DomainMember{}, &model.DomainGuestBan{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	guestSvc := service.NewGuestService(
		db,
		repository.NewUserRepository(db),
		repository.NewDomainRepository(db),
		repository.NewGuestBanRepo(db),
		service.NewAuthService(repository.NewUserRepository(db), nil, nil),
		nil,
	)
	h := NewGuestHandler(guestSvc, nil, nil, defaultAuthCookieConfig())
	router := gin.New()
	router.POST("/api/v1/auth/guest", h.Join)
	return &guestHandlerEnv{router: router, db: db}
}

func (e *guestHandlerEnv) seedDomain(t *testing.T, allowGuest bool) *model.Domain {
	t.Helper()
	d := &model.Domain{Name: "GuestRealm", OwnerUUID: "00000000-0000-0000-0000-000000000001", AllowGuest: allowGuest}
	if err := e.db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func (e *guestHandlerEnv) post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.router.ServeHTTP(rec, req)
	return rec
}

func decodeGuestResp(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return resp
}

func TestGuestHandler_Join(t *testing.T) {
	env := newGuestHandlerEnv(t)
	d := env.seedDomain(t, true)

	rec := env.post(t, "/api/v1/auth/guest", `{"nickname":"路人甲","invite_code":"`+d.InviteCode+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expect 200, got %d: %s", rec.Code, rec.Body.String())
	}
	resp := decodeGuestResp(t, rec)
	if intCode(resp["code"]) != int(pkg.SUCCESS) {
		t.Fatalf("expect success code, got %v", resp["code"])
	}
	data := resp["data"].(map[string]interface{})
	if data["access_token"] == "" || data["refresh_token"] == "" {
		t.Fatal("missing tokens in response")
	}
	if rec.Header().Get("Set-Cookie") == "" {
		t.Fatal("must set auth cookie like login")
	}
	user := data["user"].(map[string]interface{})
	if user["is_guest"] != true {
		t.Fatal("expect is_guest user")
	}
	domain := data["domain"].(map[string]interface{})
	if domain["invite_code"] != "" {
		t.Fatal("invite code must not leak to guest")
	}
}

func TestGuestHandler_Join_RejectsDisabledDomain(t *testing.T) {
	env := newGuestHandlerEnv(t)
	d := env.seedDomain(t, false)

	rec := env.post(t, "/api/v1/auth/guest", `{"nickname":"x","invite_code":"`+d.InviteCode+`"}`)
	resp := decodeGuestResp(t, rec)
	if intCode(resp["code"]) != int(pkg.FORBIDDEN) {
		t.Fatalf("expected 1013 forbidden, got %v: %s", resp["code"], rec.Body.String())
	}
}

func TestGuestHandler_Join_InvalidParams(t *testing.T) {
	env := newGuestHandlerEnv(t)

	rec := env.post(t, "/api/v1/auth/guest", `{"nickname":""}`)
	resp := decodeGuestResp(t, rec)
	if intCode(resp["code"]) != int(pkg.INVALID_PARAMS) {
		t.Fatalf("expected 2001 invalid params, got %v", resp["code"])
	}
}

type stubDomainPerm struct{ granted bool }

func (s *stubDomainPerm) HasDomainPermission(domainUUID, userUUID, permCode string) bool {
	return s.granted
}

type stubGlobalPerm struct{}

func (s *stubGlobalPerm) HasPermission(roleName, permCode string) bool { return false }

func newAdminGuestHandlerEnv(t *testing.T, granted bool) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Domain{}, &model.DomainMember{}, &model.DomainGuestBan{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	guestSvc := service.NewGuestService(
		db,
		repository.NewUserRepository(db),
		repository.NewDomainRepository(db),
		repository.NewGuestBanRepo(db),
		service.NewAuthService(repository.NewUserRepository(db), nil, nil),
		nil,
	)
	h := NewGuestHandler(guestSvc, &stubDomainPerm{granted: granted}, &stubGlobalPerm{}, defaultAuthCookieConfig())
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_uuid", "admin-1"); c.Next() })
	r.POST("/api/v1/domain/guest/config", h.Config)
	return r, db
}

func TestGuestHandler_Config_PermissionDenied(t *testing.T) {
	r, _ := newAdminGuestHandlerEnv(t, false)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domain/guest/config",
		strings.NewReader(`{"domain_uuid":"d1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	resp := decodeGuestResp(t, rec)
	if intCode(resp["code"]) != int(pkg.FORBIDDEN) {
		t.Fatalf("expect 1013, got %v: %s", resp["code"], rec.Body.String())
	}
}

func TestGuestHandler_Config_ReadAndWrite(t *testing.T) {
	r, db := newAdminGuestHandlerEnv(t, true)
	d := &model.Domain{Name: "CfgRealm", OwnerUUID: "00000000-0000-0000-0000-000000000009", AllowGuest: false}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domain/guest/config",
		strings.NewReader(`{"domain_uuid":"`+d.UUID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	resp := decodeGuestResp(t, rec)
	if intCode(resp["code"]) != int(pkg.SUCCESS) {
		t.Fatalf("read config failed: %s", rec.Body.String())
	}
	domainData := resp["data"].(map[string]interface{})
	if domainData["allow_guest"] != false {
		t.Fatal("expect default allow_guest=false")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/domain/guest/config",
		strings.NewReader(`{"domain_uuid":"`+d.UUID+`","allow_guest":true,"guest_limit":7}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	resp = decodeGuestResp(t, rec)
	if intCode(resp["code"]) != int(pkg.SUCCESS) {
		t.Fatalf("write config failed: %s", rec.Body.String())
	}
	domainData = resp["data"].(map[string]interface{})
	if domainData["allow_guest"] != true || domainData["guest_limit"] != float64(7) {
		t.Fatalf("write not applied: %v", domainData)
	}
}
