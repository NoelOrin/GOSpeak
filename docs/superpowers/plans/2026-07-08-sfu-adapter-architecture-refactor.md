# SFU Adapter Architecture Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Tighten the SFU adapter abstraction across backend and frontend so provider contracts are strongly typed, method semantics are unambiguous, and the interface surface is minimal.

**Architecture:** The backend `sfu.Provider` interface gains typed return structs (`RoomSummary`, `ParticipantSummary`), loses `MuteRoomParticipant` (merged into `MuteParticipant` with empty `trackSid`), and gains two optional extension interfaces (`StreamProvider`, `ClientInfoProvider`) replacing `interface{}` assertions. The frontend `SFUClient` interface gains `JoinParams` (replacing positional args), `isConnected()`, and a unified `async destroy()`.

**Tech Stack:** Go 1.22+ (Gin, GORM, go-socket.io), TypeScript 5+ (SolidJS, Vite), pnpm monorepo. Tests: Go `testing` + `httptest`, Vitest for `sfu-client`.

---

## File Structure

### Backend (Go)

| File | Action | Responsibility |
|------|--------|----------------|
| `app/server/internal/sfu/types.go` | Create | `RoomSummary`, `ParticipantSummary` shared structs |
| `app/server/internal/sfu/provider.go` | Modify | Update interface: typed returns, remove `MuteRoomParticipant`, add `StreamProvider` + `ClientInfoProvider` |
| `app/server/internal/sfu/dynamic_provider.go` | Modify | Update return types, remove `MuteRoomParticipant`, replace `interface{}` assertions with typed interface assertions |
| `app/server/internal/livekit/client.go` | Modify | Map to typed returns, remove `MuteRoomParticipant`, merge mute-all into `MuteParticipant` |
| `app/server/internal/agora/provider.go` | Modify | Map to typed returns, remove `MuteRoomParticipant` |
| `app/server/internal/srs/provider.go` | Modify | Map to typed returns, remove `MuteRoomParticipant` |
| `app/server/internal/daily/provider.go` | Modify | Map to typed returns, remove `MuteRoomParticipant` |
| `app/server/internal/mediasoup/provider.go` | Modify | Map to typed returns, remove `MuteRoomParticipant`, merge mute-all into `MuteParticipant` |
| `app/server/internal/service/sfu_service.go` | Modify | Update return types, replace `interface{}` assertions |
| `app/server/internal/handler/signal_handler_test.go` | Modify | Update mock + tests for new interface |

### Frontend (TypeScript)

| File | Action | Responsibility |
|------|--------|----------------|
| `packages/sfu-client/src/types.ts` | Modify | Add `JoinParams`, `isConnected()`, `async destroy()` |
| `packages/sfu-client/src/livekit-client.ts` | Modify | Accept `JoinParams`, add `isConnected()`, `async destroy()` |
| `packages/sfu-client/src/agora-client.ts` | Modify | Accept `JoinParams`, add `isConnected()`, `async destroy()` |
| `packages/sfu-client/src/srs-client.ts` | Modify | Accept `JoinParams`, add `isConnected()`, `async destroy()` |
| `packages/sfu-client/src/daily-client.ts` | Modify | Accept `JoinParams`, add `isConnected()`, `async destroy()` |
| `packages/sfu-client/src/mediasoup-client.ts` | Modify | Accept `JoinParams`, add `isConnected()`, `async destroy()` |
| `app/web/src/components/room/hooks/useRoomJoinSession.ts` | Modify | Build `JoinParams` object, pass to `joinRoom()` |

---

## Phase 1 (P0): Backend interface contract

### Task 1: Create `sfu/types.go` with shared structs

**Files:**
- Create: `app/server/internal/sfu/types.go`

- [x] **Step 1: Create the file**

```go
package sfu

// RoomSummary is the provider-agnostic room listing entry.
type RoomSummary struct {
	Name        string `json:"name"`
	MemberCount int    `json:"memberCount,omitempty"`
}

// ParticipantSummary is the provider-agnostic participant entry.
type ParticipantSummary struct {
	Identity string `json:"identity"`
	JoinedAt int64  `json:"joinedAt,omitempty"`
}
```

- [x] **Step 2: Verify it compiles**

Run: `cd app/server && go build ./internal/sfu/`
Expected: compiles with no errors

- [x] **Step 3: Commit**

```bash
git add app/server/internal/sfu/types.go
git commit -m "feat(sfu): add RoomSummary and ParticipantSummary shared types"
```

---

### Task 2: Update `sfu/provider.go` interface

**Files:**
- Modify: `app/server/internal/sfu/provider.go`

