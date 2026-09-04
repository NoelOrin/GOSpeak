package cloudflare

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_CreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, r.Method, http.MethodPost)
		assertEqual(t, r.URL.Path, "/apps/test-app-id/sessions/new")
		assertEqual(t, r.Header.Get("Authorization"), "Bearer test-secret")
		assertEqual(t, r.URL.Query().Get("correlationId"), "room1")
		if r.Header.Get("Content-Type") != "" {
			t.Fatalf("expected empty content-type for bodyless create session, got %q", r.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("expected empty body, got %q", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(NewSessionResponse{SessionID: "test-session-id"})
	}))
	defer server.Close()

	client := &Client{
		appID:      "test-app-id",
		appSecret:  "test-secret",
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
	}

	resp, err := client.CreateSession(&NewSessionRequest{CorrelationID: "room1"})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if resp.SessionID != "test-session-id" {
		t.Fatalf("expected session id test-session-id, got %s", resp.SessionID)
	}
}

func TestClient_CreateSession_ThirdpartyQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, r.URL.Path, "/apps/test-app-id/sessions/new")
		assertEqual(t, r.URL.Query().Get("thirdparty"), "true")
		assertEqual(t, r.URL.Query().Get("correlationId"), "room1")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("expected empty body, got %q", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(NewSessionResponse{SessionID: "sess-tp"})
	}))
	defer server.Close()

	client := &Client{
		appID:      "test-app-id",
		appSecret:  "test-secret",
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
	}

	resp, err := client.CreateSession(&NewSessionRequest{Thirdparty: true, CorrelationID: "room1"})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if resp.SessionID != "sess-tp" {
		t.Fatalf("expected sess-tp, got %s", resp.SessionID)
	}
}

func TestClient_GetSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, r.Method, http.MethodGet)
		assertEqual(t, r.URL.Path, "/apps/test-app-id/sessions/sess-1")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SessionInfo{
			SessionID: "sess-1",
			AppID:     "test-app-id",
		})
	}))
	defer server.Close()

	client := &Client{
		appID:      "test-app-id",
		appSecret:  "test-secret",
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
	}

	info, err := client.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if info.SessionID != "sess-1" {
		t.Fatalf("expected sess-1, got %s", info.SessionID)
	}
}

func TestClient_GetSessionTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, r.Method, http.MethodGet)
		assertEqual(t, r.URL.Path, "/apps/test-app-id/sessions/sess-1")
		assertEqual(t, r.Header.Get("Authorization"), "Bearer test-secret")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SessionStateResponse{
			Tracks: []TrackState{
				{Location: "local", MID: "0", TrackName: "audio", Status: "active"},
				{Location: "remote", MID: "7", SessionID: "sess-other", TrackName: "audio", Status: "active"},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		appID:      "test-app-id",
		appSecret:  "test-secret",
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
	}

	state, err := client.GetSessionTracks("sess-1")
	if err != nil {
		t.Fatalf("GetSessionTracks failed: %v", err)
	}
	if len(state.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(state.Tracks))
	}
	if state.Tracks[0].Location != "local" || state.Tracks[0].MID != "0" {
		t.Fatalf("unexpected local track: %+v", state.Tracks[0])
	}
	if state.Tracks[1].Location != "remote" || state.Tracks[1].MID != "7" {
		t.Fatalf("unexpected remote track: %+v", state.Tracks[1])
	}
}

func TestClient_AddTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, r.Method, http.MethodPost)
		assertEqual(t, r.URL.Path, "/apps/test-app-id/sessions/sess-1/tracks/new")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TracksResponse{
			SessionDescription: &SessionDescription{Type: "answer", SDP: "v=0\r\n"},
			Tracks:             []TrackResult{{TrackName: "mic", MID: "0", Location: "local"}},
		})
	}))
	defer server.Close()

	client := &Client{
		appID:      "test-app-id",
		appSecret:  "test-secret",
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
	}

	resp, err := client.AddTracks("sess-1", &TrackRequest{
		SessionDescription: &SessionDescription{Type: "offer", SDP: "v=0\r\n"},
		Tracks:             []TrackSpec{{Location: "local", MID: "0"}},
	})
	if err != nil {
		t.Fatalf("AddTracks failed: %v", err)
	}
	if resp.SessionDescription == nil || resp.SessionDescription.Type != "answer" {
		t.Fatalf("expected answer SDP, got %+v", resp.SessionDescription)
	}
}

func TestClient_CloseTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, r.Method, http.MethodPut)
		assertEqual(t, r.URL.Path, "/apps/test-app-id/sessions/sess-1/tracks/close")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CloseTrackResponse{
			Tracks: []CloseTrackResult{{TrackName: "mic"}},
		})
	}))
	defer server.Close()

	client := &Client{
		appID:      "test-app-id",
		appSecret:  "test-secret",
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
	}

	if _, err := client.CloseTracks("sess-1", &CloseTrackRequest{TrackNames: []string{"mic"}}); err != nil {
		t.Fatalf("CloseTracks failed: %v", err)
	}
}

func assertEqual(t *testing.T, got, expected string) {
	t.Helper()
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
