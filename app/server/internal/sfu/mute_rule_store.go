package sfu

import (
	"context"
	"sync"
	"time"

	"GOSpeak/internal/logger"
)

// cachedMuteRuleL1TTL 控制 L1 内存回填的过期时间；其他实例重新禁言产生新 ruleID 后，
// 旧实例 L1 最多再命中一次短窗口，避免 unmute 删错 rule 导致新禁言无法解除。
const cachedMuteRuleL1TTL = 30 * time.Second

// MuteRuleStore persists provider-side mute rule identifiers across instances.
// Used by degraded mute backends (e.g. Agora kicking-rule ids) so unmute can
// delete the same rule on any process.
//
// Semantics:
//   - Save stores ruleID with best-effort TTL (backend may coarsen TTL).
//   - Get returns 0 when missing / backend unavailable (callers treat as miss).
//   - Delete is idempotent.
//   - Backend() reports "memory" | "nats" | "none".
type MuteRuleStore interface {
	Save(ctx context.Context, key string, ruleID int, ttl time.Duration) error
	Get(ctx context.Context, key string) (int, error)
	Delete(ctx context.Context, key string) error
	Backend() string
}

// MuteRuleStoreSetter is implemented by providers that need multi-instance rule cache.
type MuteRuleStoreSetter interface {
	SetMuteRuleStore(store MuteRuleStore)
}

// FreshMuteRuleStore 由需要"以 shared 为权威、跳过 L1"的消费者实现
// （如 SRS 禁推黑名单：ruleID 恒为 1，L1 只会放大跨实例解禁延迟）。
type FreshMuteRuleStore interface {
	MuteRuleStore
	GetFresh(ctx context.Context, key string) (int, error)
}

type l1Entry struct {
	ruleID    int
	expiresAt time.Time
}

// CachedMuteRuleStore is a wrapper that adds FreshMuteRuleStore capability and
// a short-TTL L1 cache over a shared backend (NATS KV). Get uses L1 to reduce
// KV pressure; GetFresh bypasses L1 and always reads the authoritative shared
// value (required for SRS ruleID=1 where L1 would amplify unmute delay on NATS jitter).
// When shared==nil it reports Backend()="none" and warns instead of silently degrading to "memory".
type CachedMuteRuleStore struct {
	shared MuteRuleStore
	mu     sync.RWMutex
	l1     map[string]l1Entry
}

func NewCachedMuteRuleStore(shared MuteRuleStore) *CachedMuteRuleStore {
	return &CachedMuteRuleStore{shared: shared, l1: make(map[string]l1Entry)}
}

func (s *CachedMuteRuleStore) Backend() string {
	if s == nil || s.shared == nil {
		logger.WithComponent("SFU").Warnf("muteRuleStore backend unavailable (shared==nil), reporting none")
		return "none"
	}
	return s.shared.Backend()
}

func (s *CachedMuteRuleStore) Save(ctx context.Context, key string, ruleID int, ttl time.Duration) error {
	if s == nil || s.shared == nil {
		logger.WithComponent("SFU").Warnf("muteRuleStore Save dropped: shared==nil key=%s", key)
		return nil
	}
	err := s.shared.Save(ctx, key, ruleID, ttl)
	if err == nil {
		exp := time.Now().Add(cachedMuteRuleL1TTL)
		if ttl > 0 && ttl < cachedMuteRuleL1TTL {
			exp = time.Now().Add(ttl)
		}
		s.mu.Lock()
		s.l1[key] = l1Entry{ruleID: ruleID, expiresAt: exp}
		s.mu.Unlock()
	}
	return err
}

func (s *CachedMuteRuleStore) Get(ctx context.Context, key string) (int, error) {
	if s == nil || s.shared == nil {
		return 0, nil
	}
	s.mu.RLock()
	if e, ok := s.l1[key]; ok && time.Now().Before(e.expiresAt) {
		s.mu.RUnlock()
		return e.ruleID, nil
	}
	s.mu.RUnlock()
	id, err := s.shared.Get(ctx, key)
	if err == nil && id != 0 {
		s.mu.Lock()
		s.l1[key] = l1Entry{ruleID: id, expiresAt: time.Now().Add(cachedMuteRuleL1TTL)}
		s.mu.Unlock()
	}
	return id, err
}

// GetFresh reads the authoritative shared value (no local cache to bypass).
// It implements sfu.FreshMuteRuleStore.
func (s *CachedMuteRuleStore) GetFresh(ctx context.Context, key string) (int, error) {
	if s == nil || s.shared == nil {
		return 0, nil
	}
	return s.shared.Get(ctx, key)
}

func (s *CachedMuteRuleStore) Delete(ctx context.Context, key string) error {
	if s == nil || s.shared == nil {
		return nil
	}
	s.mu.Lock()
	delete(s.l1, key)
	s.mu.Unlock()
	return s.shared.Delete(ctx, key)
}