- [x] **Step 1: Replace the entire file**

```go
package sfu

// Provider abstracts an SFU backend (LiveKit, SRS, Agora, MediaSoup, Daily, etc.).
type Provider interface {
	GenerateToken(room, identity string) (string, error)
	GenerateAdminToken() (string, error)
	ListRooms() ([]RoomSummary, error)
	ListParticipants(room string) ([]ParticipantSummary, error)
	MuteParticipant(room, identity, trackSid string, muted bool) error
	RemoveParticipant(room, identity string) error
	DeleteRoom(room string) error
	GetHost() string
}

// StreamProvider extends Provider for backends that use stream-based
// addressing (e.g. SRS WHIP/WHEP). Callers check via type assertion.
type StreamProvider interface {
	Provider
	StreamName(room, identity string) string
	StreamInfo(room, identity string) (stream, token string, err error)
}

// ClientInfoProvider extends Provider for backends that expose
// provider-specific connection metadata to the frontend.
type ClientInfoProvider interface {
	Provider
	ClientInfo() map[string]interface{}
}
```

- [x] **Step 2: Verify compilation fails (expected — providers not updated yet)**

Run: `cd app/server && go build ./internal/sfu/`
Expected: compile errors in downstream provider packages

- [x] **Step 3: Commit**

```bash
git add app/server/internal/sfu/provider.go
git commit -m "refactor(sfu): typed returns, remove MuteRoomParticipant, add StreamProvider+ClientInfoProvider"
```

---

### Task 3: Update `sfu/dynamic_provider.go`

**Files:**
- Modify: `app/server/internal/sfu/dynamic_provider.go`

- [x] **Step 1: Replace the full file**

```go
package sfu

import (
	"strings"
	"sync"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

type ConfigResolver func() (*config.Config, error)

type DynamicProvider struct {
	resolve           ConfigResolver
	mu                sync.RWMutex
	roomRegistry      pkg.RoomRegistry
	cachedFingerprint string
	cachedProvider    Provider
}

func NewDynamicProvider(resolve ConfigResolver) *DynamicProvider {
	return &DynamicProvider{resolve: resolve}
}

func fingerprint(cfg *config.Config) string {
	return strings.Join([]string{
		cfg.SFUProvider, cfg.LiveKitHost, cfg.LiveKitKey, cfg.LiveKitSecret,
		cfg.AgoraAppID, cfg.AgoraAppCertificate, cfg.AgoraHost, cfg.AgoraCustomerID, cfg.AgoraCustomerSecret,
		cfg.MediaSoupBridgeURL, cfg.MediaSoupHost,
		cfg.SRSHost, cfg.SRSApiPort, cfg.SRSWHIPPort, cfg.SRSSecret,
		cfg.DailyAPIKey, cfg.DailyDomain,
	}, "|")
}

func (p *DynamicProvider) SetRoomRegistry(r pkg.RoomRegistry) {
	p.mu.Lock()
	p.roomRegistry = r
	if p.cachedProvider != nil {
		if rs, ok := p.cachedProvider.(pkg.RoomRegistrySetter); ok {
			rs.SetRoomRegistry(r)
		}
	}
	p.mu.Unlock()
}

func (p *DynamicProvider) GenerateToken(room, identity string) (string, error) {
	provider, err := p.current()
	if err != nil {
		return "", err
	}
	return provider.GenerateToken(room, identity)
}

func (p *DynamicProvider) GenerateAdminToken() (string, error) {
	provider, err := p.current()
	if err != nil {
		return "", err
	}
	return provider.GenerateAdminToken()
}

func (p *DynamicProvider) ListRooms() ([]RoomSummary, error) {
	provider, err := p.current()
	if err != nil {
		return nil, err
	}
	return provider.ListRooms()
}

func (p *DynamicProvider) ListParticipants(room string) ([]ParticipantSummary, error) {
	provider, err := p.current()
	if err != nil {
		return nil, err
	}
	return provider.ListParticipants(room)
}

func (p *DynamicProvider) MuteParticipant(room, identity, trackSid string, muted bool) error {
	provider, err := p.current()
	if err != nil {
		return err
	}
	return provider.MuteParticipant(room, identity, trackSid, muted)
}

func (p *DynamicProvider) RemoveParticipant(room, identity string) error {
	provider, err := p.current()
	if err != nil {
		return err
	}
	return provider.RemoveParticipant(room, identity)
}

func (p *DynamicProvider) DeleteRoom(room string) error {
	provider, err := p.current()
	if err != nil {
		return err
	}
	return provider.DeleteRoom(room)
}

func (p *DynamicProvider) GetHost() string {
	provider, err := p.current()
	if err != nil {
		return ""
	}
	return provider.GetHost()
}

func (p *DynamicProvider) ProviderName() string {
	cfg, err := p.resolve()
	if err != nil || cfg.SFUProvider == "" {
		return "livekit"
	}
	return cfg.SFUProvider
}

func (p *DynamicProvider) StreamName(room, identity string) string {
	provider, err := p.current()
	if err != nil {
		return ""
	}
	if sp, ok := provider.(StreamProvider); ok {
		return sp.StreamName(room, identity)
	}
	return ""
}

func (p *DynamicProvider) StreamInfo(room, identity string) (stream, token string, err error) {
	provider, err := p.current()
	if err != nil {
		return "", "", err
	}
	if sp, ok := provider.(StreamProvider); ok {
		return sp.StreamInfo(room, identity)
	}
	return "", "", nil
}

func (p *DynamicProvider) ClientInfo() map[string]interface{} {
	provider, err := p.current()
	if err != nil {
		return map[string]interface{}{}
	}
	if cp, ok := provider.(ClientInfoProvider); ok {
		return cp.ClientInfo()
	}
	return map[string]interface{}{}
}

func (p *DynamicProvider) current() (Provider, error) {
	cfg, err := p.resolve()
	if err != nil {
		return nil, err
	}
	fp := fingerprint(cfg)

	p.mu.RLock()
	cached := p.cachedProvider
	cachedFp := p.cachedFingerprint
	p.mu.RUnlock()
	if cached != nil && cachedFp == fp {
		return cached, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cachedProvider != nil && p.cachedFingerprint == fp {
		return p.cachedProvider, nil
	}
	provider, err := NewProvider(cfg)
	if err != nil {
		return nil, pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	if p.roomRegistry != nil {
		if rs, ok := provider.(pkg.RoomRegistrySetter); ok {
			rs.SetRoomRegistry(p.roomRegistry)
		}
	}
	p.cachedProvider = provider
	p.cachedFingerprint = fp
	return provider, nil
}
```

