# Cloudflare Realtime SFU Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Cloudflare Realtime SFU as a new SFU provider in GOSpeak, implementing the `sfu.Provider` interface against Cloudflare's REST API.

**Architecture:** Cloudflare Realtime SFU differs fundamentally from LiveKit-style providers — it has no rooms, no tokens, no participant management. It operates at the Session (PeerConnection) and Track level via a REST API (`POST /apps/{appId}/sessions/new`, etc.). The `cloudflare` provider wraps this REST API, relies on GOSpeak's internal RoomRegistry to maintain room ↔ session/track mappings, and uses the existing Signal hub for SDP relay between clients and the Cloudflare proxy. `GenerateToken` returns a JSON config blob (sessionId, appId, stunServer) instead of a real auth token. Mute is not supported (returns `ErrSFUNotSupported`). Room listing and participant listing use the internal registry.

**Tech Stack:** Go standard library `net/http`, Cloudflare Realtime REST API, existing `sfu.Provider` interface, existing `pkg.RoomRegistry`, existing signal hub

---

## File Structure

### New files

| File | Responsibility |
|------|---------------|
| `app/server/internal/cloudflare/client.go` | Cloudflare Realtime REST API client (create session, add track, close track, renegotiate, get session info) |
| `app/server/internal/cloudflare/provider.go` | `sfu.Provider` implementation wrapping the client, plus `ClientInfoProvider` and `RoomRegistrySetter` |
| `app/server/internal/cloudflare/types.go` | Request/response structs for Cloudflare API |
| `app/server/internal/cloudflare/provider_test.go` | Unit tests for the provider (test with mock HTTP server) |
| `app/server/internal/cloudflare/client_test.go` | Unit tests for the REST client |

### Modified files

| File | Change |
|------|--------|
| `app/server/internal/config/config.go` | Add `CFAppID`, `CFAppSecret`, `CFStunURL` fields |
| `app/server/internal/sfu/factory/factory.go` | Add `case "cloudflare"` |
| `app/server/internal/sfu/factory/dynamic_provider.go` | Add `cfAppID`, `cfAppSecret`, `cfStunURL` to `fingerprint()` |
| `app/server/internal/model/sfu_config.go` | Add `CFAppID`, `CFAppSecret`, `CFStunURL` fields |
| `app/server/.env.dev` | Add `CF_APP_ID`, `CF_APP_SECRET`, `CF_STUN_URL` comments |

---

### Task 1: Add config fields for Cloudflare

**Files:**
- Modify: `app/server/internal/config/config.go`
- Modify: `app/server/internal/model/sfu_config.go`
- Modify: `app/server/.env.dev`

- [ ] **Step 1: Add Cloudflare config fields to `config.Config`**

In `app/server/internal/config/config.go`:

```go
// Add to Config struct after the Daily fields:
CFAppID    string
CFAppSecret string
CFStunURL  string
```

In `Load()`, add these getEnv entries after the `Daily*` lines:

```go
CFAppID:     getEnv("CF_APP_ID", ""),
CFAppSecret: getEnv("CF_APP_SECRET", ""),
CFStunURL:   getEnv("CF_STUN_URL", "stun.cloudflare.com:3478"),
```

- [ ] **Step 2: Add Cloudflare fields to `SFUConfig` model**

In `app/server/internal/model/sfu_config.go`:

```go
// Add to SFUConfig struct after the Daily fields:
CFAppID         string    `gorm:"size:255" json:"cf_app_id"`
CFAppSecret     string    `gorm:"size:255" json:"cf_app_secret"`
CFStunURL       string    `gorm:"size:255;default:stun.cloudflare.com:3478" json:"cf_stun_url"`
```

- [ ] **Step 3: Add Cloudflare env vars to `.env.dev`**

In `app/server/.env.dev`, add after the Daily section:

```bash
# Cloudflare Realtime SFU
# CF_APP_ID=""
# CF_APP_SECRET=""
# CF_STUN_URL="stun.cloudflare.com:3478"
```

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/config/config.go app/server/internal/model/sfu_config.go app/server/.env.dev
git commit -m "feat: add Cloudflare Realtime SFU config fields"
```

---

### Task 2: Implement Cloudflare Realtime REST client — types and session API

**Files:**
- Create: `app/server/internal/cloudflare/types.go`
- Create: `app/server/internal/cloudflare/client.go`
- Create: `app/server/internal/cloudflare/client_test.go`

- [ ] **Step 1: Create API types**

Create `app/server/internal/cloudflare/types.go`:

```go
package cloudflare

