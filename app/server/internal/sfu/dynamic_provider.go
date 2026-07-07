package sfu

import (
	"sync"
	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

type ConfigResolver func() (*config.Config, error)

type DynamicProvider struct {
	resolve     ConfigResolver
	mu           sync.RWMutex
	roomRegistry pkg.RoomRegistry
}

func NewDynamicProvider(resolve ConfigResolver) *DynamicProvider {
	return &DynamicProvider{resolve: resolve}
}

// SetRoomRegistry 注入 Hub 聚合视图，转发给每次 current() 重建的底层 provider。
// 因 current() 每次 NewProvider，setter 必须在重建后重放，否则注入丢失。
func (p *DynamicProvider) SetRoomRegistry(r pkg.RoomRegistry) {
	p.mu.Lock()
	p.roomRegistry = r
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
	provider, err := NewProvider(cfg)
	if err != nil {
		return nil, pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	// 每次重建后重放 roomRegistry 注入（SRS 等实现 pkg.RoomRegistrySetter）。
	p.mu.RLock()
	registry := p.roomRegistry
	p.mu.RUnlock()
	if registry != nil {
		if rs, ok := provider.(pkg.RoomRegistrySetter); ok {
			rs.SetRoomRegistry(registry)
		}
	}
	return provider, nil
}
