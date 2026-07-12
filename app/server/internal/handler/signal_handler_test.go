package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
	"GOSpeak/internal/sfu"

	"github.com/gin-gonic/gin"
)

// ─── Mock SFU Provider ───

type mockSFU struct {
	tokenFn          func(room, identity string) (string, error)
	listRoomsFn      func() ([]sfu.RoomSummary, error)
	listParticipants func(room string) ([]sfu.ParticipantSummary, error)
	host             string
}

func (m *mockSFU) GenerateToken(room, identity string) (string, error) {
	if m.tokenFn != nil {
		return m.tokenFn(room, identity)
	}
	return "mock-token", nil
}
func (m *mockSFU) GenerateAdminToken() (string, error) { return "admin-token", nil }
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
	if m.host != "" {
		return m.host
	}
	return "wss://test.livekit.cloud"
}

// ─── Helpers ───

func setupRouter(sfu *mockSFU) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSignalHandler(service.NewSFUService(sfu, nil))
	r.POST("/token", h.GetJoinToken)
	r.POST("/signal", h.Signal)
	r.GET("/rooms", h.ListRooms)
	r.GET("/participants", h.ListParticipants)
	r.POST("/webhook", h.LivekitWebhook)
	return r
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
	if data["serverUrl"] != "wss://test.livekit.cloud" {
		t.Errorf("expected serverUrl, got %v", data["serverUrl"])
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

	body := `{"room":"test-room"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS (2001), got %d", resp.Code)
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

// ─── Signal Tests ───

func TestSignal_Success(t *testing.T) {
	r := setupRouter(&mockSFU{})

	body := `{"type":"offer","room":"test-room","identity":"user-1","data":{"sdp":"v=0..."}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/signal", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected SUCCESS, got %d", resp.Code)
	}
	data, _ := resp.Data.(map[string]interface{})
	if data["type"] != "offer" {
		t.Errorf("expected type 'offer', got %v", data["type"])
	}
}

func TestSignal_MissingType(t *testing.T) {
	r := setupRouter(&mockSFU{})

	body := `{"room":"test-room"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/signal", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS, got %d", resp.Code)
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
	r := setupRouter(&mockSFU{})

	body := `{"event":"participant_joined","room":{"name":"test-room"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected SUCCESS, got %d", resp.Code)
	}
}

func TestLivekitWebhook_InvalidJSON(t *testing.T) {
	r := setupRouter(&mockSFU{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS, got %d", resp.Code)
	}
}
