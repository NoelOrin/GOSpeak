package handler

import (
	"encoding/json"
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

// stubInviteDomainPerm 是域权限判决桩。
type stubInviteDomainPerm struct{ granted bool }

func (s stubInviteDomainPerm) HasDomainPermission(_, _, _ string) bool { return s.granted }

// stubInviteGlobalPerm 是平台权限判决桩。
type stubInviteGlobalPerm struct{ granted bool }

func (s stubInviteGlobalPerm) HasPermission(_, _ string) bool { return s.granted }

type inviteHandlerEnv struct {
	router *gin.Engine
	db     *gorm.DB
	svc    *service.InviteService
}

func newInviteHandlerEnv(t *testing.T) *inviteHandlerEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.DomainInvite{}, &model.DomainInviteRedemption{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := service.NewInviteService(
		db,
		repository.NewDomainInviteRepository(db),
		repository.NewDomainRepository(db),
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"jwt-key",
	)
	h := NewInviteHandler(svc)
	// 管理端点需要权限判决依赖；测试注入「全部放行」的桩。
	h.SetPermissionCheckers(stubInviteDomainPerm{granted: true}, stubInviteGlobalPerm{granted: true})
	router := gin.New()
	router.POST("/api/v1/domain/invites/create", h.Create)
	router.POST("/api/v1/domain/invites/list", h.List)
	router.POST("/api/v1/domain/invites/disable", h.Disable)
	router.POST("/api/v1/domain/invites/redemptions", h.Redemptions)
	router.POST("/api/v1/invite/preview", h.Preview)
	return &inviteHandlerEnv{router: router, db: db, svc: svc}
}

func (e *inviteHandlerEnv) seedDomain(t *testing.T) *model.Domain {
	t.Helper()
	d := &model.Domain{Name: "InviteRealm", OwnerUUID: "00000000-0000-0000-0000-000000000001"}
	if err := e.db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func (e *inviteHandlerEnv) post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.router.ServeHTTP(rec, req)
	return rec
}

func TestInviteHandler_Create_ReturnsCode(t *testing.T) {
	env := newInviteHandlerEnv(t)
	d := env.seedDomain(t)
	rec := env.post(t, "/api/v1/domain/invites/create", `{"domain_uuid":"`+d.UUID+`","label":"测试"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Code int                `json:"code"`
		Data model.DomainInvite `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != 0 || len(resp.Data.Code) != 36 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestInviteHandler_Create_RejectsMissingDomain(t *testing.T) {
	env := newInviteHandlerEnv(t)
	rec := env.post(t, "/api/v1/domain/invites/create", `{}`)
	if rec.Code == http.StatusOK {
		t.Fatal("missing domain_uuid must fail")
	}
}

func TestInviteHandler_List_ReturnsInvites(t *testing.T) {
	env := newInviteHandlerEnv(t)
	d := env.seedDomain(t)
	if _, err := env.svc.Create(d.UUID, "a", nil, 0, "u1"); err != nil {
		t.Fatalf("create: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/invites/list", `{"domain_uuid":"`+d.UUID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			Invites []model.DomainInvite `json:"invites"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Invites) != 1 {
		t.Fatalf("invites = %d, want 1", len(resp.Data.Invites))
	}
}

func TestInviteHandler_Preview_ReturnsDomainWithoutCode(t *testing.T) {
	env := newInviteHandlerEnv(t)
	d := env.seedDomain(t)
	invite, err := env.svc.Create(d.UUID, "", nil, 0, "u1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	rec := env.post(t, "/api/v1/invite/preview", `{"code":"`+invite.Code+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), invite.Code) {
		t.Fatal("preview must not echo the invite code")
	}
}

func TestInviteHandler_Preview_RejectsInvalidCode(t *testing.T) {
	env := newInviteHandlerEnv(t)
	rec := env.post(t, "/api/v1/invite/preview", `{"code":"`+strings.Repeat("A", 36)+`"}`)
	if rec.Code == http.StatusOK {
		t.Fatal("invalid code must fail")
	}
	if strings.Contains(rec.Body.String(), "InviteRealm") {
		t.Fatal("invalid code must not leak domain info")
	}
}
