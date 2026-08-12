package factory

import (
	"strings"
	"sync"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

type ConfigResolver func() (*config.Config, error)

type ownerAwareProvider interface {
	GenerateTokenForUser(room, identity, ownerUUID string) (string, error)
}

type sessionOwnerLookup interface {
	SessionOwner(sessionID string) (string, bool)
}

type sessionDomainLookup interface {
	SessionDomain(sessionID string) (string, bool)
}

type DynamicProvider struct {
	resolve            ConfigResolver
	mu                 sync.RWMutex
	roomRegistry       pkg.RoomRegistry
	streamRoomResolver pkg.StreamRoomResolver
	muteRuleStore      sfu.MuteRuleStore
	cachedFingerprint  string
	cachedProvider     sfu.Provider
}

// providerCloser 是可选资源释放接口；provider 重建时旧实例若实现则关闭，
// 避免 gRPC/HTTP client 在频繁热切换中累积。
type providerCloser interface {
	Close() error
}

func NewDynamicProvider(resolve ConfigResolver) *DynamicProvider {
	return &DynamicProvider{resolve: resolve}
}

func fingerprint(cfg *config.Config) string {
	// Keep field list centralized so provider config additions update the cache key.
	fields := []string{
		cfg.SFUProvider,
		cfg.LiveKitHost, cfg.LiveKitKey, cfg.LiveKitSecret,
		cfg.AgoraAppID, cfg.AgoraAppCertificate, cfg.AgoraHost, cfg.AgoraCustomerID, cfg.AgoraCustomerSecret,
		cfg.MediaSoupBridgeURL, cfg.MediaSoupHost,
		cfg.SRSHost, cfg.SRSApiPort, cfg.SRSWHIPURL, cfg.SRSSecret, cfg.SRSPublicHost,
		cfg.DailyAPIKey, cfg.DailyDomain,
		cfg.CFAppID, cfg.CFAppSecret, cfg.CFStunURL,
	}
	return strings.Join(fields, "|")
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

// SetStreamRoomResolver 注入 stream→room 反查，转发给需要它的 provider。
func (p *DynamicProvider) SetStreamRoomResolver(r pkg.StreamRoomResolver) {
	p.mu.Lock()
	p.streamRoomResolver = r
	if p.cachedProvider != nil {
		if rs, ok := p.cachedProvider.(pkg.StreamRoomResolverSetter); ok {
			rs.SetStreamRoomResolver(r)
		}
	}
	p.mu.Unlock()
}

// SetMuteRuleStore injects multi-instance mute rule cache into providers that need it.
func (p *DynamicProvider) SetMuteRuleStore(store sfu.MuteRuleStore) {
	p.mu.Lock()
	p.muteRuleStore = store
	if p.cachedProvider != nil {
		if ms, ok := p.cachedProvider.(sfu.MuteRuleStoreSetter); ok {
			ms.SetMuteRuleStore(store)
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

// GenerateTokenForUser forwards owner-aware token generation when the
// active provider supports it, so sessions can be bound to their creator.
func (p *DynamicProvider) GenerateTokenForUser(room, identity, ownerUUID string) (string, error) {
	provider, err := p.current()
	if err != nil {
		return "", err
	}
	if op, ok := provider.(ownerAwareProvider); ok {
		return op.GenerateTokenForUser(room, identity, ownerUUID)
	}
	return provider.GenerateToken(room, identity)
}

// SessionOwner returns the user UUID bound to a provider session when supported.
func (p *DynamicProvider) SessionOwner(sessionID string) (string, bool) {
	provider, err := p.current()
	if err != nil {
		return "", false
	}
	if lp, ok := provider.(sessionOwnerLookup); ok {
		return lp.SessionOwner(sessionID)
	}
	return "", false
}

// SessionDomain 返回 provider session 所属 Domain（受支持时）。
func (p *DynamicProvider) SessionDomain(sessionID string) (string, bool) {
	provider, err := p.current()
	if err != nil {
		return "", false
	}
	if lp, ok := provider.(sessionDomainLookup); ok {
		return lp.SessionDomain(sessionID)
	}
	return "", false
}

func (p *DynamicProvider) ListRooms() ([]sfu.RoomSummary, error) {
	provider, err := p.current()
	if err != nil {
		return nil, err
	}
	return provider.ListRooms()
}

func (p *DynamicProvider) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
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

// MuteParticipantTimed forwards TTL-aware mute when the active provider supports it.
func (p *DynamicProvider) MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error {
	provider, err := p.current()
	if err != nil {
		return err
	}
	if tp, ok := provider.(sfu.TimedMuteProvider); ok {
		return tp.MuteParticipantTimed(room, identity, trackSid, muted, ttlSeconds)
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

// ProviderName returns the active provider id.
// Prefer the cached concrete provider only when its fingerprint still matches
// the latest resolved config, so hot switches never report a stale name.
func (p *DynamicProvider) ProviderName() string {
	cfg, err := p.resolve()
	if err != nil {
		p.mu.RLock()
		cached := p.cachedProvider
		p.mu.RUnlock()
		if cached != nil {
			return cached.ProviderName()
		}
		return "livekit"
	}

	fp := fingerprint(cfg)
	p.mu.RLock()
	cached := p.cachedProvider
	cachedFp := p.cachedFingerprint
	p.mu.RUnlock()
	if cached != nil && cachedFp == fp {
		return cached.ProviderName()
	}
	if cfg.SFUProvider == "" {
		return "livekit"
	}
	return cfg.SFUProvider
}

func (p *DynamicProvider) Capabilities() sfu.Capabilities {
	provider, err := p.current()
	if err != nil {
		return sfu.Capabilities{}
	}
	return provider.Capabilities()
}

// SupportsStream 仅当底层 provider 实现 StreamProvider 时为 true。
func (p *DynamicProvider) SupportsStream() bool {
	provider, err := p.current()
	if err != nil {
		return false
	}
	_, ok := provider.(sfu.StreamProvider)
	return ok
}

func (p *DynamicProvider) StreamName(room, identity string) string {
	provider, err := p.current()
	if err != nil {
		return ""
	}
	if sp, ok := provider.(sfu.StreamProvider); ok {
		return sp.StreamName(room, identity)
	}
	return ""
}

func (p *DynamicProvider) StreamInfo(room, identity string) (stream, token string, err error) {
	provider, err := p.current()
	if err != nil {
		return "", "", err
	}
	if sp, ok := provider.(sfu.StreamProvider); ok {
		return sp.StreamInfo(room, identity)
	}
	return "", "", pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "stream not supported")
}

func (p *DynamicProvider) ClientInfo() map[string]interface{} {
	provider, err := p.current()
	if err != nil {
		return map[string]interface{}{}
	}
	if cp, ok := provider.(sfu.ClientInfoProvider); ok {
		return cp.ClientInfo()
	}
	return map[string]interface{}{}
}

// Close 释放当前缓存的 provider 资源（若实现 Close）。
func (p *DynamicProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cachedProvider == nil {
		return nil
	}
	if closer, ok := p.cachedProvider.(providerCloser); ok {
		return closer.Close()
	}
	return nil
}

func (p *DynamicProvider) current() (sfu.Provider, error) {
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
	if old, ok := p.cachedProvider.(providerCloser); ok && old != nil {
		_ = old.Close()
	}
	if p.roomRegistry != nil {
		if rs, ok := provider.(pkg.RoomRegistrySetter); ok {
			rs.SetRoomRegistry(p.roomRegistry)
		}
	}
	if p.streamRoomResolver != nil {
		if rs, ok := provider.(pkg.StreamRoomResolverSetter); ok {
			rs.SetStreamRoomResolver(p.streamRoomResolver)
		}
	}
	if p.muteRuleStore != nil {
		if ms, ok := provider.(sfu.MuteRuleStoreSetter); ok {
			ms.SetMuteRuleStore(p.muteRuleStore)
		}
	}
	p.cachedProvider = provider
	p.cachedFingerprint = fp
	return provider, nil
}
