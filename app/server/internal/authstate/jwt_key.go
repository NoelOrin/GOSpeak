package authstate

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/logger"
)

// production marks production mode; it is set by SetProductionMode at startup.
// In production a static default JWT key must never be used.
var production bool

// jwtCfg is captured by Configure so key logic never reads global config.
var jwtCfg *config.Config

// keyMu serializes signing-key reads and rotation so concurrent first starts
// cannot write different keys.
var keyMu sync.Mutex

// Configure captures config for signing-key decisions.
func Configure(c *config.Config) {
	keyMu.Lock()
	jwtCfg = c
	keyMu.Unlock()
}

// SetProductionMode marks the process as running in production.
func SetProductionMode() {
	production = true
}

// GetSigningKey returns the active JWT signing key.
// Development uses the static JWT_KEY so embedded-NATS restarts do not rotate
// the key and force every client to log in again. Production uses the shared
// backend when available and creates a random key on first start.
func GetSigningKey() []byte {
	keyMu.Lock()
	defer keyMu.Unlock()
	if !production {
		return staticKey()
	}
	b := currentBackend()
	if b == nil {
		return staticKey()
	}
	if val, ok, err := b.GetSigningKey(); err == nil && ok && val != "" {
		return []byte(val)
	}
	newKey := randomKey()
	now := time.Now().Unix()
	if err := b.SetSigningKey(newKey, now); err != nil {
		// A concurrent first start may have won; re-read before falling back.
		if val, ok, err2 := b.GetSigningKey(); err2 == nil && ok && val != "" {
			return []byte(val)
		}
		logger.WithComponent("AuthKV").WithError(err).Warn("failed to store JWT signing key")
		return staticKey()
	}
	_ = b.AddHistoryKey(newKey)
	return []byte(newKey)
}

// ShouldRotateKey reports whether the signing key needs rotation.
func ShouldRotateKey() bool {
	if !production {
		return false
	}
	b := currentBackend()
	if b == nil {
		return false
	}
	createdAt, ok, err := b.GetCreatedAt()
	if err != nil || !ok {
		return true
	}
	return time.Now().Unix()-createdAt >= int64(keyTTL().Seconds())
}

// RotateSigningKey rotates the signing key: backs up the old key, writes the
// new key, and updates the creation timestamp.
func RotateSigningKey() []byte {
	keyMu.Lock()
	defer keyMu.Unlock()
	if !production {
		return staticKey()
	}
	b := currentBackend()
	if b == nil {
		return staticKey()
	}
	old := ""
	if val, ok, err := b.GetSigningKey(); err == nil && ok && val != "" {
		old = val
		_ = b.AddHistoryKey(old)
	}
	newKey := randomKey()
	if err := b.UpdateSigningKey(newKey, time.Now().Unix()); err != nil {
		logger.WithComponent("AuthKV").WithError(err).Warn("JWT signing key rotate failed; keep old key")
		if old != "" {
			return []byte(old)
		}
		return staticKey()
	}
	_ = b.AddHistoryKey(newKey)
	logger.WithComponent("AuthKV").Infof("JWT signing key rotated, next rotation in %v", keyTTL())
	return []byte(newKey)
}

// GetAllSigningKeys returns the active key plus history keys for verification
// after rotations.
func GetAllSigningKeys() [][]byte {
	if !production {
		return [][]byte{staticKey()}
	}
	b := currentBackend()
	if b == nil {
		return [][]byte{staticKey()}
	}
	var keys [][]byte
	active := ""
	if val, ok, err := b.GetSigningKey(); err == nil && ok {
		active = val
		if active != "" {
			keys = append(keys, []byte(active))
		}
	}
	for _, k := range b.HistoryKeys() {
		if k != "" && k != active {
			keys = append(keys, []byte(k))
		}
	}
	if len(keys) > 0 {
		return keys
	}
	return [][]byte{staticKey()}
}

// staticKey reads the static signing key from config. Development falls back
// to a hard-coded default; production panics rather than using it.
func staticKey() []byte {
	key := ""
	if cfg := jwtCfg; cfg != nil {
		key = cfg.JWTKey
	}
	if key == "" || key == "default-secret" {
		if production {
			panic("JWT_KEY must be set in production")
		}
		if key == "" {
			key = "default-secret"
		}
	}
	return []byte(key)
}

// randomKey generates a 32-byte random key as base64.
func randomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		if cfg := jwtCfg; cfg != nil && cfg.JWTKey != "" {
			return cfg.JWTKey
		}
		return "default-secret"
	}
	return base64.StdEncoding.EncodeToString(b)
}

// keyTTL parses JWT_KEY_TTL and defaults to 24h on invalid/empty values.
func keyTTL() time.Duration {
	if cfg := jwtCfg; cfg != nil {
		return cfg.JWTKeyTTLDuration()
	}
	return 24 * time.Hour
}

// histTTL matches the refresh-token window (7 days).
const histTTL = 7 * 24 * time.Hour

var (
	rotationStopMu sync.Mutex
	rotationStopCh chan struct{}
)

// StartKeyRotationLoop checks the signing key every minute and rotates it when
// JWT_KEY_TTL elapses.
func StartKeyRotationLoop() {
	rotationStopMu.Lock()
	defer rotationStopMu.Unlock()
	if rotationStopCh != nil {
		return
	}
	ch := make(chan struct{})
	rotationStopCh = ch
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ch:
				return
			case <-ticker.C:
				if ShouldRotateKey() {
					RotateSigningKey()
				}
			}
		}
	}()
}

// StopKeyRotationLoop stops the rotation goroutine for graceful shutdown.
func StopKeyRotationLoop() {
	rotationStopMu.Lock()
	defer rotationStopMu.Unlock()
	if rotationStopCh == nil {
		return
	}
	close(rotationStopCh)
	rotationStopCh = nil
}
