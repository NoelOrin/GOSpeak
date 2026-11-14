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

// stubReviewPerm 是 JoinRequestHandler 依赖的域权限判决桩。
type stubReviewPerm struct{ granted bool }

func (s *stubReviewPerm) HasDomainPermission(domainUUID, userUUID, permCode string) bool {
	return s.granted
}

type joinRequestHandlerEnv struct {
	router *gin.Engine
	db     *gorm.DB
	svc    *service.JoinRequestService
}

func newJoinRequestHandlerEnv(t *testing.T, granted bool) *joinRequestHandlerEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Domain{}, &model.DomainMember{},
		&model.DomainRole{}, &model.DomainRolePermission{},
		&model.DomainJoinRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domainRepo := repository.NewDomainRepository(db)
	svc := service.NewJoinRequestService(repository.NewDBTxRunner(db), repository.NewDomainJoinRequestRepository(db), domainRepo, repository.NewUserRepository(db))
	h := NewJoinRequestHandler(svc, &stubReviewPerm{granted: granted}, nil)

	r := gin.New()
	// 伪造登录态：与生产 middleware.JWTAuth 注入的 key 保持一致。
	// domain_uuid 由 handler 从请求体自取，这里不预置（本 handler 也不读 context 里的 domain_uuid）。
	r.Use(func(c *gin.Context) {
		userUUID := c.GetHeader("X-User-UUID")
		if userUUID == "" {
			userUUID = "admin-1"
		}
		c.Set("user_uuid", userUUID)
		c.Set("username", userUUID)
		c.Next()
	})
	r.POST("/api/v1/domain/join-requests/list", h.List)
	r.POST("/api/v1/domain/join-requests/review", h.Review)
	r.POST("/api/v1/domain/join-approval/config", h.Config)
	return &joinRequestHandlerEnv{router: r, db: db, svc: svc}
}

