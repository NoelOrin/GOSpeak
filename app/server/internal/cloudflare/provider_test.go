package cloudflare

import (
	"encoding/json"
	"io"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

func TestService_GenerateToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/test-app/sessions/new" && r.Method == http.MethodPost {
			if got := r.URL.Query().Get("correlationId"); got != "room1" {
				t.Fatalf("expected correlationId=room1, got %q", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if len(body) != 0 {
				t.Fatalf("expected empty body for create session, got %q", string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(NewSessionResponse{SessionID: "session-abc"})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	cfg := &config.Config{
		CFAppID:     "test-app",
		CFAppSecret: "test-secret",
		CFStunURL:   "stun.cloudflare.com:3478",
	}
	svc := NewService(cfg)
	svc.client.baseURL = server.URL
	svc.client.httpClient = http.DefaultClient

	token, err := svc.GenerateToken("room1", "user1")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(token), &data); err != nil {
		t.Fatalf("token is not valid JSON: %v", err)
	}
	if data["sessionId"] != "session-abc" {
		t.Fatalf("expected session-abc, got %v", data["sessionId"])
	}
	if data["room"] != "room1" {
		t.Fatalf("expected room1, got %v", data["room"])
	}
	if data["appId"] != "test-app" {
		t.Fatalf("expected appId test-app, got %v", data["appId"])
	}
	if got, ok := svc.getSession("room1", "user1"); !ok || got != "session-abc" {
		t.Fatalf("expected session stored, got %q ok=%v", got, ok)
	}
}

func TestService_GenerateToken_NotConfigured(t *testing.T) {
	svc := NewService(&config.Config{})
	if _, err := svc.GenerateToken("room1", "user1"); err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestService_ListRooms(t *testing.T) {
	svc := &Service{
		appID:    "test-app",
		stunURL:  "stun.cloudflare.com:3478",
		sessions: map[string]map[string]string{
			"room1": {"user1": "sess-1"},
			"room2": {"user2": "sess-2"},
		},
	}

	rooms, err := svc.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(rooms))
	}
}

func TestService_ListParticipants(t *testing.T) {
	svc := &Service{
		appID: "test-app",
		sessions: map[string]map[string]string{
			"room1": {
				"alice": "sess-a",
				"bob":   "sess-b",
			},
		},
	}

	participants, err := svc.ListParticipants("room1")
	if err != nil {
		t.Fatalf("ListParticipants failed: %v", err)
	}
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}
}

func TestService_MuteParticipant_NotSupported(t *testing.T) {
	svc := &Service{}
	err := svc.MuteParticipant("room1", "user1", "track1", true)
	if err == nil {
		t.Fatal("expected error for mute not supported")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
}

func TestService_RemoveParticipant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/test-app/sessions/sess-user1" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	svc := &Service{
		appID: "test-app",
		client: &Client{
			appID:      "test-app",
			appSecret:  "test-secret",
			baseURL:    server.URL,
			httpClient: http.DefaultClient,
		},
		sessions: map[string]map[string]string{
			"room1": {"user1": "sess-user1"},
		},
	}

	if err := svc.RemoveParticipant("room1", "user1"); err != nil {
		t.Fatalf("RemoveParticipant failed: %v", err)
	}
	if _, ok := svc.getSession("room1", "user1"); ok {
		t.Fatal("expected participant to be removed")
	}
}

func TestService_DeleteRoom(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.URL.Path == "/apps/test-app/sessions/sess-1" || r.URL.Path == "/apps/test-app/sessions/sess-2") && r.Method == http.MethodDelete {
			callCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	svc := &Service{
		appID: "test-app",
		client: &Client{
			appID:      "test-app",
			appSecret:  "test-secret",
			baseURL:    server.URL,
			httpClient: http.DefaultClient,
		},
		sessions: map[string]map[string]string{
			"room1": {
				"alice": "sess-1",
				"bob":   "sess-2",
			},
		},
	}

	if err := svc.DeleteRoom("room1"); err != nil {
		t.Fatalf("DeleteRoom failed: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 close track calls, got %d", callCount)
	}
	rooms, err := svc.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}
	if len(rooms) != 0 {
		t.Fatalf("expected room1 removed, got %v", rooms)
	}
}

func TestService_GetHost(t *testing.T) {
	svc := &Service{appID: "test-app"}
	expected := "https://rtc.live.cloudflare.com/v1/apps/test-app"
	if host := svc.GetHost(); host != expected {
		t.Fatalf("expected %q, got %q", expected, host)
	}
}

func TestService_ProviderName(t *testing.T) {
	svc := &Service{}
	if name := svc.ProviderName(); name != "cloudflare" {
		t.Fatalf("expected cloudflare, got %s", name)
	}
}

func TestService_ClientInfo(t *testing.T) {
	svc := &Service{appID: "test-app", stunURL: "stun.cloudflare.com:3478"}
	info := svc.ClientInfo()
	if info["appId"] != "test-app" {
		t.Fatalf("expected test-app, got %v", info["appId"])
	}
	if info["stunUrl"] != "stun.cloudflare.com:3478" {
		t.Fatalf("expected stun url, got %v", info["stunUrl"])
	}
}
