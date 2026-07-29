package cloudflare

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

var _ sfu.Provider = (*Service)(nil)
var _ sfu.StreamProvider = (*Service)(nil)
var _ sfu.ClientInfoProvider = (*Service)(nil)
// Service implements sfu.Provider for Cloudflare Realtime SFU.
// Cloudflare has no native rooms; sessions are tracked locally after GenerateToken.
type Service struct {
	client   *Client
	appID    string
	stunURL  string
	registry pkg.RoomRegistry

	mu       sync.RWMutex
	sessions map[string]map[string]sessionMeta // room -> identity -> meta
}

type sessionMeta struct {
	sessionID string
	joinedAt  int64
}

func NewService(cfg *config.Config) *Service {
	stunURL := cfg.CFStunURL
	if stunURL == "" {
		stunURL = "stun.cloudflare.com:3478"
	}
	return &Service{
		client:   NewClient(cfg.CFAppID, cfg.CFAppSecret),
		appID:    cfg.CFAppID,
		stunURL:  stunURL,
		sessions: make(map[string]map[string]sessionMeta),
	}
}

func (s *Service) SetRoomRegistry(r pkg.RoomRegistry) {
	s.registry = r
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	if s.appID == "" || s.client == nil || s.client.appSecret == "" {
		return "", pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "CF_APP_ID and CF_APP_SECRET are required")
	}

	sessionResp, err := s.client.CreateSession(&NewSessionRequest{
		CorrelationID: room,
	})
	if err != nil {
		return "", pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "cloudflare create session: "+err.Error())
	}

	now := time.Now().Unix()
	s.putSession(room, identity, sessionResp.SessionID, now)

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

// GenerateAdminToken is not supported: Cloudflare Realtime sessions are per-participant.
func (s *Service) GenerateAdminToken() (string, error) {
	return "", pkg.NewErrSFUNotSupported()
}

func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]sfu.RoomSummary, 0, len(s.sessions))
	for name, members := range s.sessions {
		out = append(out, sfu.RoomSummary{
			Name:        name,
			MemberCount: len(members),
		})
	}
	return out, nil
}

func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	members := s.sessions[room]
	out := make([]sfu.ParticipantSummary, 0, len(members))
	for identity, meta := range members {
		out = append(out, sfu.ParticipantSummary{
			Identity: identity,
			JoinedAt: meta.joinedAt,
		})
	}
	return out, nil
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return pkg.NewErrSFUNotSupported()
}

func (s *Service) RemoveParticipant(room, identity string) error {
	if s.appID == "" {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "CF_APP_ID is required")
	}

	meta, ok := s.getSession(room, identity)
	if !ok || meta.sessionID == "" {
		return nil
	}

	if err := s.client.DeleteSession(meta.sessionID); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "cloudflare delete session: "+err.Error())
	}
	s.deleteSession(room, identity)
	return nil
}

func (s *Service) DeleteRoom(room string) error {
	if s.appID == "" {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "CF_APP_ID is required")
	}

	s.mu.Lock()
	members := s.sessions[room]
	sessionIDs := make([]string, 0, len(members))
	for _, meta := range members {
		sessionIDs = append(sessionIDs, meta.sessionID)
	}
	delete(s.sessions, room)
	s.mu.Unlock()

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		lastErr error
	)
	for _, sessionID := range sessionIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := s.client.DeleteSession(id); err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
			}
		}(sessionID)
	}
	wg.Wait()

	if s.registry != nil {
		s.registry.ClearRoom(room)
	}
	if lastErr != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, lastErr, "cloudflare delete session: "+lastErr.Error())
	}
	return nil
}

func (s *Service) GetHost() string {
	return fmt.Sprintf("https://rtc.live.cloudflare.com/v1/apps/%s", s.appID)
}

func (s *Service) ProviderName() string {
	return "cloudflare"
}

func (s *Service) Capabilities() sfu.Capabilities {
	return sfu.CapabilitiesFor("cloudflare")
}


func (s *Service) StreamName(room, identity string) string {
	meta, ok := s.getSession(room, identity)
	if !ok {
		return ""
	}
	return meta.sessionID
}

func (s *Service) StreamInfo(room, identity string) (string, string, error) {
	meta, ok := s.getSession(room, identity)
	if !ok || meta.sessionID == "" {
		return "", "", nil
	}
	return meta.sessionID, "", nil
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"appId":   s.appID,
		"stunUrl": s.stunURL,
	}
}

func (s *Service) putSession(room, identity, sessionID string, joinedAt int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[room] == nil {
		s.sessions[room] = make(map[string]sessionMeta)
	}
	s.sessions[room][identity] = sessionMeta{sessionID: sessionID, joinedAt: joinedAt}
}

func (s *Service) getSession(room, identity string) (sessionMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.sessions[room][identity]
	return m, ok
}

func (s *Service) deleteSession(room, identity string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[room] == nil {
		return
	}
	delete(s.sessions[room], identity)
	if len(s.sessions[room]) == 0 {
		delete(s.sessions, room)
	}
}