func (e *joinRequestHandlerEnv) seedDomain(t *testing.T) *model.Domain {
	t.Helper()
	d := &model.Domain{Name: "ReviewRealm", OwnerUUID: "owner-1", RequireJoinApproval: true}
	if err := e.db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func (e *joinRequestHandlerEnv) seedApplicant(t *testing.T, name string) *model.User {
	t.Helper()
	u := &model.User{Name: name, DisplayName: name}
	if err := e.db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func (e *joinRequestHandlerEnv) post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.router.ServeHTTP(rec, req)
	return rec
}

func TestJoinRequestHandler_List_ReturnsItemsAndCounts(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	u := env.seedApplicant(t, "applicant")
	if _, err := env.svc.Submit(d.UUID, u.UUID, 1); err != nil {
		t.Fatalf("submit: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/join-requests/list", `{"domain_uuid":"`+d.UUID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items  []model.DomainJoinRequestInfo `json:"items"`
			Total  int64                         `json:"total"`
			Counts map[string]int64              `json:"counts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != 0 || resp.Data.Total != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Data.Counts[model.JoinRequestPending] != 1 {
		t.Fatalf("pending count = %d, want 1", resp.Data.Counts[model.JoinRequestPending])
	}
	if resp.Data.Items[0].Name != "applicant" {
		t.Fatalf("applicant name = %q", resp.Data.Items[0].Name)
	}
}

func TestJoinRequestHandler_List_ForbiddenWithoutPermission(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, false)
	d := env.seedDomain(t)
	rec := env.post(t, "/api/v1/domain/join-requests/list", `{"domain_uuid":"`+d.UUID+`"}`)
	resp := decodeJoinResp(t, rec)
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("code = %d, want 1013", code)
	}
}

func TestJoinRequestHandler_List_FiltersByStatus(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	a := env.seedApplicant(t, "a")
	b := env.seedApplicant(t, "b")
	if _, err := env.svc.Submit(d.UUID, a.UUID, 1); err != nil {
		t.Fatalf("submit a: %v", err)
	}
	reqB, err := env.svc.Submit(d.UUID, b.UUID, 1)
	if err != nil {
		t.Fatalf("submit b: %v", err)
	}
	if _, err := env.svc.Review(d.UUID, reqB.ID, true, "admin-1"); err != nil {
		t.Fatalf("approve b: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/join-requests/list",
		`{"domain_uuid":"`+d.UUID+`","status":"approved"}`)
	var resp struct {
		Data struct {
			Items []model.DomainJoinRequestInfo `json:"items"`
			Total int64                         `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Total != 1 || resp.Data.Items[0].UserUUID != b.UUID {
		t.Fatalf("unexpected filtered response: %+v", resp.Data)
	}
}

func TestJoinRequestHandler_Review_ApproveAddsMember(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	u := env.seedApplicant(t, "approveMe")
	req, err := env.svc.Submit(d.UUID, u.UUID, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/join-requests/review",
		`{"domain_uuid":"`+d.UUID+`","request_id":`+itoa(req.ID)+`,"approve":true}`)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 0 {
		t.Fatalf("code = %d; body=%s", code, rec.Body.String())
	}
	if _, err := repository.NewDomainRepository(env.db).GetMember(d.UUID, u.UUID); err != nil {
		t.Fatalf("approved user must be a member: %v", err)
	}
}

func TestJoinRequestHandler_Review_RejectDoesNotAddMember(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	u := env.seedApplicant(t, "rejectMe")
	req, err := env.svc.Submit(d.UUID, u.UUID, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/join-requests/review",
		`{"domain_uuid":"`+d.UUID+`","request_id":`+itoa(req.ID)+`,"approve":false}`)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 0 {
		t.Fatalf("code = %d; body=%s", code, rec.Body.String())
	}
	if _, err := repository.NewDomainRepository(env.db).GetMember(d.UUID, u.UUID); err != gorm.ErrRecordNotFound {
		t.Fatalf("rejected user must not be a member: %v", err)
	}
}

func TestJoinRequestHandler_Review_TwiceReturnsError(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	u := env.seedApplicant(t, "doubleReview")
	req, err := env.svc.Submit(d.UUID, u.UUID, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	body := `{"domain_uuid":"` + d.UUID + `","request_id":` + itoa(req.ID) + `,"approve":true}`
	env.post(t, "/api/v1/domain/join-requests/review", body)
	rec := env.post(t, "/api/v1/domain/join-requests/review", body)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code == 0 {
		t.Fatalf("second review must fail, got code 0")
	}
}

func TestJoinRequestHandler_Review_ForbiddenWithoutPermission(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, false)
	d := env.seedDomain(t)
	u := env.seedApplicant(t, "noPerm")
	req, err := env.svc.Submit(d.UUID, u.UUID, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/join-requests/review",
		`{"domain_uuid":"`+d.UUID+`","request_id":`+itoa(req.ID)+`,"approve":true}`)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 1013 {
		t.Fatalf("code = %d, want 1013", code)
	}
}

func TestJoinRequestHandler_Config_TogglesApproval(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	rec := env.post(t, "/api/v1/domain/join-approval/config",
		`{"domain_uuid":"`+d.UUID+`","require_join_approval":false}`)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 0 {
		t.Fatalf("code = %d; body=%s", code, rec.Body.String())
	}
	var got model.Domain
	if err := env.db.Where("uuid = ?", d.UUID).First(&got).Error; err != nil {
		t.Fatalf("reload domain: %v", err)
	}
	if got.RequireJoinApproval {
		t.Fatal("require_join_approval must be false after toggle")
	}
}

func TestJoinRequestHandler_Config_RequiresManagePermission(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, false)
	d := env.seedDomain(t)
	rec := env.post(t, "/api/v1/domain/join-approval/config",
		`{"domain_uuid":"`+d.UUID+`","require_join_approval":true}`)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 1013 {
		t.Fatalf("code = %d, want 1013", code)
	}
}

// itoa 是测试内的 uint -> 十进制串助手，避免再引 strconv。
func itoa(v uint) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
