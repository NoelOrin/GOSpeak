package sfu

import (
	"context"
	"sync"
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
//   - Backend() reports "memory" | "redis" | "nats".
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

type memoryRuleEntry struct {
	ruleID    int
	expiresAt time.Time // zero = no expiry
}

// MemoryMuteRuleStore is process-local only. Multi-instance still weak.
type MemoryMuteRuleStore struct {
	mu   sync.Mutex
	data map[string]memoryRuleEntry
}

func NewMemoryMuteRuleStore() *MemoryMuteRuleStore {
	return &MemoryMuteRuleStore{data: make(map[string]memoryRuleEntry)}
}

func (s *MemoryMuteRuleStore) Backend() string { return "memory" }

func (s *MemoryMuteRuleStore) Save(_ context.Context, key string, ruleID int, ttl time.Duration) error {
	if s == nil || key == "" || ruleID <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]memoryRuleEntry)
	}
	entry := memoryRuleEntry{ruleID: ruleID}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	s.data[key] = entry
	return nil
}

func (s *MemoryMuteRuleStore) Get(_ context.Context, key string) (int, error) {
	if s == nil || key == "" {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.data[key]
	if !ok {
		return 0, nil
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		delete(s.data, key)
		return 0, nil
	}
	return entry.ruleID, nil
}

func (s *MemoryMuteRuleStore) Delete(_ context.Context, key string) error {
	if s == nil || key == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

// CachedMuteRuleStore is L1 memory + shared L2 (redis/nats).
// Write-through on Save/Delete; Get fills L1 from L2 on miss.
type CachedMuteRuleStore struct {
	local  *MemoryMuteRuleStore
	shared MuteRuleStore
}

func NewCachedMuteRuleStore(shared MuteRuleStore) *CachedMuteRuleStore {
	if shared == nil {
		return &CachedMuteRuleStore{local: NewMemoryMuteRuleStore()}
	}
	return &CachedMuteRuleStore{
		local:  NewMemoryMuteRuleStore(),
		shared: shared,
	}
}

func (s *CachedMuteRuleStore) Backend() string {
	if s == nil {
		return "memory"
	}
	if s.shared != nil {
		return s.shared.Backend()
	}
	return "memory"
}

func (s *CachedMuteRuleStore) Save(ctx context.Context, key string, ruleID int, ttl time.Duration) error {
	if s == nil {
		return nil
	}
	_ = s.local.Save(ctx, key, ruleID, ttl)
	if s.shared != nil {
		return s.shared.Save(ctx, key, ruleID, ttl)
	}
	return nil
}

func (s *CachedMuteRuleStore) Get(ctx context.Context, key string) (int, error) {
	if s == nil {
		return 0, nil
	}
	if id, err := s.local.Get(ctx, key); err != nil {
		return 0, err
	} else if id > 0 {
		return id, nil
	}
	if s.shared == nil {
		return 0, nil
	}
	id, err := s.shared.Get(ctx, key)
	if err != nil || id <= 0 {
		return id, err
	}
	// Best-effort L1 fill with short TTL so stale rule IDs expire quickly.
	_ = s.local.Save(ctx, key, id, cachedMuteRuleL1TTL)
	return id, nil
}

func (s *CachedMuteRuleStore) Delete(ctx context.Context, key string) error {
	if s == nil {
		return nil
	}
	_ = s.local.Delete(ctx, key)
	if s.shared != nil {
		return s.shared.Delete(ctx, key)
	}
	return nil
}
