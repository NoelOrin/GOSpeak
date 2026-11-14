package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
	"GOSpeak/internal/sfu"

	"github.com/gin-gonic/gin"
	"github.com/livekit/protocol/auth"
)

// ─── Mock SFU Provider ───

type mockSFU struct {
	sfu.BaseProvider
	tokenFn          func(room, identity string) (string, error)
	listRoomsFn      func() ([]sfu.RoomSummary, error)
	listParticipants func(room string) ([]sfu.ParticipantSummary, error)
	host             string
	// emptyHost 模拟"未配置浏览器侧地址"：provider 返回空，由 handler 同源推导。
	emptyHost bool
}

func (m *mockSFU) ProviderName() string { return "mock" }
func (m *mockSFU) Capabilities() sfu.Capabilities {
	return sfu.Capabilities{
		ServerMute:  true,
		ServerKick:  true,
		DeleteRoom:  true,
		ListRooms:   true,
		ListMembers: true,
	}
}
func (m *mockSFU) GenerateToken(room, identity string) (string, error) {
	if m.tokenFn != nil {
		return m.tokenFn(room, identity)
	}
	return "mock-token", nil
}
func (m *mockSFU) ListRooms() ([]sfu.RoomSummary, error) {
	if m.listRoomsFn != nil {
		return m.listRoomsFn()
	}
	return []sfu.RoomSummary{}, nil
}
func (m *mockSFU) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	if m.listParticipants != nil {
		return m.listParticipants(room)
	}
	return []sfu.ParticipantSummary{}, nil
}
func (m *mockSFU) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return nil
}
func (m *mockSFU) RemoveParticipant(room, identity string) error { return nil }
func (m *mockSFU) DeleteRoom(room string) error                  { return nil }
func (m *mockSFU) GetHost() string {
	if m.emptyHost {
		return ""
	}
	if m.host != "" {
		return m.host
	}
	return "wss://test.livekit.cloud"
}

// ─── Helpers ───

func setupRouter(sfu *mockSFU) *gin.Engine {
	return setupRouterWithClusterResolver(sfu, nil)
}

func setupRouterWithClusterResolver(sfu *mockSFU, resolver func(domainUUID string) (string, error)) *gin.Engine {
	return setupRouterFull(sfu, nil, resolver)
}

func setupRouterFull(sfu *mockSFU, checker func(domainUUID, userUUID string) bool, _ func(domainUUID string) (string, error)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := service.NewSFUService(sfu, nil)
	svc.SetDomainMemberChecker(checker)
	h := NewSignalHandler(svc)
	// 模拟 JWTAuth 注入 username
	r.Use(func(c *gin.Context) {
		if c.GetHeader("X-Test-User") != "" {
			c.Set("username", c.GetHeader("X-Test-User"))
		} else {
			c.Set("username", "user-1")
		}
		c.Next()
	})
	r.POST("/token", h.GetJoinToken)
	r.GET("/rooms", h.ListRooms)
	r.GET("/participants", h.ListParticipants)
	return r
}

type mockLiveKitJobPublisher struct {
	published [][]byte
}

func (m *mockLiveKitJobPublisher) PublishLiveKit(ctx context.Context, raw []byte) error {
	m.published = append(m.published, append([]byte(nil), raw...))
	return nil
}

const testLiveKitKey = "APItestkey"

func setupWebhookRouter(sfu *mockSFU, secret string, jobs livekitJobPublisher) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSignalHandler(service.NewSFUService(sfu, nil))
	h.SetLiveKitSecretResolver(func() string { return secret })
	h.SetLiveKitKeyResolver(func() string { return testLiveKitKey })
	if jobs != nil {
		h.SetJobs(jobs)
	}
	r.POST("/webhook", h.LivekitWebhook)
	return r
}

// signLiveKitWebhook 按 LiveKit 真实方式签发：Authorization JWT，
// 并把 body 的 sha256(base64) 写进 claims，用于校验完整性。
func signLiveKitWebhook(t *testing.T, body, secret string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(body))
	at := auth.NewAccessToken(testLiveKitKey, secret).
		SetValidFor(5 * time.Minute).
		SetSha256(base64.StdEncoding.EncodeToString(sum[:]))
	token, err := at.ToJWT()
	if err != nil {
		t.Fatalf("sign webhook: %v", err)
	}
	return token
}

