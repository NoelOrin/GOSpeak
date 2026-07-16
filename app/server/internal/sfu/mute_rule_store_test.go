package sfu

import (
	"context"
	"testing"
	"time"
)

func TestMemoryMuteRuleStore_SaveGetDelete(t *testing.T) {
	store := NewMemoryMuteRuleStore()
	ctx := context.Background()
	if err := store.Save(ctx, "r|u", 42, time.Minute); err != nil {
		t.Fatal(err)
	}
	id, err := store.Get(ctx, "r|u")
	if err != nil || id != 42 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if err := store.Delete(ctx, "r|u"); err != nil {
		t.Fatal(err)
	}
	id, err = store.Get(ctx, "r|u")
	if err != nil || id != 0 {
		t.Fatalf("after delete id=%d err=%v", id, err)
	}
}

func TestMemoryMuteRuleStore_Expiry(t *testing.T) {
	store := NewMemoryMuteRuleStore()
	ctx := context.Background()
	if err := store.Save(ctx, "k", 7, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	id, err := store.Get(ctx, "k")
	if err != nil || id != 0 {
		t.Fatalf("expected expired miss, id=%d err=%v", id, err)
	}
}

func TestCachedMuteRuleStore_WriteThrough(t *testing.T) {
	shared := NewMemoryMuteRuleStore()
	cache := NewCachedMuteRuleStore(shared)
	ctx := context.Background()
	if err := cache.Save(ctx, "a|b", 9, time.Minute); err != nil {
		t.Fatal(err)
	}
	// Shared should see it even if local is cleared.
	cache.local = NewMemoryMuteRuleStore()
	id, err := cache.Get(ctx, "a|b")
	if err != nil || id != 9 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if cache.Backend() != "memory" {
		// shared is memory, Backend reports shared
		t.Fatalf("backend=%s", cache.Backend())
	}
}