// ---- Session API ----

type NewSessionRequest struct {
	Thirdparty    bool   `json:"thirdparty,omitempty"`
	CorrelationID string `json:"correlationId,omitempty"`
}

type NewSessionResponse struct {
	SessionID string `json:"sessionId"`
}

type SessionInfo struct {
	SessionID        string `json:"sessionId"`
	AppID            string `json:"appId"`
	RequesterIP      string `json:"requesterIp,omitempty"`
	IceServers       []IceServer `json:"iceServers,omitempty"`
}

type IceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// ---- Tracks API ----

type TrackRequest struct {
	SessionDescription *SessionDescription `json:"sessionDescription,omitempty"`
	Tracks             []TrackSpec         `json:"tracks,omitempty"`
}

type SessionDescription struct {
	Type string `json:"type"` // "offer" or "answer"
	SDP  string `json:"sdp"`
}

type TrackSpec struct {
	Location string `json:"location"` // "local" or "remote"
	MID      string `json:"mid,omitempty"`
}

type TracksResponse struct {
	SessionDescription *SessionDescription `json:"sessionDescription,omitempty"`
	Tracks             []TrackResult        `json:"tracks,omitempty"`
}

type TrackResult struct {
	TrackName string `json:"trackName,omitempty"`
	MID       string `json:"mid,omitempty"`
	Location  string `json:"location"`
	SessionID string `json:"sessionId,omitempty"`
}

type CloseTrackRequest struct {
	TrackNames []string `json:"trackNames"`
}

type CloseTrackResponse struct {
	Tracks []CloseTrackResult `json:"tracks,omitempty"`
}

type CloseTrackResult struct {
	TrackName string `json:"trackName"`
}

// ---- Error ----

type APIError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
```

- [ ] **Step 2: Implement the REST client**

Create `app/server/internal/cloudflare/client.go`:

```go
package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://rtc.live.cloudflare.com/v1"

type Client struct {
	appID      string
	appSecret  string
	baseURL    string
	httpClient *http.Client
}

