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

// Service implements sfu.Provider for Cloudflare Realtime SFU.
// Cloudflare has no native rooms; sessions are tracked locally after GenerateToken.
type Service struct {
	client   *Client
	appID    string
	stunURL  string
	registry pkg.RoomRegistry

	mu       sync.RWMutex
	sessions map[string]map[string]string // room -> identity -> sessionID
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
		sessions: make(map[string]map[string]string),
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

	s.putSession(room, identity, sessionResp.SessionID)

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
	for identity := range members {
		out = append(out, sfu.ParticipantSummary{
			Identity: identity,
			JoinedAt: time.Now().Unix(),
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

	sessionID, ok := s.getSession(room, identity)
	if !ok || sessionID == "" {
		return nil
	}

	if err := s.client.DeleteSession(sessionID); err != nil {
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
	for _, sessionID := range members {
		sessionIDs = append(sessionIDs, sessionID)
	}
	delete(s.sessions, room)
	s.mu.Unlock()

	var lastErr error
	for _, sessionID := range sessionIDs {
		if err := s.client.DeleteSession(sessionID); err != nil {
			lastErr = err
		}
	}
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

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"appId":   s.appID,
		"stunUrl": s.stunURL,
	}
}

func (s *Service) putSession(room, identity, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[room] == nil {
		s.sessions[room] = make(map[string]string)
	}
	s.sessions[room][identity] = sessionID
}

func (s *Service) getSession(room, identity string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessionID, ok := s.sessions[room][identity]
	return sessionID, ok
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
