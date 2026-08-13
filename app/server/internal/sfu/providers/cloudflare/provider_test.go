package cloudflare

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
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
	if got, ok := svc.getSession("room1", "user1"); !ok || got.sessionID != "session-abc" {
		t.Fatalf("expected session stored, got %q ok=%v", got.sessionID, ok)
	}
}

func TestService_SessionDomain(t *testing.T) {
	svc := NewService(&config.Config{CFAppID: "app", CFAppSecret: "secret"})
	svc.putSession("dom-1:room-a", "alice", "session-1", 1, "uuid-alice")

	domain, ok := svc.SessionDomain("session-1")
	if !ok || domain != "dom-1" {
		t.Fatalf("SessionDomain = %q, %v; want dom-1, true", domain, ok)
	}
	if _, ok := svc.SessionDomain("unknown-session"); ok {
		t.Fatal("expected unknown session to have no domain")
	}
}

func TestService_GenerateTokenForUser_StoresOwner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/test-app/sessions/new" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(NewSessionResponse{SessionID: "session-owner"})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	svc := NewService(&config.Config{CFAppID: "test-app", CFAppSecret: "test-secret"})
	svc.client.baseURL = server.URL
	svc.client.httpClient = http.DefaultClient

	if _, err := svc.GenerateTokenForUser("room1", "user1", "uuid-owner"); err != nil {
		t.Fatalf("GenerateTokenForUser failed: %v", err)
	}
	meta, ok := svc.getSession("room1", "user1")
	if !ok || meta.ownerUUID != "uuid-owner" {
		t.Fatalf("expected owner uuid-owner in session meta, got %+v ok=%v", meta, ok)
	}
	owner, ok := svc.SessionOwner("session-owner")
	if !ok || owner != "uuid-owner" {
		t.Fatalf("expected SessionOwner uuid-owner, got %q ok=%v", owner, ok)
	}
	if _, ok := svc.SessionOwner("unknown-session"); ok {
		t.Fatal("expected unknown session to have no owner")
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
		appID:   "test-app",
		stunURL: "stun.cloudflare.com:3478",
		sessions: map[string]map[string]sessionMeta{
			"room1": {"user1": {sessionID: "sess-1"}},
			"room2": {"user2": {sessionID: "sess-2"}},
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
		sessions: map[string]map[string]sessionMeta{
			"room1": {
				"alice": {sessionID: "sess-a"},
				"bob":   {sessionID: "sess-b"},
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

func TestService_MuteParticipant_CloseTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/test-app/sessions/sess-mute" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SessionStateResponse{
				Tracks: []TrackState{
					{Location: "local", MID: "0", TrackName: "audio", Status: "active"},
					{Location: "remote", MID: "7", SessionID: "sess-other", TrackName: "audio", Status: "active"},
				},
			})
			return
		}
		if r.URL.Path == "/apps/test-app/sessions/sess-mute/tracks/close" && r.Method == http.MethodPut {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			var req CloseTrackRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("decode close tracks body: %v", err)
			}
			if !req.Force {
				t.Fatal("expected force=true for mute close tracks")
			}
			if len(req.Tracks) != 1 || req.Tracks[0].MID != "0" {
				t.Fatalf("expected only local mid 0 close, got %+v", req.Tracks)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(CloseTrackResponse{})
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
		sessions: map[string]map[string]sessionMeta{
			"room1": {"user1": {sessionID: "sess-mute"}},
		},
	}

	if err := svc.MuteParticipant("room1", "user1", "", true); err != nil {
		t.Fatalf("MuteParticipant failed: %v", err)
	}
}

