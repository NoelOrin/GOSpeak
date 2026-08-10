package authstate

import (
	"errors"
	"testing"
	"time"

	"GOSpeak/internal/config"
)

var errKeyExists = errors.New("signing key exists")

type fakeBackend struct {
	activeKey string
	createdAt int64
	history   []string
	setCalls  int
	blacklist map[string]time.Time
	families  map[string]bool
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		blacklist: make(map[string]time.Time),
		families:  make(map[string]bool),
	}
}

func (f *fakeBackend) BlacklistToken(jti string, remaining time.Duration) error {
	if jti != "" && remaining > 0 {
		f.blacklist[jti] = time.Now().Add(remaining)
	}
	return nil
}

func (f *fakeBackend) IsBlacklisted(jti string) bool {
	ok, _ := f.IsBlacklistedErr(jti)
	return ok
}

func (f *fakeBackend) IsBlacklistedErr(jti string) (bool, error) {
	exp, ok := f.blacklist[jti]
	return ok && time.Now().Before(exp), nil
}

func (f *fakeBackend) GetSigningKey() (string, bool, error) {
	if f.activeKey == "" {
		return "", false, nil
	}
	return f.activeKey, true, nil
}

func (f *fakeBackend) SetSigningKey(key string, createdAtUnix int64) error {
	f.setCalls++
	if f.activeKey != "" {
		return errKeyExists
	}
	f.activeKey = key
	f.createdAt = createdAtUnix
	return nil
}

func (f *fakeBackend) UpdateSigningKey(key string, createdAtUnix int64) error {
	f.history = append(f.history, f.activeKey)
	f.activeKey = key
	f.createdAt = createdAtUnix
	return nil
}

func (f *fakeBackend) GetCreatedAt() (int64, bool, error) {
	return f.createdAt, f.createdAt != 0, nil
}

func (f *fakeBackend) AddHistoryKey(key string) error {
	for _, k := range f.history {
		if k == key {
			return nil
		}
	}
	f.history = append(f.history, key)
	return nil
}

func (f *fakeBackend) HistoryKeys() []string {
	return append([]string(nil), f.history...)
}

func (f *fakeBackend) MarkRefreshFamilyUsed(family string, _ time.Duration) (bool, error) {
	if f.families[family] {
		return false, nil
	}
	f.families[family] = true
	return true, nil
}

func (f *fakeBackend) IsRefreshFamilyUsed(family string) (bool, error) {
	return f.families[family], nil
}

func (f *fakeBackend) RevokeRefreshFamily(family string) error {
	f.families[family] = true
	return nil
}

func (f *fakeBackend) Backend() string { return "fake" }

func resetAuthState(t *testing.T) {
	t.Helper()
	oldCfg := jwtCfg
	oldProd := production
	oldBackend := currentBackend()
	t.Cleanup(func() {
		keyMu.Lock()
		jwtCfg = oldCfg
		keyMu.Unlock()
		production = oldProd
		SetBackend(oldBackend)
	})
	Configure(&config.Config{JWTKey: "static-key", JWTKeyTTL: "24h"})
	production = false
	SetBackend(nil)
}

func TestGetSigningKey_DevUsesStaticKey(t *testing.T) {
	resetAuthState(t)
	if got := string(GetSigningKey()); got != "static-key" {
		t.Fatalf("dev signing key = %q, want static-key", got)
	}
}

func TestGetSigningKey_ProductionCreatesSharedKey(t *testing.T) {
	resetAuthState(t)
	production = true
	b := newFakeBackend()
	SetBackend(b)

	first := GetSigningKey()
	second := GetSigningKey()
	if string(first) != string(second) {
		t.Fatal("signing key changed between calls")
	}
	if len(first) == 0 {
		t.Fatal("expected a generated signing key")
	}
	if b.setCalls != 1 {
		t.Fatalf("SetSigningKey calls = %d, want 1", b.setCalls)
	}
	if keys := GetAllSigningKeys(); len(keys) == 0 {
		t.Fatal("expected signing keys")
	}
}

func TestRotateSigningKey_KeepsHistory(t *testing.T) {
	resetAuthState(t)
	production = true
	b := newFakeBackend()
	SetBackend(b)

	old := GetSigningKey()
	next := RotateSigningKey()
	if string(old) == string(next) {
		t.Fatal("rotated key must differ")
	}
	if b.activeKey != string(next) {
		t.Fatalf("active key = %q, want %q", b.activeKey, next)
	}
	found := false
	for _, k := range GetAllSigningKeys() {
		if string(k) == string(old) {
			found = true
		}
	}
	if !found {
		t.Fatal("old key missing from history")
	}
}

func TestBlacklistAndRefreshFamily_MemoryFallback(t *testing.T) {
	resetAuthState(t)
	if err := BlacklistToken("jti-1", time.Minute); err != nil {
		t.Fatalf("blacklist: %v", err)
	}
	if IsBlacklisted("jti-1") {
		t.Fatal("memory backend has no blacklist; must stay not blacklisted")
	}
	if revoked, err := IsBlacklistedErr("jti-1"); err != nil || revoked {
		t.Fatalf("IsBlacklistedErr = %v, %v; want false, nil", revoked, err)
	}

	marked, err := MarkRefreshFamilyUsed("fam-1")
	if err != nil || !marked {
		t.Fatalf("first family mark = %v, %v; want true, nil", marked, err)
	}
	marked, err = MarkRefreshFamilyUsed("fam-1")
	if err != nil || marked {
		t.Fatalf("second family mark = %v, %v; want false, nil", marked, err)
	}
	if used, _ := IsRefreshFamilyUsed("fam-1"); !used {
		t.Fatal("family should be marked used")
	}
}

func TestBlacklistAndRefreshFamily_Backend(t *testing.T) {
	resetAuthState(t)
	b := newFakeBackend()
	SetBackend(b)

	if err := BlacklistToken("jti-2", time.Minute); err != nil {
		t.Fatalf("blacklist: %v", err)
	}
	if !IsBlacklisted("jti-2") {
		t.Fatal("expected blacklisted via backend")
	}

	marked, err := MarkRefreshFamilyUsed("fam-2")
	if err != nil || !marked {
		t.Fatalf("first family mark = %v, %v; want true, nil", marked, err)
	}
	marked, err = MarkRefreshFamilyUsed("fam-2")
	if err != nil || marked {
		t.Fatalf("second family mark = %v, %v; want false, nil", marked, err)
	}
}
