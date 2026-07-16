package redis

import (
	"testing"
	"time"

	"GOSpeak/internal/config"
)

type stubAuthBackend struct {
	key       string
	createdAt int64
	history   []string
}

func (s *stubAuthBackend) BlacklistToken(string, time.Duration) error { return nil }
func (s *stubAuthBackend) IsBlacklisted(string) bool                  { return false }
func (s *stubAuthBackend) GetSigningKey() (string, bool, error) {
	if s.key == "" {
		return "", false, nil
	}
	return s.key, true, nil
}
func (s *stubAuthBackend) SetSigningKey(key string, createdAtUnix int64) error {
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
func (s *stubAuthBackend) Backend() string      { return "stub" }

func TestGetSigningKey_DevUsesStaticEvenWithAuthBackend(t *testing.T) {
	prevClient := Client
	prevAuth := secondaryAuth
	prevProd := production
	t.Cleanup(func() {
		Client = prevClient
		secondaryAuth = prevAuth
		production = prevProd
		config.SetCurrent(nil)
	})

	Client = nil
	production = false
	config.SetCurrent(&config.Config{JWTKey: "dev-static-key", JWTKeyTTL: "24h"})

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
	prevClient := Client
	prevAuth := secondaryAuth
	prevProd := production
	t.Cleanup(func() {
		Client = prevClient
		secondaryAuth = prevAuth
		production = prevProd
		config.SetCurrent(nil)
	})

	Client = nil
	production = true
	config.SetCurrent(&config.Config{JWTKey: "prod-static-fallback", JWTKeyTTL: "24h"})

	backend := &stubAuthBackend{key: "shared-prod-key"}
	SetAuthBackend(backend)

	got := string(GetSigningKey())
	if got != "shared-prod-key" {
		t.Fatalf("prod GetSigningKey = %q, want auth backend key", got)
	}
}