func TestMuteParticipant_SkipsClosedTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/apps/test-app/sessions/sess-1" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(SessionStateResponse{Tracks: []TrackState{
				{Location: "local", MID: "m-active", Status: "active"},
				{Location: "local", MID: "m-closed", Status: "closed"},
			}})
		case r.URL.Path == "/apps/test-app/sessions/sess-1/tracks/close" && r.Method == http.MethodPut:
			var req CloseTrackRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode close request: %v", err)
			}
			if len(req.Tracks) != 1 || req.Tracks[0].MID != "m-active" {
				t.Fatalf("close specs = %+v, want only active mid", req.Tracks)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(CloseTrackResponse{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	svc := NewService(&config.Config{CFAppID: "test-app", CFAppSecret: "test-secret"})
	svc.client.baseURL = server.URL
	svc.client.httpClient = http.DefaultClient
	svc.putSession("r1", "alice", "sess-1", 1, "")

	if err := svc.MuteParticipant("r1", "alice", "", true); err != nil {
		t.Fatalf("MuteParticipant failed: %v", err)
	}
}

func TestMuteParticipant_CloseTracksError_RecheckTreatsAsSuccess(t *testing.T) {
	getCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/apps/test-app/sessions/sess-1" && r.Method == http.MethodGet:
			getCalls++
			w.Header().Set("Content-Type", "application/json")
			if getCalls == 1 {
				_ = json.NewEncoder(w).Encode(SessionStateResponse{Tracks: []TrackState{
					{Location: "local", MID: "m-active", Status: "active"},
				}})
				return
			}
			_ = json.NewEncoder(w).Encode(SessionStateResponse{Tracks: []TrackState{
				{Location: "local", MID: "m-active", Status: "closed"},
			}})
		case r.URL.Path == "/apps/test-app/sessions/sess-1/tracks/close" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	svc := NewService(&config.Config{CFAppID: "test-app", CFAppSecret: "test-secret"})
	svc.client.baseURL = server.URL
	svc.client.httpClient = http.DefaultClient
	svc.putSession("r1", "alice", "sess-1", 1, "")

	if err := svc.MuteParticipant("r1", "alice", "", true); err != nil {
		t.Fatalf("MuteParticipant should treat already-closed tracks as success, got %v", err)
	}
}

func TestService_MuteParticipant_NoLocalTracks(t *testing.T) {
	closeCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/test-app/sessions/sess-mute" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SessionStateResponse{
				Tracks: []TrackState{
					{Location: "remote", MID: "7", SessionID: "sess-other", TrackName: "audio", Status: "active"},
				},
			})
			return
		}
		if r.URL.Path == "/apps/test-app/sessions/sess-mute/tracks/close" {
			closeCalled = true
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
		sessions: map[string]map[string]sessionMeta{
			"room1": {"user1": {sessionID: "sess-mute"}},
		},
	}

	if err := svc.MuteParticipant("room1", "user1", "", true); err != nil {
		t.Fatalf("MuteParticipant failed: %v", err)
	}
	if closeCalled {
		t.Fatal("expected no CloseTracks call when session has no local tracks")
	}
}

func TestService_MuteParticipant_GetStateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/test-app/sessions/sess-mute" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusInternalServerError)
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
		sessions: map[string]map[string]sessionMeta{
			"room1": {"user1": {sessionID: "sess-mute"}},
		},
	}
	err := svc.MuteParticipant("room1", "user1", "", true)
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.SFU_ERROR {
		t.Fatalf("expected SFU_ERROR, got %v", err)
	}
}

func TestService_MuteParticipant_Unmute_SoftFallback(t *testing.T) {
	svc := &Service{}
	err := svc.MuteParticipant("room1", "user1", "", false)
	if !errors.Is(err, pkg.ErrSFUNotSupported) {
		t.Fatalf("expected ErrSFUNotSupported for unmute, got %v", err)
	}
}

func TestMuteParticipant_Unmute_ReturnsAppError(t *testing.T) {
	svc := NewService(&config.Config{CFAppID: "app", CFAppSecret: "sec"})
	err := svc.MuteParticipant("r", "alice", "", false)
	if err == nil {
		t.Fatal("unmute should return ErrSFUNotSupported")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("unmute should return *pkg.AppError, got %T", err)
	}
}