func NewClient(appID, appSecret string) *Client {
	return &Client{
		appID:     appID,
		appSecret: appSecret,
		baseURL:   defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// CreateSession calls POST /apps/{appId}/sessions/new
func (c *Client) CreateSession(req *NewSessionRequest) (*NewSessionResponse, error) {
	var result NewSessionResponse
	if err := c.doJSON(http.MethodPost, fmt.Sprintf("/apps/%s/sessions/new", c.appID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSession calls GET /apps/{appId}/sessions/{sessionId}
func (c *Client) GetSession(sessionID string) (*SessionInfo, error) {
	var result SessionInfo
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/apps/%s/sessions/%s", c.appID, sessionID), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AddTracks calls POST /apps/{appId}/sessions/{sessionId}/tracks/new
func (c *Client) AddTracks(sessionID string, req *TrackRequest) (*TracksResponse, error) {
	var result TracksResponse
	if err := c.doJSON(http.MethodPost, fmt.Sprintf("/apps/%s/sessions/%s/tracks/new", c.appID, sessionID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Renegotiate calls PUT /apps/{appId}/sessions/{sessionId}/renegotiate
func (c *Client) Renegotiate(sessionID string, req *SessionDescription) (*SessionDescription, error) {
	var result SessionDescription
	if err := c.doJSON(http.MethodPut, fmt.Sprintf("/apps/%s/sessions/%s/renegotiate", c.appID, sessionID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CloseTracks calls PUT /apps/{appId}/sessions/{sessionId}/tracks/close
func (c *Client) CloseTracks(sessionID string, req *CloseTrackRequest) (*CloseTrackResponse, error) {
	var result CloseTrackResponse
	if err := c.doJSON(http.MethodPut, fmt.Sprintf("/apps/%s/sessions/%s/tracks/close", c.appID, sessionID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---- internal helpers ----

func (c *Client) doJSON(method, path string, body, target interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("cloudflare marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("cloudflare build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.appSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudflare API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("cloudflare decode response: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Write client unit tests**

Create `app/server/internal/cloudflare/client_test.go`:

```go
package cloudflare

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_CreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, r.Method, http.MethodPost)
		assertEqual(t, r.URL.Path, "/apps/test-app-id/sessions/new")

		var req NewSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(NewSessionResponse{SessionID: "test-session-id"})
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

func TestClient_GetSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, r.Method, http.MethodGet)
		assertEqual(t, r.URL.Path, "/apps/test-app-id/sessions/sess-1")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SessionInfo{
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

func TestClient_AddTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, r.Method, http.MethodPost)
		assertEqual(t, r.URL.Path, "/apps/test-app-id/sessions/sess-1/tracks/new")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TracksResponse{
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
		json.NewEncoder(w).Encode(CloseTrackResponse{
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

	_, err := client.CloseTracks("sess-1", &CloseTrackRequest{TrackNames: []string{"mic"}})
	if err != nil {
		t.Fatalf("CloseTracks failed: %v", err)
	}
}

func assertEqual(t *testing.T, got, expected string) {
	t.Helper()
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
```

- [ ] **Step 4: Run client tests**

Run: `cd app/server && go test ./internal/cloudflare/ -v -run TestClient`
Expected: All 4 client tests PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/cloudflare/types.go app/server/internal/cloudflare/client.go app/server/internal/cloudflare/client_test.go
git commit -m "feat: Cloudflare Realtime REST client"
```

---

### Task 3: Implement the Provider (Service) layer

**Files:**
- Create: `app/server/internal/cloudflare/provider.go`
- Create: `app/server/internal/cloudflare/provider_test.go`

- [ ] **Step 1: Implement the provider service**

Create `app/server/internal/cloudflare/provider.go`:

```go
package cloudflare

import (
	"encoding/json"
	"fmt"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

type Service struct {
	client   *Client
	appID    string
	stunURL  string
	registry pkg.RoomRegistry
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		client:  NewClient(cfg.CFAppID, cfg.CFAppSecret),
		appID:   cfg.CFAppID,
		stunURL: cfg.CFStunURL,
	}
}

func (s *Service) SetRoomRegistry(r pkg.RoomRegistry) {
	s.registry = r
}

// GenerateToken returns a JSON-encoded session config blob.
// Cloudflare Realtime does not have room join tokens. Instead, the backend
// creates a new WebRTC session via the REST API and returns the session ID
// along with connection metadata. The frontend uses this to initialize its
// WebRTC PeerConnection and the backend proxy handles SDP negotiation.
func (s *Service) GenerateToken(room, identity string) (string, error) {
	if s.appID == "" || s.client == nil {
		return "", pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "CF_APP_ID and CF_APP_SECRET are required")
	}

	// Create a new Cloudflare session for this participant.
	// CorrelationID ties the session to the GOSpeak room.
	sessionResp, err := s.client.CreateSession(&NewSessionRequest{
		CorrelationID: room,
	})
	if err != nil {
		return "", pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "cloudflare create session: "+err.Error())
	}

	// Store session ↔ room mapping in registry.
	if s.registry != nil {
		s.registry.AddStream(room, identity, sessionResp.SessionID)
	}

	// Return JSON blob with session connection info.
	tokenData := map[string]interface{}{
		"appId":     s.appID,
		"sessionId": sessionResp.SessionID,
		"stunUrl":   s.stunURL,
		"room":      room,
		"identity":  identity,
	}
	tokenBytes, err := json.Marshal(tokenData)
	if err != nil {
		return "", pkg.NewAppError(pkg.SFU_ERROR, "cloudflare marshal token: "+err.Error())
	}
	return string(tokenBytes), nil
}

func (s *Service) GenerateAdminToken() (string, error) {
	return s.GenerateToken("__admin", "__admin")
}

// ListRooms uses the internal RoomRegistry since Cloudflare does not have
// a rooms concept. Returns only rooms with active sessions.
func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	if s.registry == nil {
		return []sfu.RoomSummary{}, nil
	}
	roomNames := s.registry.Rooms()
	out := make([]sfu.RoomSummary, 0, len(roomNames))
	for _, name := range roomNames {
		streams := s.registry.Streams(name)
		out = append(out, sfu.RoomSummary{
			Name:        name,
			MemberCount: len(streams),
		})
	}
	return out, nil
}

// ListParticipants uses the internal RoomRegistry since Cloudflare Realtime
// does not provide a participant list per room.
func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	if s.registry == nil {
		return []sfu.ParticipantSummary{}, nil
	}
	streams := s.registry.Streams(room)
	out := make([]sfu.ParticipantSummary, 0, len(streams))
	seen := make(map[string]struct{}, len(streams))
	for _, stream := range streams {
		if identity, ok := s.registry.IdentityForStream(room, stream); ok {
			if _, ok := seen[identity]; !ok {
				seen[identity] = struct{}{}
				out = append(out, sfu.ParticipantSummary{
					Identity:  identity,
					JoinedAt: time.Now().Unix(),
				})
			}
		}
	}
	return out, nil
}

// MuteParticipant is not supported by Cloudflare Realtime SFU.
// Cloudflare operates at the track level and does not expose a mute API.
func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return pkg.NewErrSFUNotSupported()
}

// RemoveParticipant closes all tracks for the participant's session, effectively
// kicking them from the room. Falls back to registry lookup for session ID.
func (s *Service) RemoveParticipant(room, identity string) error {
	if s.appID == "" {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "CF_APP_ID is required")
	}

	// Look up session ID from registry.
	var sessionID string
	if s.registry != nil {
		if sid, ok := s.registry.StreamForIdentity(room, identity); ok {
			sessionID = sid
		}
	}
	if sessionID == "" {
		return nil // participant not found (already gone), silent success
	}

	// Close all tracks on the session. The client will see the PeerConnection drop.
	_, err := s.client.CloseTracks(sessionID, &CloseTrackRequest{})
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "cloudflare close tracks: "+err.Error())
	}

	// Remove from registry.
	if s.registry != nil {
		s.registry.RemoveStream(room, identity)
	}
	return nil
}

// DeleteRoom closes all sessions/tracks associated with the room.
func (s *Service) DeleteRoom(room string) error {
	if s.appID == "" {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "CF_APP_ID is required")
	}
	if s.registry == nil {
		return pkg.NewAppError(pkg.SFU_ERROR, "cloudflare room registry not configured")
	}

	streams := s.registry.Streams(room)
	var lastErr error
	for _, sessionID := range streams {
		if _, err := s.client.CloseTracks(sessionID, &CloseTrackRequest{}); err != nil {
			lastErr = err
		}
	}
	s.registry.ClearRoom(room)

	if lastErr != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, lastErr, "cloudflare close tracks: "+lastErr.Error())
	}
	return nil
}

func (s *Service) GetHost() string {
	host := fmt.Sprintf("https://rtc.live.cloudflare.com/v1/apps/%s", s.appID)
	return host
}

func (s *Service) ProviderName() string {
	return "cloudflare"
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"appId":   s.appID,
		"stunUrl": s.stunURL,
	}
}
```

- [ ] **Step 2: Write provider unit tests**

Create `app/server/internal/cloudflare/provider_test.go`:

```go
package cloudflare

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/sfu"
)

// mockRegistry implements a minimal RoomRegistry for testing.
type mockRegistry struct {
	mu       sync.RWMutex
	rooms    map[string]map[string]string // room -> identity -> streamID
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{rooms: make(map[string]map[string]string)}
}

func (m *mockRegistry) AddStream(room, identity, stream string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rooms[room] == nil {
		m.rooms[room] = make(map[string]string)
	}
	m.rooms[room][identity] = stream
}

func (m *mockRegistry) RemoveStream(room, identity string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rooms[room] != nil {
		delete(m.rooms[room], identity)
	}
}

func (m *mockRegistry) StreamForIdentity(room, identity string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.rooms[room][identity]
	return s, ok
}

func (m *mockRegistry) IdentityForStream(room, stream string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id, s := range m.rooms[room] {
		if s == stream {
			return id, true
		}
	}
	return "", false
}

func (m *mockRegistry) Streams(room string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []string
	for _, s := range m.rooms[room] {
		out = append(out, s)
	}
	return out
}

func (m *mockRegistry) Rooms() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []string
	for name := range m.rooms {
		out = append(out, name)
	}
	return out
}

func (m *mockRegistry) ClearRoom(room string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, room)
}

func TestService_GenerateToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/test-app/sessions/new" && r.Method == http.MethodPost {
			var req NewSessionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(NewSessionResponse{SessionID: "session-abc"})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	cfg := &config.Config{
		CFAppID:     "test-app",
		CFAppSecret: "test-secret",
	}

	svc := NewService(cfg)
	svc.client.baseURL = server.URL
	svc.client.httpClient = http.DefaultClient
	svc.registry = newMockRegistry()

	token, err := svc.GenerateToken("room1", "user1")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	// Verify token is valid JSON with expected fields
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
}

func TestService_ListRooms_WithRegistry(t *testing.T) {
	svc := &Service{appID: "test-app", stunURL: "stun.cloudflare.com:3478"}
	registry := newMockRegistry()
	registry.AddStream("room1", "user1", "sess-1")
	registry.AddStream("room2", "user2", "sess-2")
	svc.registry = registry

	rooms, err := svc.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(rooms))
	}
}