type response struct {
	Code pkg.ErrCode `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func parseResp(t *testing.T, body string) response {
	t.Helper()
	var r response
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("failed to parse response: %v, body: %s", err, body)
	}
	return r
}

// ─── GetJoinToken Tests ───

func TestGetJoinToken_Success(t *testing.T) {

	sfu := &mockSFU{}
	r := setupRouter(sfu)

	body := `{"room":"test-room","identity":"user-1"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data is not a map")
	}
	if data["token"] != "mock-token" {
		t.Errorf("expected token 'mock-token', got %v", data["token"])
	}
	if data["room"] != "test-room" {
		t.Errorf("expected room 'test-room', got %v", data["room"])
	}
	if data["identity"] != "user-1" {
		t.Errorf("expected identity 'user-1', got %v", data["identity"])
	}
	// provider 返回了显式 public url 时直接使用，不做同源推导。
	if data["serverUrl"] != "wss://test.livekit.cloud" {
		t.Errorf("expected serverUrl, got %v", data["serverUrl"])
	}
	if data["sfuRoom"] != "test-room" {
		t.Errorf("expected sfuRoom 'test-room', got %v", data["sfuRoom"])
	}
	if data["domain_uuid"] != "" {
		t.Errorf("expected empty domain_uuid, got %v", data["domain_uuid"])
	}
}

// 未配置 LIVEKIT_PUBLIC_URL 时（默认形态），serverUrl 由请求 Host 推导为同源
// /livekit 地址，浏览器据此连应用内置反代，LiveKit 无需独立域名与证书。
func TestGetJoinToken_DerivesSameOriginServerURL(t *testing.T) {
	r := setupRouter(&mockSFU{emptyHost: true})

	body := `{"room":"test-room"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "https://voice.example.com/token", strings.NewReader(body))
	req.Host = "voice.example.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data is not a map")
	}
	if data["serverUrl"] != "wss://voice.example.com/livekit" {
		t.Fatalf("serverUrl = %v, want wss://voice.example.com/livekit", data["serverUrl"])
	}
}

func TestGetJoinToken_MissingRoom(t *testing.T) {
	r := setupRouter(&mockSFU{})

	body := `{"identity":"user-1"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS (2001), got %d", resp.Code)
	}
}

