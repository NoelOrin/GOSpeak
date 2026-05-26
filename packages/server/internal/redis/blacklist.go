package redis

import (
	"context"
	"time"
)

const blacklistPrefix = "jwt:blacklist:"

// BlacklistToken marks a JTI as revoked for the duration of its remaining
// lifetime. No-op when Redis is not connected (logout is best-effort).
func BlacklistToken(jti string, remaining time.Duration) error {
	if Client == nil || jti == "" || remaining <= 0 {
		return nil
	}
	ctx := context.Background()
	return Client.Set(ctx, blacklistPrefix+jti, "1", remaining).Err()
}

// IsBlacklisted reports whether the given JTI has been revoked.
// Returns false when Redis is not connected.
func IsBlacklisted(jti string) bool {
	if Client == nil || jti == "" {
		return false
	}
	ctx := context.Background()
	n, err := Client.Exists(ctx, blacklistPrefix+jti).Result()
	return err == nil && n > 0
}
