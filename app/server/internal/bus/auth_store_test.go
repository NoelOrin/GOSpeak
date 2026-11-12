package bus

import (
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestAuthStore_BlacklistAndSigningKey(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenAuthStore(AuthStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_auth",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.BlacklistToken("jti-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if !store.IsBlacklisted("jti-1") {
		t.Fatal("expected blacklisted")
	}
	if store.IsBlacklisted("missing") {
		t.Fatal("missing should not blacklist")
	}

	if err := store.SetSigningKey("key-a", time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	_ = store.AddHistoryKey("key-a")
	val, ok, err := store.GetSigningKey()
	if err != nil || !ok || val != "key-a" {
		t.Fatalf("signing key=%q ok=%v err=%v", val, ok, err)
	}
	if hist := store.HistoryKeys(); len(hist) == 0 || hist[0] != "key-a" {
		t.Fatalf("history=%v", hist)
	}
}

func TestAuthStore_SigningKeyRecordAndLegacyCompat(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenAuthStore(AuthStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_auth_legacy",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.SetSigningKey("key-new", 1234567890); err != nil {
		t.Fatal(err)
	}
	val, ok, err := store.GetSigningKey()
	if err != nil || !ok || val != "key-new" {
		t.Fatalf("signing key=%q ok=%v err=%v", val, ok, err)
	}
	created, ok, err := store.GetCreatedAt()
	if err != nil || !ok || created != 1234567890 {
		t.Fatalf("created=%d ok=%v err=%v", created, ok, err)
	}

	// 旧格式兼容：纯字符串 key + 独立 created_at
	legacy := &AuthStore{kv: store.kv}
	_ = legacy.put("jwt.active", "key-legacy")
	_ = legacy.put("jwt.created_at", "111")
	val, ok, err = store.GetSigningKey()
	if err != nil || !ok || val != "key-legacy" {
		t.Fatalf("legacy key=%q ok=%v err=%v", val, ok, err)
	}
	created, ok, err = store.GetCreatedAt()
	if err != nil || !ok || created != 111 {
		t.Fatalf("legacy created=%d ok=%v err=%v", created, ok, err)
	}
}

func newTestAuthStore(t *testing.T) *AuthStore {
	t.Helper()
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenAuthStore(AuthStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_auth_cas",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

var errBrokenKV = errors.New("broken kv")

// brokenKV 仅覆盖测试路径使用的 Get，其余方法通过嵌入接口占位。
type brokenKV struct {
	nats.KeyValue
}

func (brokenKV) Get(string) (nats.KeyValueEntry, error) {
	return nil, errBrokenKV
}

func TestAuthStore_SetSigningKeyCAS(t *testing.T) {
	store := newTestAuthStore(t)
	if err := store.SetSigningKey("k1", 1); err != nil {
		t.Fatalf("first set: %v", err)
	}
	if err := store.SetSigningKey("k2", 2); err != ErrSigningKeyExists {
		t.Fatalf("expected ErrSigningKeyExists, got %v", err)
	}
	val, ok, err := store.GetSigningKey()
	if err != nil || !ok || val != "k1" {
		t.Fatalf("signing key=%q ok=%v err=%v, want k1 not overwritten", val, ok, err)
	}
}

func TestAuthStore_UpdateSigningKeyOverwrites(t *testing.T) {
	store := newTestAuthStore(t)
	if err := store.SetSigningKey("k1", 1); err != nil {
		t.Fatalf("first set: %v", err)
	}
	if err := store.UpdateSigningKey("k2", 2); err != nil {
		t.Fatalf("update: %v", err)
	}
	val, ok, err := store.GetSigningKey()
	if err != nil || !ok || val != "k2" {
		t.Fatalf("signing key=%q ok=%v err=%v", val, ok, err)
	}
	created, ok, err := store.GetCreatedAt()
	if err != nil || !ok || created != 2 {
		t.Fatalf("created=%d ok=%v err=%v", created, ok, err)
	}
}

func TestAuthStore_IsBlacklistedErrPropagates(t *testing.T) {
	store := &AuthStore{kv: brokenKV{}}
	if store.IsBlacklisted("jti") {
		t.Fatal("broken store fail-open must return false")
	}
	if _, err := store.IsBlacklistedErr("jti"); err == nil {
		t.Fatal("expected error from broken store")
	}
}
