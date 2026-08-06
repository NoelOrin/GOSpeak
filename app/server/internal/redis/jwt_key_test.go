package redis

import (
	"errors"
	"testing"
	"time"

	"sync"

	"GOSpeak/internal/config"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

type stubAuthBackend struct {
	key               string
	createdAt         int64
	history           []string
	setErr            error
	setCalls          int
	updateCalls       int
	getCalls          int
	updateErr         error
	blacklistErr      error
	blacklistErrCalls int
}

func (s *stubAuthBackend) BlacklistToken(string, time.Duration) error { return nil }
func (s *stubAuthBackend) IsBlacklisted(string) bool                  { return false }
func (s *stubAuthBackend) IsBlacklistedErr(string) (bool, error) {
	s.blacklistErrCalls++
	if s.blacklistErr != nil {
		return false, s.blacklistErr
	}
	return false, nil
}
func (s *stubAuthBackend) GetSigningKey() (string, bool, error) {
	s.getCalls++
	if s.getCalls == 1 && s.setErr != nil {
		return "", false, nil
	}
	if s.key == "" {
		return "", false, nil
	}
	return s.key, true, nil
}
func (s *stubAuthBackend) SetSigningKey(key string, createdAtUnix int64) error {
	s.setCalls++
	if s.setErr != nil {
		return s.setErr
	}
	s.key = key
	s.createdAt = createdAtUnix
	return nil
}
func (s *stubAuthBackend) UpdateSigningKey(key string, createdAtUnix int64) error {
	s.updateCalls++
	if s.updateErr != nil {
		return s.updateErr
	}
	s.key = key
	s.createdAt = createdAtUnix
	return nil
}
func (s *stubAuthBackend) GetCreatedAt() (int64, bool, error) {
	if s.createdAt == 0 {
		return 0, false, nil
	}
	return s.createdAt, true, nil
}
func (s *stubAuthBackend) AddHistoryKey(key string) error {
	s.history = append(s.history, key)
	return nil
}
func (s *stubAuthBackend) HistoryKeys() []string { return append([]string(nil), s.history...) }
func (s *stubAuthBackend) Backend() string       { return "stub" }

func TestGetSigningKey_DevUsesStaticEvenWithAuthBackend(t *testing.T) {
	prevClient := client
	prevAuth := secondaryAuth
	prevProd := production
	prevJwtCfg := jwtCfg
	t.Cleanup(func() {
		client = prevClient
		secondaryAuth = prevAuth
		production = prevProd
		jwtCfg = prevJwtCfg
	})

	client = nil
	production = false
	jwtCfg = &config.Config{JWTKey: "dev-static-key", JWTKeyTTL: "24h"}

	backend := &stubAuthBackend{key: "random-from-nats"}
	SetAuthBackend(backend)

	got := string(GetSigningKey())
	if got != "dev-static-key" {
		t.Fatalf("dev GetSigningKey = %q, want static JWT_KEY", got)
	}

	keys := GetAllSigningKeys()
	if len(keys) != 1 || string(keys[0]) != "dev-static-key" {
		t.Fatalf("dev GetAllSigningKeys = %v, want only static key", keys)
	}

	if ShouldRotateKey() {
		t.Fatal("dev ShouldRotateKey should be false")
	}
	if string(RotateSigningKey()) != "dev-static-key" {
		t.Fatal("dev RotateSigningKey should return static key")
	}
	if backend.key != "random-from-nats" {
		t.Fatalf("dev mode should not mutate auth backend key, got %q", backend.key)
	}
}

func TestGetSigningKey_ProdUsesAuthBackend(t *testing.T) {
	prevClient := client
	prevAuth := secondaryAuth
	prevProd := production
	t.Cleanup(func() {
		client = prevClient
		secondaryAuth = prevAuth
		production = prevProd
		config.SetCurrent(nil)
	})

	client = nil
	production = true
	config.SetCurrent(&config.Config{JWTKey: "prod-static-fallback", JWTKeyTTL: "24h"})

	backend := &stubAuthBackend{key: "shared-prod-key"}
	SetAuthBackend(backend)

	got := string(GetSigningKey())
	if got != "shared-prod-key" {
		t.Fatalf("prod GetSigningKey = %q, want auth backend key", got)
	}
}

func TestGetSigningKey_ProdFirstStartReReadsWinner(t *testing.T) {
	prevClient := client
	prevAuth := secondaryAuth
	prevProd := production
	t.Cleanup(func() {
		client = prevClient
		secondaryAuth = prevAuth
		production = prevProd
		config.SetCurrent(nil)
	})

	client = nil
	production = true
	config.SetCurrent(&config.Config{JWTKey: "prod-static-fallback", JWTKeyTTL: "24h"})

	backend := &stubAuthBackend{key: "winner-key", setErr: errors.New("exists")}
	SetAuthBackend(backend)

	got := string(GetSigningKey())
	if got != "winner-key" {
		t.Fatalf("prod GetSigningKey = %q, want winner key after CAS conflict", got)
	}
	if backend.setCalls != 1 {
		t.Fatalf("SetSigningKey called %d times, want 1", backend.setCalls)
	}
}

func TestRotateSigningKey_UsesUpdateSigningKey(t *testing.T) {
	prevClient := client
	prevAuth := secondaryAuth
	prevProd := production
	t.Cleanup(func() {
		client = prevClient
		secondaryAuth = prevAuth
		production = prevProd
		config.SetCurrent(nil)
	})

	client = nil
	production = true
	config.SetCurrent(&config.Config{JWTKey: "prod-static-fallback", JWTKeyTTL: "24h"})

	backend := &stubAuthBackend{key: "old-key", createdAt: 1}
	SetAuthBackend(backend)

	got := string(RotateSigningKey())
	if got == "old-key" {
		t.Fatalf("RotateSigningKey = %q, want rotated key", got)
	}
	if backend.updateCalls != 1 {
		t.Fatalf("UpdateSigningKey called %d times, want 1", backend.updateCalls)
	}
	if backend.setCalls != 0 {
		t.Fatalf("SetSigningKey called %d times, want 0", backend.setCalls)
	}
}

func TestGetSigningKey_ConcurrentFirstUseCreatesOneKey(t *testing.T) {
	prevClient := client
	prevAuth := secondaryAuth
	prevProd := production
	t.Cleanup(func() {
		client = prevClient
		secondaryAuth = prevAuth
		production = prevProd
		config.SetCurrent(nil)
	})

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	client = goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	production = true
	config.SetCurrent(&config.Config{JWTKeyTTL: "24h"})

	const goroutines = 16
	keys := make(chan string, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			keys <- string(GetSigningKey())
		}()
	}
	wg.Wait()
	close(keys)

	first := ""
	for k := range keys {
		if first == "" {
			first = k
		}
		if k != first {
			t.Fatalf("concurrent GetSigningKey returned different keys")
		}
	}
	history, err := mr.SMembers("jwt:signing_key:history")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("first-use key created %d times, want 1", len(history))
	}
}

