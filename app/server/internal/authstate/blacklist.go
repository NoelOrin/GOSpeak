package authstate

import (
	"sync"
	"time"

	"GOSpeak/internal/logger"
)

const (
	refreshFamilyTTL = 7 * 24 * time.Hour
)

// memoryRefreshFamilies is the single-instance fallback used when no shared
// backend is available, so refresh replay is still detected within one process.
var memoryRefreshFamilies = struct {
	sync.Mutex
	m map[string]time.Time
}{m: make(map[string]time.Time)}

// BlacklistToken marks jti revoked for the remaining token lifetime. Without a
// shared backend the write is a no-op (best effort).
func BlacklistToken(jti string, remaining time.Duration) error {
	if jti == "" || remaining <= 0 {
		return nil
	}
	if b := currentBackend(); b != nil {
		return b.BlacklistToken(jti, remaining)
	}
	return nil
}

// IsBlacklistedErr reports whether jti is revoked. Storage errors are returned
// so security-sensitive callers can choose their policy.
func IsBlacklistedErr(jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	if b := currentBackend(); b != nil {
		return b.IsBlacklistedErr(jti)
	}
	return false, nil
}

// IsBlacklisted reports whether jti is revoked and treats storage errors as
// not blacklisted (fail-open, same as the pre-migration behavior).
func IsBlacklisted(jti string) bool {
	if jti == "" {
		return false
	}
	ok, err := IsBlacklistedErr(jti)
	if err != nil {
		logger.WithComponent("AuthKV").Warnf("IsBlacklisted backend error, treating as not blacklisted (fail-open): %v", err)
		return false
	}
	return ok
}

// IsRefreshFamilyUsed reports whether a refresh family has been used or
// revoked. Errors are propagated because family state is security-sensitive.
func IsRefreshFamilyUsed(family string) (bool, error) {
	if family == "" {
		return false, nil
	}
	if b := currentBackend(); b != nil {
		return b.IsRefreshFamilyUsed(family)
	}
	return isMemoryRefreshFamilyUsed(family), nil
}

// MarkRefreshFamilyUsed atomically marks a family used (SetNX semantics).
// true means this call won the mark; false means the family was already used.
func MarkRefreshFamilyUsed(family string) (bool, error) {
	if family == "" {
		return false, nil
	}
	if b := currentBackend(); b != nil {
		return b.MarkRefreshFamilyUsed(family, refreshFamilyTTL)
	}
	return markMemoryRefreshFamilyUsed(family), nil
}

// RevokeRefreshFamily invalidates every refresh token in the family.
func RevokeRefreshFamily(family string) error {
	if family == "" {
		return nil
	}
	if b := currentBackend(); b != nil {
		return b.RevokeRefreshFamily(family)
	}
	revokeMemoryRefreshFamily(family)
	return nil
}

func markMemoryRefreshFamilyUsed(family string) bool {
	now := time.Now()
	memoryRefreshFamilies.Lock()
	defer memoryRefreshFamilies.Unlock()
	if exp, ok := memoryRefreshFamilies.m[family]; ok && now.Before(exp) {
		return false
	}
	memoryRefreshFamilies.m[family] = now.Add(refreshFamilyTTL)
	return true
}

func isMemoryRefreshFamilyUsed(family string) bool {
	now := time.Now()
	memoryRefreshFamilies.Lock()
	defer memoryRefreshFamilies.Unlock()
	exp, ok := memoryRefreshFamilies.m[family]
	if !ok {
		return false
	}
	if !now.Before(exp) {
		delete(memoryRefreshFamilies.m, family)
		return false
	}
	return true
}

func revokeMemoryRefreshFamily(family string) {
	memoryRefreshFamilies.Lock()
	defer memoryRefreshFamilies.Unlock()
	memoryRefreshFamilies.m[family] = time.Now().Add(refreshFamilyTTL)
}
