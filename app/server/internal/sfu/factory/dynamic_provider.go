package factory

import (
	"strings"
	"sync"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

type ConfigResolver func() (*config.Config, error)

// DynamicProvider 是 SFU 配置热切换门面：按 resolve 出的配置缓存并切换
// 当前 provider，方法调用全部纯转发（含 ErrSFUNotSupported），降级决策由调用方做。
type DynamicProvider struct {
	resolve           ConfigResolver
	mu                sync.RWMutex
	cachedFingerprint string
	cachedProvider    sfu.Provider
	cachedConfig      *cachedConfig
}

// providerCloser 是可选资源释放接口；provider 重建时旧实例若实现则关闭，
// 避免 gRPC/HTTP client 在频繁热切换中累积。
type providerCloser interface {
	Close() error
}

// configCacheTTL 控制热路径上配置解析结果的缓存时长。每次 SFU 操作（生成 token、
// mute/kick/ListRooms）原本都要走 SFUConfigService.ResolveConfig 打 2 次 DB，
// 这里缓存解析结果，切 provider/改配置后最多延迟一个 TTL 生效，对语音路由无感。
const configCacheTTL = 2 * time.Second

// cachedConfig 缓存最近一次 resolve 出来的 *config.Config，连同其 fingerprint 与写入时间，
// 避免 current()/ProviderName() 在热路径上反复命中数据库。
type cachedConfig struct {
	cfg      *config.Config
	fp       string
	storedAt time.Time
}

func NewDynamicProvider(resolve ConfigResolver) *DynamicProvider {
	return &DynamicProvider{resolve: resolve}
}

func fingerprint(cfg *config.Config) string {
	// Keep field list centralized so provider config additions update the cache key.
	fields := []string{
		cfg.SFUProvider,
		cfg.LiveKitHost, cfg.LiveKitPublicURL, cfg.LiveKitKey, cfg.LiveKitSecret,
	}
	return strings.Join(fields, "|")
}

// resolveCached 每次仍执行 resolve（缓冲 provider 配置解析），但当 fingerprint 在 TTL 内
// 未变化时复用已解析的 cfg，避免热路径频繁重建 provider 与缓存写入。resolve 失败时：
// 有未过期的旧缓存则继续用旧配置（热路径不全挂），否则透传错误。
func (p *DynamicProvider) resolveCached() (*config.Config, string, error) {
	p.mu.RLock()
	cc := p.cachedConfig
	p.mu.RUnlock()

	cfg, err := p.resolve()
	if err != nil {
		if cc != nil && time.Since(cc.storedAt) < configCacheTTL {
			return cc.cfg, cc.fp, nil
		}
		return nil, "", err
	}
	fp := fingerprint(cfg)
	if cc != nil && fp == cc.fp && time.Since(cc.storedAt) < configCacheTTL {
		return cc.cfg, cc.fp, nil
	}
	p.mu.Lock()
	p.cachedConfig = &cachedConfig{cfg: cfg, fp: fp, storedAt: time.Now()}
	p.mu.Unlock()
	return cfg, fp, nil
}

func (p *DynamicProvider) GenerateToken(room, identity string) (string, error) {
	provider, err := p.current()
	if err != nil {
		return "", err
	}
	return provider.GenerateToken(room, identity)
}

func (p *DynamicProvider) GenerateTokenWithPublish(room, identity string, canPublish bool) (string, error) {
	provider, err := p.current()
	if err != nil {
		return "", err
	}
	return provider.GenerateTokenWithPublish(room, identity, canPublish)
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

func (p *DynamicProvider) MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error {
	provider, err := p.current()
	if err != nil {
		return err
	}
	return provider.MuteParticipantTimed(room, identity, trackSid, muted, ttlSeconds)
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
	cfg, fp, err := p.resolveCached()
	if err != nil {
		p.mu.RLock()
		cached := p.cachedProvider
		p.mu.RUnlock()
		if cached != nil {
			return cached.ProviderName()
		}
		return sfu.DefaultProviderName
	}

	p.mu.RLock()
	cached := p.cachedProvider
	cachedFp := p.cachedFingerprint
	p.mu.RUnlock()
	if cached != nil && cachedFp == fp {
		return cached.ProviderName()
	}
	if cfg.SFUProvider == "" {
		return sfu.DefaultProviderName
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

// SupportsStream 仅当底层 provider 支持流寻址时为 true。
func (p *DynamicProvider) SupportsStream() bool {
	provider, err := p.current()
	if err != nil {
		return false
	}
	return provider.SupportsStream()
}

func (p *DynamicProvider) StreamName(room, identity string) string {
	provider, err := p.current()
	if err != nil {
		return ""
	}
	return provider.StreamName(room, identity)
}

func (p *DynamicProvider) StreamInfo(room, identity string) (stream, token string, err error) {
	provider, err := p.current()
	if err != nil {
		return "", "", err
	}
	return provider.StreamInfo(room, identity)
}

func (p *DynamicProvider) ClientInfo() map[string]interface{} {
	provider, err := p.current()
	if err != nil {
		return map[string]interface{}{}
	}
	return provider.ClientInfo()
}

// Close 释放当前缓存的 provider 资源（若实现 Close），并清空缓存避免 reuse-after-close。
func (p *DynamicProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cachedProvider == nil {
		return nil
	}
	var err error
	if closer, ok := p.cachedProvider.(providerCloser); ok {
		err = closer.Close()
	}
	p.cachedProvider = nil
	p.cachedFingerprint = ""
	p.cachedConfig = nil
	return err
}

func (p *DynamicProvider) current() (sfu.Provider, error) {
	cfg, fp, err := p.resolveCached()
	if err != nil {
		return nil, err
	}

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
	p.cachedProvider = provider
	p.cachedFingerprint = fp
	return provider, nil
}
