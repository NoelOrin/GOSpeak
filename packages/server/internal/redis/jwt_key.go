package redis

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"time"
)

const jwtKeyRedisKey = "jwt:signing_key"

// GetOrRotateSigningKey returns the active JWT signing key.
// When Redis is connected the key is stored with JWT_KEY_TTL TTL; once the TTL
// expires a new random key is generated and stored, automatically invalidating
// all tokens that were signed with the old key.
// When Redis is not connected the static JWT_KEY env var is used as a fallback.
func GetOrRotateSigningKey() []byte {
	if Client == nil {
		return staticKey()
	}

	ctx := context.Background()
	val, err := Client.Get(ctx, jwtKeyRedisKey).Result()
	if err == nil {
		return []byte(val)
	}

	// Key missing or expired: generate a new random key and store it.
	newKey := randomKey()
	ttl := keyTTL()
	if setErr := Client.Set(ctx, jwtKeyRedisKey, newKey, ttl).Err(); setErr != nil {
		fmt.Printf("[Redis] failed to store JWT signing key: %v\n", setErr)
	} else {
		fmt.Printf("[Redis] JWT signing key rotated, next rotation in %v\n", ttl)
	}
	return []byte(newKey)
}

func staticKey() []byte {
	key := os.Getenv("JWT_KEY")
	if key == "" {
		key = "default-secret"
	}
	return []byte(key)
}

func randomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; fall back to static key so startup doesn't break.
		return os.Getenv("JWT_KEY")
	}
	return base64.StdEncoding.EncodeToString(b)
}

func keyTTL() time.Duration {
	if s := os.Getenv("JWT_KEY_TTL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}
	return 24 * time.Hour
}