- [x] **Step 2: Verify sfu package compiles**

Run: `cd app/server && go build ./internal/sfu/`
Expected: compiles

- [x] **Step 3: Commit**

```bash
git add app/server/internal/sfu/dynamic_provider.go
git commit -m "refactor(sfu): update DynamicProvider for typed returns and StreamProvider/ClientInfoProvider"
```

---

### Task 4: Update LiveKit provider

**Files:**
- Modify: `app/server/internal/livekit/client.go`

- [x] **Step 1: Add `sfu` import**

Add `"GOSpeak/internal/sfu"` to the internal-packages import group.

- [x] **Step 2: Replace `ListRooms` method**

```go
func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	if s.client == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	resp, err := s.client.ListRooms(context.Background(), &livekit.ListRoomsRequest{})
	if err != nil {
		return nil, pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	out := make([]sfu.RoomSummary, 0, len(resp.Rooms))
	for _, r := range resp.Rooms {
		out = append(out, sfu.RoomSummary{
			Name:        r.Name,
			MemberCount: int(r.NumParticipants),
		})
	}
	return out, nil
}
```

- [x] **Step 3: Replace `ListParticipants` method**

```go
func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	if s.client == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	resp, err := s.client.ListParticipants(context.Background(), &livekit.ListParticipantsRequest{
		Room: room,
	})
	if err != nil {
		return nil, pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	out := make([]sfu.ParticipantSummary, 0, len(resp.Participants))
	for _, p := range resp.Participants {
		out = append(out, sfu.ParticipantSummary{
			Identity: p.Identity,
			JoinedAt: p.JoinedAt,
		})
	}
	return out, nil
}
```

- [x] **Step 4: Replace `MuteParticipant` and delete `MuteRoomParticipant`**

Replace the existing `MuteParticipant` method and delete the entire `MuteRoomParticipant` method:

```go
func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	if s.client == nil {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	if trackSid != "" {
		_, err := s.client.MutePublishedTrack(context.Background(), &livekit.MuteRoomTrackRequest{
			Room:     room,
			Identity: identity,
			TrackSid: trackSid,
			Muted:    muted,
		})
		if err != nil {
			return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
		}
		return nil
	}
	resp, err := s.client.ListParticipants(context.Background(), &livekit.ListParticipantsRequest{
		Room: room,
	})
	if err != nil {
		return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	for _, p := range resp.Participants {
		if p.Identity == identity {
			for _, track := range p.Tracks {
				if _, err := s.client.MutePublishedTrack(context.Background(), &livekit.MuteRoomTrackRequest{
					Room:     room,
					Identity: identity,
					TrackSid: track.Sid,
					Muted:    muted,
				}); err != nil {
					return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
				}
			}
			break
		}
	}
	return nil
}
```

