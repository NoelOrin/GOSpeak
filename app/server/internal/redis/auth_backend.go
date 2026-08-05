package redis

import "time"

// AuthBackend is optional multi-instance auth state when Redis is unavailable.
// Typically backed by NATS JetStream KV (bus.AuthStore).
type AuthBackend interface {
	BlacklistToken(jti string, remaining time.Duration) error
	// IsBlacklisted 保持 bool 接口，存储错误时按未黑名单放行（fail-open）。
	IsBlacklisted(jti string) bool
	// IsBlacklistedErr 返回底层错误，供安全敏感调用方决定 fail-open/fail-closed。
	IsBlacklistedErr(jti string) (bool, error)
	GetSigningKey() (string, bool, error)
	SetSigningKey(key string, createdAtUnix int64) error
	// UpdateSigningKey 覆盖 active key，用于密钥轮换；
	// SetSigningKey 是 CAS 首启语义，不能覆盖。
	UpdateSigningKey(key string, createdAtUnix int64) error
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