func TestRotateSigningKey_KeepsOldKeyOnUpdateFailure(t *testing.T) {
	prevClient := client
	prevAuth := secondaryAuth
	prevProd := production
	t.Cleanup(func() {
		client = prevClient
		secondaryAuth = prevAuth
		production = prevProd
		config.SetCurrent(nil)
	})

	client = nil
	production = true
	config.SetCurrent(&config.Config{JWTKey: "prod-static-fallback", JWTKeyTTL: "24h"})

	backend := &stubAuthBackend{key: "old-key", createdAt: 1, updateErr: errors.New("kv down")}
	SetAuthBackend(backend)

	got := string(RotateSigningKey())
	if got != "old-key" {
		t.Fatalf("RotateSigningKey = %q, want old key after update failure", got)
	}
	if backend.key != "old-key" {
		t.Fatalf("active key changed to %q after failed update", backend.key)
	}
	if backend.createdAt != 1 {
		t.Fatalf("createdAt changed to %d after failed update", backend.createdAt)
	}
	if backend.updateCalls != 1 {
		t.Fatalf("UpdateSigningKey called %d times, want 1", backend.updateCalls)
	}
}

func TestIsBlacklisted_SecondaryBackendErrorIsFailOpen(t *testing.T) {
	prevClient := client
	prevAuth := secondaryAuth
	t.Cleanup(func() {
		client = prevClient
		secondaryAuth = prevAuth
	})

	client = nil
	backend := &stubAuthBackend{blacklistErr: errors.New("kv down")}
	SetAuthBackend(backend)

	if IsBlacklisted("jti") {
		t.Fatal("IsBlacklisted must return false when secondary backend errors (fail-open)")
	}
	if backend.blacklistErrCalls != 1 {
		t.Fatalf("IsBlacklistedErr called %d times, want 1", backend.blacklistErrCalls)
	}
}
