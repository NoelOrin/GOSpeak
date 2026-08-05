package agora

import (
	"context"
	"errors"
	"fmt"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

// defaultMuteRuleTTL used when caller does not pass mute duration.
const defaultMuteRuleTTL = 24 * 60 * 60 // 24h

type Service struct {
	appID          string
	appCertificate string
	host           string
	customerID     string
	customerSecret string

	// muteRules is multi-instance rule id cache (memory / redis / nats KV).
	muteRules sfu.MuteRuleStore
	rest      *RESTClient
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		appID:          cfg.AgoraAppID,
		appCertificate: cfg.AgoraAppCertificate,
		host:           cfg.AgoraHost,
		customerID:     cfg.AgoraCustomerID,
		customerSecret: cfg.AgoraCustomerSecret,
		muteRules:      sfu.NewMemoryMuteRuleStore(),
		rest:           NewRESTClient(cfg.AgoraAppID, cfg.AgoraCustomerID, cfg.AgoraCustomerSecret),
	}
}

// SetMuteRuleStore injects shared mute-rule cache (redis/nats preferred).
func (s *Service) SetMuteRuleStore(store sfu.MuteRuleStore) {
	if s == nil {
		return
	}
	if store == nil {
		s.muteRules = sfu.NewMemoryMuteRuleStore()
		return
	}
	s.muteRules = store
}

func (s *Service) ProviderName() string { return "agora" }

func (s *Service) Capabilities() sfu.Capabilities {
	return sfu.CapabilitiesFor("agora")
}

// Close 释放 provider 资源；Agora REST client 为每次调用创建，无长期句柄。
func (s *Service) Close() error { return nil }

func (s *Service) GenerateToken(room, identity string) (string, error) {
	if s.appID == "" || s.appCertificate == "" {
		return "", pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "AGORA_APP_ID and AGORA_APP_CERTIFICATE are required")
	}
	// Account-mode token: identity is a string userAccount, matching kicking-rule uid string.
	token, err := buildRTCToken(s.appID, s.appCertificate, room, identity)
	if err != nil {
		return "", pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	return token, nil
}

func (s *Service) GenerateAdminToken() (string, error) {
	return "", pkg.NewErrSFUNotSupported()
}

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

// MuteParticipant degraded hard mute via publish privilege rule.
// Without a rule id, returns error so Hub does NOT claim hard/degraded success.
func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return s.MuteParticipantTimed(room, identity, trackSid, muted, defaultMuteRuleTTL)
}

// MuteParticipantTimed implements sfu.TimedMuteProvider.
func (s *Service) MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error {
	if s.appID == "" {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "AGORA_APP_ID is required")
	}
	key := muteRuleKey(room, identity)
	if !muted {
		return s.clearMuteRule(room, identity, key)
	}
	if ttlSeconds <= 0 {
		ttlSeconds = defaultMuteRuleTTL
	}

	// Replace any existing rule first (best effort).
	_ = s.clearMuteRule(room, identity, key)

	ruleID, err := s.restClient().CreateKickingRule(
		room,
		identity,
		ttlSeconds,
		[]string{"publish_audio", "publish_video"},
	)
	if err != nil {
		return s.mapRESTError(err)
	}
	if ruleID <= 0 {
		return pkg.NewAppError(pkg.SFU_ERROR, "agora mute rule created without id; refuse hard success")
	}

	_ = s.saveRule(key, ruleID, time.Duration(ttlSeconds)*time.Second)
	return nil
}

// RemoveParticipant degraded hard kick: short join_channel rule.
func (s *Service) RemoveParticipant(room, identity string) error {
	if s.appID == "" {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "AGORA_APP_ID is required")
	}
	if _, err := s.restClient().CreateKickingRule(room, identity, 60, []string{"join_channel"}); err != nil {
		return s.mapRESTError(err)
	}
	return nil
}

func (s *Service) DeleteRoom(room string) error {
	if err := s.restClient().DeleteChannel(room); err != nil {
		return s.mapRESTError(err)
	}
	return nil
}

func (s *Service) GetHost() string {
	if s.host != "" {
		return s.host
	}
	return "https://api.agora.io"
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{"appId": s.appID}
}

func (s *Service) restClient() *RESTClient {
	if s.rest != nil {
		return s.rest
	}
	return NewRESTClient(s.appID, s.customerID, s.customerSecret)
}

func (s *Service) mapRESTError(err error) error {
	if errors.Is(err, ErrRESTCredentialsMissing) {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, ErrRESTCredentialsMissing.Error())
	}
	return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
}

func (s *Service) ruleStore() sfu.MuteRuleStore {
	if s == nil || s.muteRules == nil {
		return sfu.NewMemoryMuteRuleStore()
	}
	return s.muteRules
}

func (s *Service) saveRule(key string, ruleID int, ttl time.Duration) error {
	return s.ruleStore().Save(context.Background(), key, ruleID, ttl)
}

func (s *Service) getRule(key string) int {
	id, err := s.ruleStore().Get(context.Background(), key)
	if err != nil {
		return 0
	}
	return id
}

func (s *Service) deleteRule(key string) {
	_ = s.ruleStore().Delete(context.Background(), key)
}

func (s *Service) clearMuteRule(room, identity, key string) error {
	ruleID := s.getRule(key)
	// Recovery path: list rules by channel+identity when store miss.
	if ruleID == 0 {
		ids, err := s.restClient().FindKickingRuleIDs(room, identity)
		if err == nil {
			var last error
			for _, id := range ids {
				if err := s.restClient().DeleteKickingRule(id); err != nil {
					last = err
				}
			}
			s.deleteRule(key)
			if last != nil {
				return s.mapRESTError(last)
			}
			return nil
		}
		// No stored id and list failed: soft-unmute only (policy already cleared).
		return nil
	}
	if err := s.restClient().DeleteKickingRule(ruleID); err != nil {
		// Keep store key so a later retry can still try delete.
		return s.mapRESTError(err)
	}
	s.deleteRule(key)
	return nil
}

func muteRuleKey(room, identity string) string {
	return fmt.Sprintf("%s|%s", room, identity)
}