func TestGetJoinToken_MissingIdentity(t *testing.T) {
	r := setupRouter(&mockSFU{})

	// identity 可省略：服务端用 JWT username 覆盖
	body := `{"room":"test-room"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected SUCCESS, got %d", resp.Code)
	}
	data := resp.Data.(map[string]interface{})
	if data["identity"] != "user-1" {
		t.Fatalf("expected identity from JWT, got %v", data["identity"])
	}
}

func TestGetJoinToken_EmptyBody(t *testing.T) {
	r := setupRouter(&mockSFU{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/token", nil)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS (2001), got %d", resp.Code)
	}
}

func TestGetJoinToken_SFUError(t *testing.T) {
	sfu := &mockSFU{
		tokenFn: func(room, identity string) (string, error) {
			return "", pkg.NewAppError(pkg.SFU_ERROR, "sfu connection failed")
		},
	}
	r := setupRouter(sfu)

	body := `{"room":"test-room","identity":"user-1"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SFU_ERROR {
		t.Fatalf("expected SFU_ERROR (6002), got %d", resp.Code)
	}
}

func TestGetJoinToken_DomainSuccess(t *testing.T) {
	r := setupRouterFull(&mockSFU{}, func(domainUUID, userUUID string) bool {
		return domainUUID == "domain-a" && userUUID == "user-1"
	}, nil)

	body := `{"room":"test-room","domain_uuid":"domain-a"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
	data := resp.Data.(map[string]interface{})
	if data["room"] != "test-room" {
		t.Errorf("expected logical room 'test-room', got %v", data["room"])
	}
	if data["sfuRoom"] != "domain-a:test-room" {
		t.Errorf("expected sfuRoom 'domain-a:test-room', got %v", data["sfuRoom"])
	}
	if data["domain_uuid"] != "domain-a" {
		t.Errorf("expected domain_uuid 'domain-a', got %v", data["domain_uuid"])
	}
}

func TestGetJoinToken_DomainNonMemberForbidden(t *testing.T) {
	r := setupRouterFull(&mockSFU{}, func(domainUUID, userUUID string) bool {
		return false
	}, nil)

	body := `{"room":"test-room","domain_uuid":"domain-a"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.FORBIDDEN {
		t.Fatalf("expected FORBIDDEN (1013), got %d", resp.Code)
	}
}

// ─── Signal Tests ───

func TestLivekitWebhook_MissingSignatureRejected(t *testing.T) {
	r := setupWebhookRouter(&mockSFU{}, "test-secret", nil)
	body := `{"event":"participant_left","room":{"name":"test-room"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(body))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.TOKEN_WRONG {
		t.Fatalf("expected TOKEN_WRONG, got %d", resp.Code)
	}
}

func TestLivekitWebhook_InvalidSignatureRejected(t *testing.T) {
	r := setupWebhookRouter(&mockSFU{}, "test-secret", nil)
	body := `{"event":"participant_left","room":{"name":"test-room"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(body))
	req.Header.Set("Authorization", "t=1,v1=deadbeef")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.TOKEN_WRONG {
		t.Fatalf("expected TOKEN_WRONG, got %d", resp.Code)
	}
}

func TestLivekitWebhook_WrongSecretRejected(t *testing.T) {
	r := setupWebhookRouter(&mockSFU{}, "test-secret", nil)
	body := `{"event":"participant_left","room":{"name":"test-room"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(body))
	req.Header.Set("Authorization", signLiveKitWebhook(t, body, "wrong-secret"))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLivekitWebhook_BodyTamperedRejected(t *testing.T) {
	r := setupWebhookRouter(&mockSFU{}, "test-secret", nil)
	signed := `{"event":"participant_left","room":{"name":"test-room"}}`
	tampered := `{"event":"room_finished","room":{"name":"test-room"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(tampered))
	req.Header.Set("Authorization", signLiveKitWebhook(t, signed, "test-secret"))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLivekitWebhook_NoSecretConfiguredRejected(t *testing.T) {
	r := setupWebhookRouter(&mockSFU{}, "", nil)
	body := `{"event":"participant_left","room":{"name":"test-room"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(body))
	req.Header.Set("Authorization", signLiveKitWebhook(t, body, "test-secret"))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SFU_NOT_CONFIGURED {
		t.Fatalf("expected SFU_NOT_CONFIGURED, got %d", resp.Code)
	}
}

func TestLivekitWebhook_ValidSignaturePublishesJob(t *testing.T) {
	jobs := &mockLiveKitJobPublisher{}
	r := setupWebhookRouter(&mockSFU{}, "test-secret", jobs)

	body := `{"event":"participant_left","room":{"name":"test-room"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", signLiveKitWebhook(t, body, "test-secret"))
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected SUCCESS, got %d", resp.Code)
	}
	if len(jobs.published) != 1 || string(jobs.published[0]) != body {
		t.Fatalf("expected raw webhook body published, got %q", jobs.published)
	}
}

// ─── ListRooms Tests ───

func TestListRooms_Success(t *testing.T) {
	sfu := &mockSFU{
		listRoomsFn: func() ([]sfu.RoomSummary, error) {
			return []sfu.RoomSummary{
				{Name: "room-1", MemberCount: 3},
				{Name: "room-2", MemberCount: 1},
			}, nil
		},
	}
	r := setupRouter(sfu)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rooms", nil)
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected SUCCESS, got %d", resp.Code)
	}
	rooms, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("data is not an array")
	}
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(rooms))
	}
}

func TestListRooms_Empty(t *testing.T) {
	r := setupRouter(&mockSFU{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rooms", nil)
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected SUCCESS, got %d", resp.Code)
	}
}

func TestListRooms_SFUError(t *testing.T) {
	sfu := &mockSFU{
		listRoomsFn: func() ([]sfu.RoomSummary, error) {
			return nil, errors.New("livekit unavailable")
		},
	}
	r := setupRouter(sfu)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rooms", nil)
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.INTERNAL_ERROR {
		t.Fatalf("expected INTERNAL_ERROR, got %d", resp.Code)
	}
}

// ─── ListParticipants Tests ───

func TestListParticipants_Success(t *testing.T) {
	sfu := &mockSFU{
		listParticipants: func(room string) ([]sfu.ParticipantSummary, error) {
			return []sfu.ParticipantSummary{
				{Identity: "user-1"},
			}, nil
		},
	}
	r := setupRouter(sfu)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/participants?room=test-room", nil)
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected SUCCESS, got %d", resp.Code)
	}
}

func TestListParticipants_MissingRoom(t *testing.T) {
	r := setupRouter(&mockSFU{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/participants", nil)
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS, got %d", resp.Code)
	}
}

// ─── LivekitWebhook Tests ───

func TestLivekitWebhook_Success(t *testing.T) {
	r := setupWebhookRouter(&mockSFU{}, "test-secret", nil)

	body := `{"event":"participant_joined","room":{"name":"test-room"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", signLiveKitWebhook(t, body, "test-secret"))
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected SUCCESS, got %d", resp.Code)
	}
}

func TestLivekitWebhook_InvalidJSON(t *testing.T) {
	r := setupWebhookRouter(&mockSFU{}, "test-secret", nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", signLiveKitWebhook(t, "not json", "test-secret"))
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS, got %d", resp.Code)
	}
}

// ─── GetWSEndpoint Tests ───

func setupWSEndpointRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSignalHandler(service.NewSFUService(&mockSFU{}, nil))
	r.Use(func(c *gin.Context) {
		c.Set("claims", &pkg.Claims{
			Username: "user-1", DisplayName: "User 1", UserUUID: "u-1",
			Role: "user", TokenVersion: 1,
		})
		c.Next()
	})
	r.GET("/ws-endpoint", h.GetWSEndpoint)
	return r
}
