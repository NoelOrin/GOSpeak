// Package authstate manages shared JWT auth state: token blacklist,
// refresh-family replay protection, and signing-key rotation. It has no
// dependency on Redis; multi-instance state is provided by an injectable
// Backend (typically bus.AuthStore over NATS JetStream KV) and degrades to
// process-local memory when no backend is configured.
package authstate

import (
	"sync"
	"time"
)

// Backend stores multi-instance auth state.
type Backend interface {
	BlacklistToken(jti string, remaining time.Duration) error
	// IsBlacklisted keeps the bool interface and treats storage errors as not
	// blacklisted (fail-open).
	IsBlacklisted(jti string) bool
	// IsBlacklistedErr returns the underlying error so security-sensitive
	// callers can decide between fail-open and fail-closed.
	IsBlacklistedErr(jti string) (bool, error)
	GetSigningKey() (string, bool, error)
	// SetSigningKey uses create-only semantics; a key that already exists is
	// rejected.
	SetSigningKey(key string, createdAtUnix int64) error
	// UpdateSigningKey overwrites the active key during rotation.
	UpdateSigningKey(key string, createdAtUnix int64) error
	GetCreatedAt() (int64, bool, error)
	AddHistoryKey(key string) error
	HistoryKeys() []string
	MarkRefreshFamilyUsed(family string, ttl time.Duration) (bool, error)
	IsRefreshFamilyUsed(family string) (bool, error)
	RevokeRefreshFamily(family string) error
	Backend() string
}

var (
	backendMu sync.RWMutex
	backend   Backend
)

// SetBackend injects the shared auth store (e.g. NATS KV). A nil value
// disables multi-instance auth state.
func SetBackend(b Backend) {
	backendMu.Lock()
	backend = b
	backendMu.Unlock()
}

func currentBackend() Backend {
	backendMu.RLock()
	defer backendMu.RUnlock()
	return backend
}

// BackendName returns the active backend name, or "" when none is configured.
func BackendName() string {
	if b := currentBackend(); b != nil {
		return b.Backend()
	}
	return ""
}