- [x] **Step 5: Verify compilation**

Run: `cd app/server && go build ./internal/livekit/`
Expected: compiles

- [x] **Step 6: Commit**

```bash
git add app/server/internal/livekit/client.go
git commit -m "refactor(livekit): typed ListRooms/ListParticipants, merge MuteRoomParticipant into MuteParticipant"
```

---

### Task 5: Update Agora provider

**Files:**
- Modify: `app/server/internal/agora/provider.go`

- [x] **Step 1: Add `sfu` import**

Add `"GOSpeak/internal/sfu"` to the internal-packages import group.

- [x] **Step 2: Replace `ListRooms` method**

```go
func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	rooms, err := s.restClient().ListRooms()
	if err != nil {
		return nil, s.mapRESTError(err)
	}
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for _, name := range rooms {
		out = append(out, sfu.RoomSummary{Name: name})
	}
	return out, nil
}
```

- [x] **Step 3: Replace `ListParticipants` method**

```go
func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	users, err := s.restClient().GetChannelUsers(room)
	if err != nil {
		return nil, s.mapRESTError(err)
	}
	out := make([]sfu.ParticipantSummary, 0, len(users))
	for _, uid := range users {
		out = append(out, sfu.ParticipantSummary{Identity: uid})
	}
	return out, nil
}
```

- [x] **Step 4: Delete `MuteRoomParticipant` method**

Remove these lines entirely:

```go
func (s *Service) MuteRoomParticipant(room, identity string, muted bool) error {
	return nil
}
```

- [x] **Step 5: Verify compilation**

Run: `cd app/server && go build ./internal/agora/`
Expected: compiles

- [x] **Step 6: Commit**

```bash
git add app/server/internal/agora/provider.go
git commit -m "refactor(agora): typed ListRooms/ListParticipants, remove MuteRoomParticipant"
```

---

### Task 6: Update SRS provider

**Files:**
- Modify: `app/server/internal/srs/provider.go`
- Modify: `app/server/internal/srs/provider_test.go`

- [x] **Step 1: Add `sfu` import to `provider.go`**

Add `"GOSpeak/internal/sfu"` to the internal-packages import group.

- [x] **Step 2: Replace `ListRooms` method**

```go
func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	if s.registry == nil {
		return nil, pkg.NewAppError(pkg.SFU_ERROR, "srs room registry not configured")
	}
	rooms := s.registry.Rooms()
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for _, name := range rooms {
		out = append(out, sfu.RoomSummary{Name: name})
	}
	return out, nil
}
```

- [x] **Step 3: Replace `ListParticipants` method**

```go
func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	participants, err := s.client.ListParticipants(room)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	out := make([]sfu.ParticipantSummary, 0, len(participants))
	for _, p := range participants {
		out = append(out, sfu.ParticipantSummary{
			Identity: p["id"].(string),
		})
	}
	return out, nil
}
```

- [x] **Step 4: Delete `MuteRoomParticipant` method**

Remove these lines entirely:

```go
func (s *Service) MuteRoomParticipant(room, identity string, muted bool) error {
	return pkg.NewErrSFUNotSupported()
}
```

- [x] **Step 5: Add `sfu` import to `provider_test.go`**

Add `"GOSpeak/internal/sfu"` to the import block.

- [x] **Step 6: Update `TestListRooms_WithRegistry` assertion**

Replace the assertion block:

```go
	got, err := s.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms: %v", err)
	}
	rooms, ok := got.([]sfu.RoomSummary)
	if !ok {
		t.Fatalf("expected []sfu.RoomSummary, got %T", got)
	}
	names := make([]string, len(rooms))
	for i, r := range rooms {
		names[i] = r.Name
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"room-a", "room-b"}) {
		t.Fatalf("expected [room-a room-b], got %v", names)
	}
```

- [x] **Step 7: Update `TestListRooms_WithRegistry_NilReturnsEmpty` assertion**

Replace the assertion block:

```go
	got, err := s.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms: %v", err)
	}
	rooms, ok := got.([]sfu.RoomSummary)
	if !ok || len(rooms) != 0 {
		t.Fatalf("expected empty []sfu.RoomSummary, got %T %v", got, got)
	}
```

- [x] **Step 8: Verify compilation and tests**

Run: `cd app/server && go build ./internal/srs/ && go test ./internal/srs/ -v`
Expected: compiles, all tests PASS

- [x] **Step 9: Commit**

