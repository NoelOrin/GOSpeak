package bus

import (
	"testing"
	"time"
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
