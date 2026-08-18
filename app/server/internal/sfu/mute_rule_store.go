package sfu

import (
	"context"
	"time"
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
//   - Backend() reports "memory" | "nats".
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

// CachedMuteRuleStore is a thin wrapper that adds the FreshMuteRuleStore
// (GetFresh) capability over a shared backend (always NATS KV in production).
// It deliberately holds no process-local memory state: a missing backend is a
// programming error, never a silent in-memory degradation.
type CachedMuteRuleStore struct {
	shared MuteRuleStore
}

func NewCachedMuteRuleStore(shared MuteRuleStore) *CachedMuteRuleStore {
	return &CachedMuteRuleStore{shared: shared}
}

func (s *CachedMuteRuleStore) Backend() string {
	if s == nil || s.shared == nil {
		return "memory"
	}
	return s.shared.Backend()
}

func (s *CachedMuteRuleStore) Save(ctx context.Context, key string, ruleID int, ttl time.Duration) error {
	if s == nil || s.shared == nil {
		return nil
	}
	return s.shared.Save(ctx, key, ruleID, ttl)
}

func (s *CachedMuteRuleStore) Get(ctx context.Context, key string) (int, error) {
	if s == nil || s.shared == nil {
		return 0, nil
	}
	return s.shared.Get(ctx, key)
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
	return s.shared.Delete(ctx, key)
}