```bash
git add app/server/internal/srs/provider.go app/server/internal/srs/provider_test.go
git commit -m "refactor(srs): typed ListRooms/ListParticipants, remove MuteRoomParticipant"
```

---

### Task 7: Update Daily provider

**Files:**
- Modify: `app/server/internal/daily/provider.go`

- [x] **Step 1: Add `sfu` import**

Add `"GOSpeak/internal/sfu"` to the internal-packages import group.

- [x] **Step 2: Replace `ListRooms` method**

```go
func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	if s.apiKey == "" {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "DAILY_API_KEY is required")
	}
	rooms, err := s.client.ListRooms()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for _, r := range rooms {
		out = append(out, sfu.RoomSummary{Name: r.Name})
	}
	return out, nil
}
```

- [x] **Step 3: Replace `ListParticipants` method**

```go
func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	if s.apiKey == "" {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "DAILY_API_KEY is required")
	}
	participants, err := s.client.ListParticipants(room)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	out := make([]sfu.ParticipantSummary, 0, len(participants))
	for _, p := range participants {
		if id, ok := p["user_name"].(string); ok {
			out = append(out, sfu.ParticipantSummary{Identity: id})
		}
	}
	return out, nil
}
```

- [x] **Step 4: Delete `MuteRoomParticipant` method**

Remove these lines entirely:

```go
func (s *Service) MuteRoomParticipant(room, identity string, muted bool) error {
	return pkg.NewErrSFUNotSupported()
}
```

- [x] **Step 5: Verify compilation**

Run: `cd app/server && go build ./internal/daily/`
Expected: compiles

- [x] **Step 6: Commit**

```bash
git add app/server/internal/daily/provider.go
git commit -m "refactor(daily): typed ListRooms/ListParticipants, remove MuteRoomParticipant"
```

---

### Task 8: Update MediaSoup provider

**Files:**
- Modify: `app/server/internal/mediasoup/provider.go`

- [x] **Step 1: Add `sfu` import**

Add `"GOSpeak/internal/sfu"` to the internal-packages import group.

- [x] **Step 2: Replace `ListRooms` method**

```go
func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	rooms, err := s.Bridge.ListRouters()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for _, name := range rooms {
		out = append(out, sfu.RoomSummary{Name: name})
	}
	return out, nil
}
```

- [x] **Step 3: Replace `ListParticipants` method**

```go
func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	participants, err := s.partBridge.ListParticipants(room)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	out := make([]sfu.ParticipantSummary, 0, len(participants))
	for _, p := range participants {
		out = append(out, sfu.ParticipantSummary{Identity: p.Identity})
	}
	return out, nil
}
```

- [x] **Step 4: Replace `MuteParticipant` and delete `MuteRoomParticipant`**

```go
func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	var err error
	if trackSid != "" {
		if muted {
			err = s.partBridge.PauseProducer(room, trackSid)
		} else {
			err = s.partBridge.ResumeProducer(room, trackSid)
		}
	} else {
		if muted {
			err = s.partBridge.PauseParticipant(room, identity)
		} else {
			err = s.partBridge.ResumeParticipant(room, identity)
		}
	}
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}
```

Delete the entire `MuteRoomParticipant` method.

- [x] **Step 5: Verify compilation and tests**

Run: `cd app/server && go build ./internal/mediasoup/ && go test ./internal/mediasoup/ -v`
Expected: compiles, all tests PASS

- [x] **Step 6: Commit**

```bash
git add app/server/internal/mediasoup/provider.go
git commit -m "refactor(mediasoup): typed ListRooms/ListParticipants, merge MuteRoomParticipant into MuteParticipant"
```

---

### Task 9: Update `service/sfu_service.go`

**Files:**
- Modify: `app/server/internal/service/sfu_service.go`

- [x] **Step 1: Update `ListRooms` and `ListParticipants` return types**

Replace the bottom two methods:

```go
func (s *SFUService) ListRooms() ([]sfu.RoomSummary, error) {
	return s.provider.ListRooms()
}

func (s *SFUService) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	return s.provider.ListParticipants(room)
}
```

- [x] **Step 2: Replace `interface{}` assertions with typed interface assertions**

In `GetJoinToken`, replace the three `if p, ok :=` blocks with:

```go
	if p, ok := s.provider.(sfu.ClientInfoProvider); ok {
		res.ClientInfo = p.ClientInfo()
	}
	if p, ok := s.provider.(sfu.StreamProvider); ok {
		st, stok, err := p.StreamInfo(room, identity)
		if err != nil {
			return nil, err
		}
		res.Stream = st
		res.StreamToken = stok
	}
```

Keep the `ProviderName` assertion unchanged (anonymous interface convention).

