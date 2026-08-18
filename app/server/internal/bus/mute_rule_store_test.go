package bus

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"GOSpeak/internal/sfu"

	"github.com/nats-io/nats.go"
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

func TestNATSMuteRuleStore_DefaultBucketTTL(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenNATSMuteRuleStore(NATSMuteRuleStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_mute_default_ttl",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	status, err := store.kv.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.TTL() != 24*time.Hour {
		t.Fatalf("default bucket TTL=%s, want 24h physical cap", status.TTL())
	}
}

func TestNATSMuteRuleStore_PerKeyTTL(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenNATSMuteRuleStore(NATSMuteRuleStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_mute_ttl",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	key := "room|user"
	kvKey := muteRuleKVKey(key)

	// 短 TTL：Save 后未过期可读，过期后按 miss 返回（不依赖 bucket TTL 兜底）。
	shortKey := "room|short"
	if err := store.Save(ctx, shortKey, 11, 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if id, err := store.Get(ctx, shortKey); err != nil || id != 11 {
		t.Fatalf("short before expiry id=%d err=%v, want 11", id, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		id, err := store.Get(ctx, shortKey)
		if err != nil {
			t.Fatalf("short after expiry err=%v", err)
		}
		if id == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("short after expiry id=%d, want 0 within 2s", id)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 正向：带 TTL 保存后未过期可读，且 value 编码为 ruleID:unixExpires（毫秒）。
	if err := store.Save(ctx, key, 1, time.Hour); err != nil {
		t.Fatal(err)
	}
	if id, err := store.Get(ctx, key); err != nil || id != 1 {
		t.Fatalf("before expiry id=%d err=%v, want 1", id, err)
	}
	entry, err := store.kv.Get(kvKey)
	if err != nil {
		t.Fatalf("kv.Get after save: %v", err)
	}
	got := string(entry.Value())
	if !strings.HasPrefix(got, "1:") {
		t.Fatalf("value=%q, want ruleID:unixExpires prefix", got)
	}
	expiresAt, err := strconv.ParseInt(strings.TrimPrefix(got, "1:"), 10, 64)
	if err != nil || expiresAt < 1_000_000_000_000 || expiresAt <= time.Now().UnixMilli() {
		t.Fatalf("value=%q expiresAt=%d err=%v, want future unix millis", got, expiresAt, err)
	}

	// 新格式毫秒过期值：Get 返回 0，后续 Save 可覆盖成新规则。
	if _, err := store.kv.Put(kvKey, []byte(fmt.Sprintf("1:%d", time.Now().Add(-time.Minute).UnixMilli()))); err != nil {
		t.Fatal(err)
	}
	if id, err := store.Get(ctx, key); err != nil || id != 0 {
		t.Fatalf("after expiry id=%d err=%v, want 0", id, err)
	}
	// 过期值可被后续 Save 覆盖成新规则：重新 Save 后 Get 返回新 ruleID。
	if err := store.Save(ctx, key, 2, 0); err != nil {
		t.Fatal(err)
	}
	if id, err := store.Get(ctx, key); err != nil || id != 2 {
		t.Fatalf("after overwrite id=%d err=%v, want 2", id, err)
	}

	// 旧格式秒级过期值按秒解释：未来秒值归一化为毫秒后仍可读。
	futureLegacySeconds := time.Now().Add(time.Hour).Unix()
	if _, err := store.kv.Put(kvKey, []byte(fmt.Sprintf("7:%d", futureLegacySeconds))); err != nil {
		t.Fatal(err)
	}
	if id, err := store.Get(ctx, key); err != nil || id != 7 {
		t.Fatalf("legacy future seconds id=%d err=%v, want 7", id, err)
	}

	// 旧格式无过期后缀，永不过期。
	if _, err := store.kv.Put(kvKey, []byte("42")); err != nil {
		t.Fatal(err)
	}
	if id, err := store.Get(ctx, key); err != nil || id != 42 {
		t.Fatalf("legacy format id=%d err=%v, want 42", id, err)
	}

	// 损坏值视为无规则，不返回错误。
	for _, bad := range []string{"bad", "1:not-a-number", "1:0", "1:-1", "-1", "0:123"} {
		if _, err := store.kv.Put(kvKey, []byte(bad)); err != nil {
			t.Fatal(err)
		}
		if id, err := store.Get(ctx, key); err != nil || id != 0 {
			t.Fatalf("bad value %q id=%d err=%v, want 0", bad, id, err)
		}
	}
}

func TestParseMuteRuleValue_RejectsNonPositiveRuleID(t *testing.T) {
	for _, value := range []string{"-1", "0", "0:123"} {
		id, expiresAt, err := parseMuteRuleValue(value)
		if err == nil {
			t.Fatalf("parseMuteRuleValue(%q) id=%d expiresAt=%d, want error", value, id, expiresAt)
		}
	}
}

func TestNATSMuteRuleStore_PermanentRuleWithDisabledBucketTTL(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	zeroBucketTTL := time.Duration(0)
	cfg := NATSMuteRuleStoreConfig{
		URL:       es.ClientURL(),
		Prefix:    "gospeak_test_mute_perm",
		BucketTTL: &zeroBucketTTL,
	}
	store, err := OpenNATSMuteRuleStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	status, err := store.kv.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.TTL() != 0 {
		t.Fatalf("explicit bucket TTL=%s, want 0 (per-key TTL only)", status.TTL())
	}

	ctx := context.Background()
	key := "room|perm"
	if err := store.Save(ctx, key, 9, 0); err != nil {
		t.Fatal(err)
	}
	entry, err := store.kv.Get(muteRuleKVKey(key))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(entry.Value()); got != "9" {
		t.Fatalf("permanent value=%q, want bare ruleID", got)
	}

	// 同一 bucket 由另一 store 实例读取，验证永久规则在无 bucket TTL 下持久。
	store2, err := OpenNATSMuteRuleStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store2.Close() })
	if id, err := store2.Get(ctx, key); err != nil || id != 9 {
		t.Fatalf("permanent cross-store id=%d err=%v, want 9", id, err)
	}
}


func TestResolveMuteRuleStore_NATSModeWithConn(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	raw, err := OpenNATSMuteRuleStore(NATSMuteRuleStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_mute_res",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })

	// mode=nats with a live NATS connection resolves to the nats backend.
	store, backend, err := ResolveMuteRuleStore(ResolveMuteRuleConfig{
		Mode:   "nats",
		Prefix: "gospeak_test_mute_res",
		NATS:   raw.nc,
	})
	if err != nil || store == nil || backend != "nats" {
		t.Fatalf("resolve mode=nats with NC: backend=%s store=%v err=%v, want nats", backend, store, err)
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

// ResolveMuteRuleStore must fail when NATS is unavailable instead of silently
// degrading to an in-memory store.
func TestResolveMuteRuleStore_RequiresNATS(t *testing.T) {
	if _, _, err := ResolveMuteRuleStore(ResolveMuteRuleConfig{Mode: "auto"}); err == nil {
		t.Fatal("expected error when NATS unavailable (auto)")
	}
	if _, _, err := ResolveMuteRuleStore(ResolveMuteRuleConfig{Mode: "nats"}); err == nil {
		t.Fatal("expected error when NATS unavailable (nats)")
	}
	if _, _, err := ResolveMuteRuleStore(ResolveMuteRuleConfig{Mode: "none"}); err == nil {
		t.Fatal("expected error for forbidden mode none")
	}
	if _, _, err := ResolveMuteRuleStore(ResolveMuteRuleConfig{Mode: "memory"}); err == nil {
		t.Fatal("expected error for forbidden mode memory")
	}
}

func TestResolveMuteRuleStore_PassesBucketTTLToNATS(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	nc, err := nats.Connect(es.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)

	zeroBucketTTL := time.Duration(0)
	prefix := "gospeak_test_mute_res_bucket_ttl"
	store, backend, err := ResolveMuteRuleStore(ResolveMuteRuleConfig{
		Mode:      "nats",
		Prefix:    prefix,
		NATS:      nc,
		BucketTTL: &zeroBucketTTL,
	})
	if err != nil || store == nil || backend != "nats" {
		t.Fatalf("resolve backend=%s store=%v, want nats", backend, store)
	}

	probe, err := OpenNATSMuteRuleStore(NATSMuteRuleStoreConfig{
		URL:    es.ClientURL(),
		Prefix: prefix,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = probe.Close() })
	status, err := probe.kv.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.TTL() != 0 {
		t.Fatalf("resolved bucket TTL=%s, want 0 (BucketTTL must reach NATS store)", status.TTL())
	}
}

func TestMuteRuleKVKey(t *testing.T) {
	if got := muteRuleKVKey("room|user"); got != "room.user" {
		t.Fatalf("got %q", got)
	}
}
