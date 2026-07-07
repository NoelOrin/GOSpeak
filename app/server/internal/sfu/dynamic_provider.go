package sfu

import (
	"strings"
	"sync"
	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

type ConfigResolver func() (*config.Config, error)

type DynamicProvider struct {
	resolve ConfigResolver
	mu           sync.RWMutex
	roomRegistry pkg.RoomRegistry
	// 缓存底层 provider：LiveKit NewService 建 gRPC client，每次调用重建开销大。
	// 按 SFU 连接字段指纹失效——SFUConfigHandler 改 DB 后 ResolveConfig 返新值，指纹变即重建。
	cachedFingerprint string
	cachedProvider    Provider
}

func NewDynamicProvider(resolve ConfigResolver) *DynamicProvider {
	return &DynamicProvider{resolve: resolve}
}

// fingerprint 取 SFU 连接相关字段：provider 类型 + 各后端 host/key/secret/port。
// 仅这些决定 NewProvider 构建的 client 连接，其余 cfg 字段不影响。
func fingerprint(cfg *config.Config) string {
	return strings.Join([]string{
		cfg.SFUProvider, cfg.LiveKitHost, cfg.LiveKitKey, cfg.LiveKitSecret,
		cfg.AgoraAppID, cfg.AgoraAppCertificate, cfg.AgoraHost, cfg.AgoraCustomerID, cfg.AgoraCustomerSecret,
		cfg.MediaSoupBridgeURL, cfg.MediaSoupHost,
		cfg.SRSHost, cfg.SRSApiPort, cfg.SRSWHIPPort, cfg.SRSSecret,
		cfg.DailyAPIKey, cfg.DailyDomain,
	}, "|")
}

// SetRoomRegistry 注入 Hub 聚合视图，转发给已缓存的底层 provider。
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

func (p *DynamicProvider) ListRooms() (interface{}, error) {
	provider, err := p.current()
	if err != nil {
		return nil, err
	}
	return provider.ListRooms()
}

func (p *DynamicProvider) ListParticipants(room string) (interface{}, error) {
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

func (p *DynamicProvider) MuteRoomParticipant(room, identity string, muted bool) error {
	provider, err := p.current()
	if err != nil {
		return err
	}
	return provider.MuteRoomParticipant(room, identity, muted)
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
	if sn, ok := provider.(interface{ StreamName(room, identity string) string }); ok {
		return sn.StreamName(room, identity)
	}
	return ""
}

func (p *DynamicProvider) StreamInfo(room, identity string) (stream, token string, err error) {
	provider, err := p.current()
	if err != nil {
		return "", "", err
	}
	if sp, ok := provider.(interface {
		StreamInfo(room, identity string) (string, string, error)
	}); ok {
		return sp.StreamInfo(room, identity)
	}
	return "", "", nil
}

func (p *DynamicProvider) ClientInfo() map[string]interface{} {
	provider, err := p.current()
	if err != nil {
		return map[string]interface{}{}
	}
	if infoProvider, ok := provider.(interface{ ClientInfo() map[string]interface{} }); ok {
		return infoProvider.ClientInfo()
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