- [x] **Step 3: Verify compilation**

Run: `cd app/server && go build ./internal/service/`
Expected: compiles

- [x] **Step 4: Commit**

```bash
git add app/server/internal/service/sfu_service.go
git commit -m "refactor(service): typed ListRooms/ListParticipants, use StreamProvider/ClientInfoProvider"
```

---

### Task 10: Update handler tests

**Files:**
- Modify: `app/server/internal/handler/signal_handler_test.go`

- [x] **Step 1: Add `sfu` import**

Add `"GOSpeak/internal/sfu"` to the import block.

- [x] **Step 2: Replace the `mockSFU` struct and its methods**

```go
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
```

- [x] **Step 3: Update `TestListRooms_Success` mock data**

```go
	sfu := &mockSFU{
		listRoomsFn: func() ([]sfu.RoomSummary, error) {
			return []sfu.RoomSummary{
				{Name: "room-1", MemberCount: 3},
				{Name: "room-2", MemberCount: 1},
			}, nil
		},
	}
```

- [x] **Step 4: Update `TestListRooms_SFUError` mock data**

```go
	sfu := &mockSFU{
		listRoomsFn: func() ([]sfu.RoomSummary, error) {
			return nil, errors.New("livekit unavailable")
		},
	}
```

- [x] **Step 5: Update `TestListParticipants_Success` mock data**

```go
	sfu := &mockSFU{
		listParticipants: func(room string) ([]sfu.ParticipantSummary, error) {
			return []sfu.ParticipantSummary{
				{Identity: "user-1"},
			}, nil
		},
	}
```

- [x] **Step 6: Update `TestGetJoinToken_SFUError` to use `*AppError`**

```go
	sfu := &mockSFU{
		tokenFn: func(room, identity string) (string, error) {
			return "", pkg.NewAppError(pkg.SFU_ERROR, "sfu connection failed")
		},
	}
```

And update the expected code:

```go
	if resp.Code != pkg.SFU_ERROR {
		t.Fatalf("expected SFU_ERROR (6002), got %d", resp.Code)
	}
```

- [x] **Step 7: Run handler tests**

Run: `cd app/server && go test ./internal/handler/ -v`
Expected: all PASS

- [x] **Step 8: Run full test suite**

Run: `cd app/server && go test ./...`
Expected: all PASS

- [x] **Step 9: Commit**

```bash
git add app/server/internal/handler/signal_handler_test.go
git commit -m "test(handler): update mock and tests for typed SFU returns"
```

---

## Phase 2 (P2): Frontend interface — JoinParams + isConnected + async destroy

### Task 11: Update `sfu-client/src/types.ts`

**Files:**
- Modify: `packages/sfu-client/src/types.ts`

- [x] **Step 1: Add `JoinParams` interface**

Add after the `SFUClientOptions` interface:

```typescript
export interface JoinParams {
	token: string;
	serverUrl: string;
	identity: string;
	room?: string;
	stream?: string;
	streamToken?: string;
}
```

- [x] **Step 2: Update `SFUClient` interface**

Replace the `joinRoom` method signature:

```typescript
	joinRoom(params: JoinParams): Promise<void>;
```

Add `isConnected` method after `onReconnected`:

```typescript
	/** Returns true if the media session is currently connected and joined. */
	isConnected(): boolean;
```

Change `destroy()` return type:

```typescript
	/** Final cleanup hook for any remaining provider resources. */
	destroy(): Promise<void>;
```

- [x] **Step 3: Commit**

```bash
git add packages/sfu-client/src/types.ts
git commit -m "refactor(sfu-client): add JoinParams, isConnected, async destroy to SFUClient interface"
```

---

### Task 12: Update `livekit-client.ts`

**Files:**
- Modify: `packages/sfu-client/src/livekit-client.ts`

- [x] **Step 1: Add `JoinParams` to the import**

```typescript
import type { JoinParams, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";
```

- [x] **Step 2: Replace `joinRoom` signature and body**

```typescript
	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: url, identity } = params;
		await this.room.prepareConnection(url, token);
		await this.room.connect(url, token);
		await this.room.localParticipant.setMicrophoneEnabled(true);
	}
```

- [x] **Step 3: Add `isConnected` method**

```typescript
	isConnected(): boolean {
		return !this.hasLeft && this.room.state === "connected";
	}
```

- [x] **Step 4: `destroy()` is already `async` — no change needed**

- [x] **Step 5: Verify type-check**

Run: `cd packages/sfu-client && npx tsc --noEmit`
Expected: type errors only in other 4 client files (not livekit-client.ts)

- [x] **Step 6: Commit**