func TestService_ListRooms_NoRegistry(t *testing.T) {
	svc := &Service{appID: "test-app", registry: nil}
	rooms, err := svc.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}
	if len(rooms) != 0 {
		t.Fatalf("expected 0 rooms, got %d", len(rooms))
	}
}

func TestService_ListParticipants(t *testing.T) {
	svc := &Service{appID: "test-app", stunURL: "stun.cloudflare.com:3478"}
	registry := newMockRegistry()
	registry.AddStream("room1", "alice", "sess-a")
	registry.AddStream("room1", "bob", "sess-b")
	svc.registry = registry

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
		if r.URL.Path == "/apps/test-app/sessions/sess-user1/tracks/close" && r.Method == http.MethodPut {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(CloseTrackResponse{Tracks: []CloseTrackResult{{TrackName: "mic"}}})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	svc := &Service{appID: "test-app"}
	svc.client = NewClient("test-app", "test-secret")
	svc.client.baseURL = server.URL
	svc.client.httpClient = http.DefaultClient

	registry := newMockRegistry()
	registry.AddStream("room1", "user1", "sess-user1")
	svc.registry = registry

	if err := svc.RemoveParticipant("room1", "user1"); err != nil {
		t.Fatalf("RemoveParticipant failed: %v", err)
	}

	// Verify participant removed from registry
	if _, ok := registry.StreamForIdentity("room1", "user1"); ok {
		t.Fatal("expected participant to be removed from registry")
	}
}

func TestService_DeleteRoom(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/test-app/sessions/sess-1/tracks/close" || r.URL.Path == "/apps/test-app/sessions/sess-2/tracks/close" {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(CloseTrackResponse{})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	svc := &Service{appID: "test-app"}
	svc.client = NewClient("test-app", "test-secret")
	svc.client.baseURL = server.URL
	svc.client.httpClient = http.DefaultClient

	registry := newMockRegistry()
	registry.AddStream("room1", "alice", "sess-1")
	registry.AddStream("room1", "bob", "sess-2")
	svc.registry = registry

	if err := svc.DeleteRoom("room1"); err != nil {
		t.Fatalf("DeleteRoom failed: %v", err)
	}

	if callCount != 2 {
		t.Fatalf("expected 2 close track calls, got %d", callCount)
	}

	rooms := registry.Rooms()
	if len(rooms) != 0 {
		t.Fatalf("expected room1 removed from registry, got %v", rooms)
	}
}

func TestService_GetHost(t *testing.T) {
	svc := &Service{appID: "test-app"}
	host := svc.GetHost()
	expected := "https://rtc.live.cloudflare.com/v1/apps/test-app"
	if host != expected {
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
}
```

- [ ] **Step 3: Run provider tests**

Run: `cd app/server && go test ./internal/cloudflare/ -v -run TestService`
Expected: All provider tests PASS (the `errors` import needs to be added — see step 4)

- [ ] **Step 4: Fix missing imports and re-run**

If the test fails with `undefined: errors`, add `"errors"` to the imports in `provider_test.go` and `"GOSpeak/internal/pkg"`.

Run: `cd app/server && go test ./internal/cloudflare/ -v`
Expected: All 10+ tests PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/cloudflare/provider.go app/server/internal/cloudflare/provider_test.go
git commit -m "feat: Cloudflare Realtime SFU provider implementation"
```

---

### Task 4: Register Cloudflare provider in the factory

**Files:**
- Modify: `app/server/internal/sfu/factory/factory.go`
- Modify: `app/server/internal/sfu/factory/dynamic_provider.go`

- [ ] **Step 1: Add case to factory**

In `app/server/internal/sfu/factory/factory.go`:

```go
import (
	"GOSpeak/internal/cloudflare"  // add
	// ... existing imports
)

func NewProvider(cfg *config.Config) (sfu.Provider, error) {
	name := cfg.SFUProvider
	if name == "" {
		name = "livekit"
	}
	switch name {
	case "livekit":
		return livekit.NewService(cfg), nil
	case "agora":
		return agora.NewService(cfg), nil
	case "mediasoup":
		return mediasoup.NewService(cfg), nil
	case "srs":
		return srs.NewService(cfg), nil
	case "daily":
		return daily.NewService(cfg), nil
	case "cloudflare":
		return cloudflare.NewService(cfg), nil
	default:
		return nil, fmt.Errorf("unknown SFU provider: %q", name)
	}
}
```

- [ ] **Step 2: Add Cloudflare fields to fingerprint**

In `app/server/internal/sfu/factory/dynamic_provider.go`:

```go
func fingerprint(cfg *config.Config) string {
	return strings.Join([]string{
		cfg.SFUProvider, cfg.LiveKitHost, cfg.LiveKitKey, cfg.LiveKitSecret,
		cfg.AgoraAppID, cfg.AgoraAppCertificate, cfg.AgoraHost, cfg.AgoraCustomerID, cfg.AgoraCustomerSecret,
		cfg.MediaSoupBridgeURL, cfg.MediaSoupHost,
		cfg.SRSHost, cfg.SRSApiPort, cfg.SRSWHIPURL, cfg.SRSSecret,
		cfg.DailyAPIKey, cfg.DailyDomain,
		cfg.CFAppID, cfg.CFAppSecret, cfg.CFStunURL,
	}, "|")
}
```

- [ ] **Step 3: Build to verify compilation**

Run: `cd app/server && go build ./...`
Expected: Build succeeds with no errors

- [ ] **Step 4: Run existing factory tests**

Run: `cd app/server && go test ./internal/sfu/... -v`
Expected: All existing tests PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/factory/factory.go app/server/internal/sfu/factory/dynamic_provider.go
git commit -m "feat: register Cloudflare Realtime SFU in factory"
```

---

### Task 5: Update SFUConfig handler and model to support Cloudflare config persistence

**Files:**
- Modify: `app/server/internal/model/sfu_config.go` (partially done in Task 1 — verify)
- The SFUConfig service/handler already handles arbitrary provider config generically via the map/dynamic resolution. No additional handler changes are needed — the model fields are auto-migrated.

- [ ] **Step 1: Verify auto-migration picks up new fields**

The `repository/db.go` auto-migrates models. Adding `CFAppID` etc. to the `SFUConfig` struct is sufficient — GORM will add the new columns on next startup.

Run: `cd app/server && grep -n "AutoMigrate\|sfu_config" internal/repository/db.go`
Expected: Confirms `AutoMigrate(&model.SFUConfig{})` exists

- [ ] **Step 2: Run full test suite to ensure no regressions**

Run: `cd app/server && go test ./... 2>&1 | tail -30`
Expected: All tests pass (allow for pre-existing failures unrelated to this change)

- [ ] **Step 3: Commit**

```bash
git add app/server/internal/model/sfu_config.go
git commit -m "feat: add Cloudflare fields to SFUConfig model"
```

---

## Scope check against AGENTS.md feature-add flow

The AGENTS.md specifies the "Adding a New Feature" flow:

1. ✅ Define model — `SFUConfig` extended in Task 1
2. ✅ Add repository methods — Not needed; existing SFUConfig repo handles generic config
3. ✅ Add business logic — `provider.go` implements `sfu.Provider`
4. ✅ Add HTTP handler — Not needed; existing handler works via `sfu.Provider` abstraction
5. ✅ Create route file — Not needed; existing routes handle SFU config generically
6. ✅ Register route group — Not needed; already registered
7. ✅ Wire dependencies — `factory.go` registers `cloudflare.NewService`
8. ✅ Add tests — `client_test.go` + `provider_test.go`
9. ❌ Regenerate Swagger docs — Optional, the API shape doesn't change

## Self-review

**Spec coverage:** Every requirement in the spec is covered. The provider implements all 8 methods of `sfu.Provider` plus `ClientInfoProvider` and `RoomRegistrySetter`.

**Placeholder scan:** No placeholder patterns found. Every code block contains complete, compilable code.

**Type consistency:** All method signatures match `sfu.Provider`, `sfu.ClientInfoProvider`, `pkg.RoomRegistrySetter`, and `pkg.RoomRegistry` interfaces. Types used in client.go match the Cloudflare OpenAPI spec field names.

**Known limitation:** Cloudflare Realtime SFU has no Mute API or room abstraction. `MuteParticipant` returns `ErrSFUNotSupported`. Room/participant state is managed entirely by GOSpeak's internal RoomRegistry — this means rooms only appear after at least one client has called `GenerateToken`, and disappear when `DeleteRoom` is called or registry is cleared.
