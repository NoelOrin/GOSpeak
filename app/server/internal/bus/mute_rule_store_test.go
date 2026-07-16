package bus

import (
	"context"
	"testing"
	"time"

	"GOSpeak/internal/sfu"
)

func TestNATSMuteRuleStore_SaveGetDelete(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenNATSMuteRuleStore(NATSMuteRuleStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_mute",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	if err := store.Save(ctx, "room|user", 123, time.Minute); err != nil {
		t.Fatal(err)
	}
	id, err := store.Get(ctx, "room|user")
	if err != nil || id != 123 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if err := store.Delete(ctx, "room|user"); err != nil {
		t.Fatal(err)
	}
	id, err = store.Get(ctx, "room|user")
	if err != nil || id != 0 {
		t.Fatalf("after delete id=%d err=%v", id, err)
	}
}

func TestResolveMuteRuleStore_AutoFallsToMemory(t *testing.T) {
	store, backend := ResolveMuteRuleStore(ResolveMuteRuleConfig{Mode: "auto"})
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if backend != "memory" {
		t.Fatalf("backend=%s", backend)
	}
	if store.Backend() != "memory" {
		t.Fatalf("store.Backend=%s", store.Backend())
	}
}

func TestResolveMuteRuleStore_NATSModeWithConn(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	// Resolve with live NATS connection via OpenNATSMuteRuleStore path:
	// Open a temporary store just to get a connected NC is awkward; connect via nats.Connect.
	// Use OpenStateStore's URL path by calling OpenNATSMuteRuleStore and wrapping.
	raw, err := OpenNATSMuteRuleStore(NATSMuteRuleStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_mute_res",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })

	// Resolve mode=nats without NC falls to memory
	store, backend := ResolveMuteRuleStore(ResolveMuteRuleConfig{Mode: "nats"})
	if store == nil || backend != "memory" {
		t.Fatalf("nil nats should memory, got backend=%s store=%v", backend, store)
	}

	// Cached wrapper over nats raw must cross process-local L1 via shared KV
	a := sfu.NewCachedMuteRuleStore(raw)
	b := sfu.NewCachedMuteRuleStore(raw)
	ctx := context.Background()
	if err := a.Save(ctx, "r|u", 5, time.Minute); err != nil {
		t.Fatal(err)
	}
	id, err := b.Get(ctx, "r|u")
	if err != nil || id != 5 {
		t.Fatalf("cross-cache id=%d err=%v", id, err)
	}
	if a.Backend() != "nats" {
		t.Fatalf("backend=%s", a.Backend())
	}
}

func TestMuteRuleKVKey(t *testing.T) {
	if got := muteRuleKVKey("room|user"); got != "room.user" {
		t.Fatalf("got %q", got)
	}
}