```bash
git add packages/sfu-client/src/livekit-client.ts
git commit -m "refactor(sfu-client): LiveKitSFUClient uses JoinParams + isConnected"
```

---

### Task 13: Update `agora-client.ts`

**Files:**
- Modify: `packages/sfu-client/src/agora-client.ts`

- [x] **Step 1: Add `JoinParams` to the import**

```typescript
import type { JoinParams, RemoteAudioTrackLike, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";
```

- [x] **Step 2: Replace `joinRoom` signature and body**

```typescript
	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: appId, identity, room } = params;
		const channelName = room || appId;
		const resolvedAppId = appId || this.envAgoraAppId();
		if (!resolvedAppId) {
			throw new Error("Agora App ID is required");
		}

		await this.client.join(resolvedAppId, channelName, token, identity);
		this.hasJoined = true;
		this.localAudioTrack = await AgoraRTC.createMicrophoneAudioTrack({
			AEC: this.options.audioCapture?.echoCancellation ?? true,
			ANS: this.options.audioCapture?.noiseSuppression ?? true,
			AGC: this.options.audioCapture?.autoGainControl ?? true,
			encoderConfig: {
				sampleRate: this.options.audioCapture?.sampleRate,
				stereo: this.options.audioCapture?.channelCount === 2,
				bitrate: this.options.publishAudio?.maxBitrate,
			},
		});
		await this.client.publish(this.localAudioTrack);
	}
```

- [x] **Step 3: Add `isConnected` method**

```typescript
	isConnected(): boolean {
		return this.hasJoined;
	}
```

- [x] **Step 4: Change `destroy()` to `async`**

```typescript
	async destroy(): Promise<void> {
		await this.leaveRoom();
		this.client.removeAllListeners();
	}
```

- [x] **Step 5: Verify type-check**

Run: `cd packages/sfu-client && npx tsc --noEmit`
Expected: type errors only in other 3 client files

- [x] **Step 6: Commit**

```bash
git add packages/sfu-client/src/agora-client.ts
git commit -m "refactor(sfu-client): AgoraSFUClient uses JoinParams + isConnected + async destroy"
```

---

### Task 14: Update `srs-client.ts`

**Files:**
- Modify: `packages/sfu-client/src/srs-client.ts`

- [x] **Step 1: Add `JoinParams` to the import**

```typescript
import type { JoinParams, RemoteAudioTrackLike, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";
```

- [x] **Step 2: Replace `joinRoom` signature and body**

Change the method signature from:

```typescript
	async joinRoom(
		token: string,
		url: string,
		identity: string,
		room?: string,
		stream?: string,
		streamToken?: string,
	): Promise<void> {
```

to:

```typescript
	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: url, identity, room, stream, streamToken } = params;
```

Keep the rest of the method body unchanged.

- [x] **Step 3: Add `isConnected` method**

```typescript
	isConnected(): boolean {
		return this.hasJoined && !this.leaving;
	}
```

- [x] **Step 4: Change `destroy()` to `async`**

```typescript
	async destroy(): Promise<void> {
		await this.leaveRoom();
	}
```

- [x] **Step 5: Verify type-check**

Run: `cd packages/sfu-client && npx tsc --noEmit`
Expected: type errors only in other 2 client files (daily, mediasoup)

- [x] **Step 6: Commit**

```bash
git add packages/sfu-client/src/srs-client.ts
git commit -m "refactor(sfu-client): SRSSFUClient uses JoinParams + isConnected + async destroy"
```

---

### Task 15: Update `daily-client.ts`

**Files:**
- Modify: `packages/sfu-client/src/daily-client.ts`

- [x] **Step 1: Add `JoinParams` to the import**

```typescript
import type { JoinParams, RemoteAudioTrackLike, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";
```

- [x] **Step 2: Replace `joinRoom` signature and body**

```typescript
	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: url, identity, room } = params;
		const dailyModule = await import("@daily-co/daily-js");
		const daily = dailyModule.default;
		const callObject = daily.createCallObject();
		this.callObject = callObject;
		this.bindEvents(callObject);
		const resolvedURL = this.resolveRoomURL(url, room);
		await callObject.join({
			url: resolvedURL,
			token,
			userName: identity,
		});
		callObject.setLocalAudio(true);
		this.hasJoined = true;
	}
```

- [x] **Step 3: Add `isConnected` method**

```typescript
	isConnected(): boolean {
		return this.hasJoined;
	}
```

- [x] **Step 4: Change `destroy()` to `async`**

```typescript
	async destroy(): Promise<void> {
		await this.leaveRoom();
	}
```

- [x] **Step 5: Verify type-check**

