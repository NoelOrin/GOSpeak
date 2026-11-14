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

type inviteJoinHandlerEnv struct {
	router    *gin.Engine
	db        *gorm.DB
	inviteSvc *service.InviteService
}

func newInviteJoinHandlerEnv(t *testing.T) *inviteJoinHandlerEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Domain{}, &model.DomainMember{},
		&model.DomainRole{}, &model.DomainRolePermission{},
		&model.DomainInvite{}, &model.DomainInviteRedemption{},
		&model.DomainJoinRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domainRepo := repository.NewDomainRepository(db)
	userRepo := repository.NewUserRepository(db)
	inviteRepo := repository.NewDomainInviteRepository(db)
	inviteSvc := service.NewInviteService(inviteRepo, domainRepo, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "jwt-key")
	joinReqSvc := service.NewJoinRequestService(repository.NewDBTxRunner(db), repository.NewDomainJoinRequestRepository(db), domainRepo, userRepo)
	joinSvc := service.NewInviteJoinService(repository.NewDBTxRunner(db), inviteSvc, joinReqSvc, inviteRepo, domainRepo, userRepo, nil, nil)
	h := NewInviteJoinHandler(joinSvc, defaultAuthCookieConfig())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		if uuid := c.GetHeader("X-User-UUID"); uuid != "" {
			c.Set("user_uuid", uuid)
		}
		c.Next()
	})
	r.POST("/api/v1/invite/register", h.Register)
	r.POST("/api/v1/invite/join", h.Join)
	r.POST("/api/v1/invite/my-request", h.MyRequest)
	r.POST("/api/v1/invite/cancel-request", h.CancelRequest)
	return &inviteJoinHandlerEnv{router: r, db: db, inviteSvc: inviteSvc}
}

func (e *inviteJoinHandlerEnv) seedDomainInvite(t *testing.T, requireApproval bool) (*model.Domain, *model.DomainInvite) {
	t.Helper()
	d := &model.Domain{Name: "JoinRealm", OwnerUUID: "owner-1", RequireJoinApproval: requireApproval}
	if err := e.db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	invite, err := e.inviteSvc.Create(d.UUID, "测试", nil, 0, "owner-1")
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	return d, invite
}

func (e *inviteJoinHandlerEnv) post(t *testing.T, path, body string, headers ...map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, h := range headers {
		for k, v := range h {
			req.Header.Set(k, v)
		}
	}
	e.router.ServeHTTP(rec, req)
	return rec
}

func decodeJoinResp(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body.String())
	}
	return resp
}

func TestInviteJoinHandler_Register_DirectJoinSetsCookie(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	_, invite := env.seedDomainInvite(t, false)

	rec := env.post(t, "/api/v1/invite/register",
		`{"code":"`+invite.Code+`","username":"handler1","password":"password123","nickname":"甲"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeJoinResp(t, rec)
	data, _ := resp["data"].(map[string]interface{})
	if data["status"] != "joined" {
		t.Fatalf("status = %v, want joined", data["status"])
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("register must set auth cookies")
	}
}

func TestInviteJoinHandler_Register_PendingWhenApprovalRequired(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	_, invite := env.seedDomainInvite(t, true)

	rec := env.post(t, "/api/v1/invite/register",
		`{"code":"`+invite.Code+`","username":"handler2","password":"password123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeJoinResp(t, rec)
	data, _ := resp["data"].(map[string]interface{})
	if data["status"] != "pending" {
		t.Fatalf("status = %v, want pending", data["status"])
	}
}

func TestInviteJoinHandler_Register_MissingCodeReturns2001(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	rec := env.post(t, "/api/v1/invite/register", `{"username":"x","password":"password123"}`)
	resp := decodeJoinResp(t, rec)
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("code = %d, want 2001", code)
	}
}

func TestInviteJoinHandler_Register_InvalidCodeReturns2001(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	rec := env.post(t, "/api/v1/invite/register",
		`{"code":"`+strings.Repeat("A", 36)+`","username":"handler3","password":"password123"}`)
	resp := decodeJoinResp(t, rec)
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("code = %d, want 2001", code)
	}
	if strings.Contains(rec.Body.String(), "JoinRealm") {
		t.Fatal("invalid code must not leak domain info")
	}
}

func TestInviteJoinHandler_Join_RequiresAuth(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	rec := env.post(t, "/api/v1/invite/join", `{"domain_uuid":"whatever"}`)
	resp := decodeJoinResp(t, rec)
	if code := intCode(resp["code"]); code == 0 {
		t.Fatalf("join without user must fail, got code 0")
	}
}

func TestInviteJoinHandler_Join_ByCode(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	_, invite := env.seedDomainInvite(t, false)
	if err := env.db.Create(&model.User{Name: "joiner"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var user model.User
	if err := env.db.Where("name = ?", "joiner").First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	rec := env.post(t, "/api/v1/invite/join", `{"code":"`+invite.Code+`"}`,
		map[string]string{"X-User-UUID": user.UUID})
	resp := decodeJoinResp(t, rec)
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("code = %d; body=%s", code, rec.Body.String())
	}
}

func TestInviteJoinHandler_MyRequestAndCancel(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	d, _ := env.seedDomainInvite(t, true)
	d.IsPublic = true
	if err := env.db.Save(d).Error; err != nil {
		t.Fatalf("make public: %v", err)
	}
	if err := env.db.Create(&model.User{Name: "waiter2"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var user model.User
	if err := env.db.Where("name = ?", "waiter2").First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	headers := map[string]string{"X-User-UUID": user.UUID}

	rec := env.post(t, "/api/v1/invite/join", `{"domain_uuid":"`+d.UUID+`"}`, headers)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 0 {
		t.Fatalf("join code = %d; body=%s", code, rec.Body.String())
	}

	rec = env.post(t, "/api/v1/invite/my-request", `{"domain_uuid":"`+d.UUID+`"}`, headers)
	resp := decodeJoinResp(t, rec)
	data, _ := resp["data"].(map[string]interface{})
	if data["status"] != "pending" {
		t.Fatalf("status = %v, want pending", data["status"])
	}

	rec = env.post(t, "/api/v1/invite/cancel-request", `{"domain_uuid":"`+d.UUID+`"}`, headers)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 0 {
		t.Fatalf("cancel code = %d; body=%s", code, rec.Body.String())
	}

	rec = env.post(t, "/api/v1/invite/my-request", `{"domain_uuid":"`+d.UUID+`"}`, headers)
	resp = decodeJoinResp(t, rec)
	data, _ = resp["data"].(map[string]interface{})
	if data["status"] != "" {
		t.Fatalf("status after cancel = %v, want empty", data["status"])
	}
}
