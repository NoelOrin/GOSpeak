package redis

import "time"

// AuthBackend is optional multi-instance auth state when Redis is unavailable.
// Typically backed by NATS JetStream KV (bus.AuthStore).
type AuthBackend interface {
	BlacklistToken(jti string, remaining time.Duration) error
	IsBlacklisted(jti string) bool
	GetSigningKey() (string, bool, error)
	SetSigningKey(key string, createdAtUnix int64) error
	GetCreatedAt() (int64, bool, error)
	AddHistoryKey(key string) error
	HistoryKeys() []string
	Backend() string
}

// secondaryAuth is used when Redis Client is nil.
var secondaryAuth AuthBackend

// SetAuthBackend injects NATS (or other) auth store fallback.
func SetAuthBackend(b AuthBackend) {
	secondaryAuth = b
}

// AuthBackendName returns active secondary backend or empty.
func AuthBackendName() string {
	if secondaryAuth == nil {
		return ""
	}
	return secondaryAuth.Backend()
}