Run: `cd packages/sfu-client && npx tsc --noEmit`
Expected: type errors only in mediasoup-client.ts

- [x] **Step 6: Commit**

```bash
git add packages/sfu-client/src/daily-client.ts
git commit -m "refactor(sfu-client): DailySFUClient uses JoinParams + isConnected + async destroy"
```

---

### Task 16: Update `mediasoup-client.ts`

**Files:**
- Modify: `packages/sfu-client/src/mediasoup-client.ts`

- [x] **Step 1: Add `JoinParams` to the import**

```typescript
import type { JoinParams, RemoteAudioTrackLike, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";
```

- [x] **Step 2: Replace `joinRoom` signature and body**

Change the method signature from:

```typescript
	async joinRoom(
		token: string,
		_url: string,
		identity: string,
		room?: string,
	): Promise<void> {
		if (!this.socket) throw new Error("mediasoup client requires a socket.io socket");

		const [tokenRoom, tokenIdentity] = token.split(":", 2);
		this.roomId = room || tokenRoom;
		this.identity = tokenIdentity || identity;
```

to:

```typescript
	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: _url, identity, room } = params;
		if (!this.socket) throw new Error("mediasoup client requires a socket.io socket");

		const [tokenRoom, tokenIdentity] = token.split(":", 2);
		this.roomId = room || tokenRoom;
		this.identity = tokenIdentity || identity;
```

Keep the rest of the method body unchanged.

- [x] **Step 3: Add `isConnected` method**

```typescript
	isConnected(): boolean {
		return this.hasJoined;
	}
```

- [x] **Step 4: `destroy()` is already `async` — no change needed**

- [x] **Step 5: Verify type-check passes**

Run: `cd packages/sfu-client && npx tsc --noEmit`
Expected: no type errors

- [x] **Step 6: Run sfu-client tests**

Run: `cd packages/sfu-client && npx vitest run`
Expected: all PASS

- [x] **Step 7: Commit**

```bash
git add packages/sfu-client/src/mediasoup-client.ts
git commit -m "refactor(sfu-client): MediaSoupSFUClient uses JoinParams + isConnected"
```

---

### Task 17: Update `useRoomJoinSession.ts` caller

**Files:**
- Modify: `app/web/src/components/room/hooks/useRoomJoinSession.ts`

- [x] **Step 1: Add `JoinParams` to the import from `@gospeak/sfu-client/types`**

Update the existing import line to include `JoinParams`:

```typescript
import type { SFUClient, JoinParams } from "@gospeak/sfu-client/types";
```

- [x] **Step 2: Replace the `joinRoom` call**

Find the existing call (around line 200):

```typescript
						await raceAbort(
							createdClient.joinRoom(
								data.token,
								sessionMeta.connectTarget,
								data.identity,
								data.room,
								data.stream,
								data.streamToken,
							),
							signal,
						);
```

Replace with:

```typescript
						const joinParams: JoinParams = {
							token: data.token,
							serverUrl: sessionMeta.connectTarget,
							identity: data.identity,
							room: data.room,
							stream: data.stream,
							streamToken: data.streamToken,
						};
						await raceAbort(
							createdClient.joinRoom(joinParams),
							signal,
						);
```

- [x] **Step 3: Verify web type-check**

Run: `cd app/web && npx tsc --noEmit`
Expected: no type errors

- [x] **Step 4: Commit**

```bash
git add app/web/src/components/room/hooks/useRoomJoinSession.ts
git commit -m "refactor(web): use JoinParams for SFUClient.joinRoom()"
```

---

## Final Verification

### Task 18: Full build and test sweep

- [x] **Step 1: Run Go tests**

Run: `cd app/server && go test ./...`
Expected: all PASS

- [x] **Step 2: Run sfu-client type-check and tests**

Run: `cd packages/sfu-client && npx tsc --noEmit && npx vitest run`
Expected: no type errors, all tests PASS

- [x] **Step 3: Run web type-check**

Run: `cd app/web && npx tsc --noEmit`
Expected: no type errors

- [x] **Step 4: Commit (if any remaining fixes)**

```bash
git add -A
git commit -m "chore: final verification fixes for SFU adapter refactor"
```

---

## Out of Scope

The following item from the architecture review is intentionally excluded from this plan:

- **P3: `SFUSignalHandler` signal-library decoupling** — Replacing `RegisterRoutes(server *socketio.Server)` with an abstract `SignalConnection` interface is a larger architectural change that touches the signal hub, mediasoup signal handler, and requires a new adapter layer. It can be pursued as a separate plan once Phase 1 and Phase 2 are stable. No current feature is blocked by leaving the socket.io coupling in place.
