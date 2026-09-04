package sfu

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeMuteRuleStore 是测试用 MuteRuleStore 实现（不进入生产代码，杜绝被误用作降级后端）。
type fakeMuteRuleStore struct {
	mu   sync.Mutex
	data map[string]fakeMuteEntry
}

type fakeMuteEntry struct {
	ruleID    int
	expiresAt time.Time
}

func newFakeMuteRuleStore() *fakeMuteRuleStore {
	return &fakeMuteRuleStore{data: map[string]fakeMuteEntry{}}
}

func (s *fakeMuteRuleStore) Backend() string { return "fake" }

func (s *fakeMuteRuleStore) Save(_ context.Context, key string, ruleID int, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ruleID <= 0 {
		delete(s.data, key)
		return nil
	}
	e := fakeMuteEntry{ruleID: ruleID}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	s.data[key] = e
	return nil
}

func (s *fakeMuteRuleStore) Get(_ context.Context, key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[key]
	if !ok {
		return 0, nil
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		delete(s.data, key)
		return 0, nil
	}
	return e.ruleID, nil
}

func (s *fakeMuteRuleStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func TestCachedMuteRuleStore_WriteThrough(t *testing.T) {
	shared := newFakeMuteRuleStore()
	cache := NewCachedMuteRuleStore(shared)
	ctx := context.Background()
	if err := cache.Save(ctx, "a|b", 9, time.Minute); err != nil {
		t.Fatal(err)
	}
	// 写入穿透到 shared。
	id, err := shared.Get(ctx, "a|b")
	if err != nil || id != 9 {
		t.Fatalf("shared id=%d err=%v", id, err)
	}
	// 读取也走 shared（不再有本地 L1 缓存）。
	id, err = cache.Get(ctx, "a|b")
	if err != nil || id != 9 {
		t.Fatalf("cache id=%d err=%v", id, err)
	}
	if cache.Backend() != "fake" {
		t.Fatalf("backend=%s", cache.Backend())
	}
}

func TestCachedMuteRuleStore_GetFreshDelegatesToShared(t *testing.T) {
	shared := newFakeMuteRuleStore()
	cache := NewCachedMuteRuleStore(shared)
	ctx := context.Background()
	if err := cache.Save(ctx, "srs_pub_block:gs-abc", 1, time.Minute); err != nil {
		t.Fatal(err)
	}
	// GetFresh 直接读 shared；shared 删除后立即可见（无 L1 遮挡）。
	if id, err := cache.GetFresh(ctx, "srs_pub_block:gs-abc"); err != nil || id != 1 {
		t.Fatalf("fresh id=%d err=%v", id, err)
	}
	if err := shared.Delete(ctx, "srs_pub_block:gs-abc"); err != nil {
		t.Fatal(err)
	}
	if id, err := cache.GetFresh(ctx, "srs_pub_block:gs-abc"); err != nil || id != 0 {
		t.Fatalf("after delete fresh id=%d err=%v", id, err)
	}
}