func TestService_MuteParticipant_NoSession(t *testing.T) {
	svc := &Service{
		appID: "test-app",
		client: &Client{
			appID:      "test-app",
			appSecret:  "test-secret",
			baseURL:    "http://unused",
			httpClient: http.DefaultClient,
		},
		sessions: map[string]map[string]sessionMeta{},
	}
	err := svc.MuteParticipant("room1", "ghost", "", true)
	if !errors.Is(err, pkg.ErrSFUNotSupported) {
		t.Fatalf("expected ErrSFUNotSupported for missing session, got %v", err)
	}
}

func TestMuteParticipant_LocalSessionMiss_SoftFallback(t *testing.T) {
	svc := NewService(&config.Config{CFAppID: "app", CFAppSecret: "sec"})
	err := svc.MuteParticipant("r", "ghost", "", true)
	if !errors.Is(err, pkg.ErrSFUNotSupported) {
		t.Fatalf("local miss must not fake success, err=%v", err)
	}
}

func TestService_MuteParticipant_NotConfigured(t *testing.T) {
	svc := &Service{}
	err := svc.MuteParticipant("room1", "user1", "", true)
	if err == nil {
		t.Fatal("expected configuration error")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.SFU_NOT_CONFIGURED {
		t.Fatalf("expected SFU_NOT_CONFIGURED, got %v", err)
	}
}

func TestService_MuteParticipant_CloseTracksError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/test-app/sessions/sess-mute" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SessionStateResponse{
				Tracks: []TrackState{{Location: "local", MID: "0", Status: "active"}},
			})
			return
		}
		if r.URL.Path == "/apps/test-app/sessions/sess-mute/tracks/close" && r.Method == http.MethodPut {
			w.WriteHeader(http.StatusInternalServerError)
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
		sessions: map[string]map[string]sessionMeta{
			"room1": {"user1": {sessionID: "sess-mute"}},
		},
	}
	err := svc.MuteParticipant("room1", "user1", "", true)
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.SFU_ERROR {
		t.Fatalf("expected SFU_ERROR, got %v", err)
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
		sessions: map[string]map[string]sessionMeta{
			"room1": {"user1": {sessionID: "sess-user1"}},
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
		sessions: map[string]map[string]sessionMeta{
			"room1": {
				"alice": {sessionID: "sess-1"},
				"bob":   {sessionID: "sess-2"},
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

func TestService_Capabilities(t *testing.T) {
	caps := (&Service{}).Capabilities()
	if !reflect.DeepEqual(caps, sfu.CapabilitiesFor("cloudflare")) {
		t.Fatalf("Capabilities() = %+v, want %+v", caps, sfu.CapabilitiesFor("cloudflare"))
	}
}

func TestService_RemoveParticipant_DeleteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/test-app/sessions/sess-user1" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusInternalServerError)
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
		sessions: map[string]map[string]sessionMeta{
			"room1": {"user1": {sessionID: "sess-user1"}},
		},
	}
	err := svc.RemoveParticipant("room1", "user1")
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.SFU_ERROR {
		t.Fatalf("expected SFU_ERROR, got %v", err)
	}
	if _, ok := svc.getSession("room1", "user1"); !ok {
		t.Fatal("session should remain after failed delete")
	}
}

func TestService_DeleteRoom_PartialError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		callCount++
		if r.URL.Path == "/apps/test-app/sessions/sess-1" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
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
		sessions: map[string]map[string]sessionMeta{
			"room1": {
				"alice": {sessionID: "sess-1"},
				"bob":   {sessionID: "sess-2"},
			},
		},
	}
	err := svc.DeleteRoom("room1")
	if err == nil {
		t.Fatal("expected partial failure error")
	}
	if callCount != 2 {
		t.Fatalf("expected 2 delete calls, got %d", callCount)
	}
}
