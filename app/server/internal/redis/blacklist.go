// Package redis — Token 黑名单子模块。
// 基于 Redis Key 过期机制实现 JWT 登出注销，Redis 未连接时静默跳过（best-effort）。
package redis

import (
	"context"
	"sync"
	"time"

	"GOSpeak/internal/logger"
)

// blacklistPrefix 黑名单 key 前缀，完整 key: jwt:blacklist:<jti>
const blacklistPrefix = "jwt:blacklist:"

// refreshFamilyPrefix refresh family 状态 key 前缀，完整 key: refresh:family:<family>
const (
	refreshFamilyPrefix = "refresh:family:"
	refreshFamilyTTL    = 7 * 24 * time.Hour
)

// memoryRefreshFamilies 是 Redis/NATS 都不可用时的单实例兜底，保证单进程内仍可检测刷新重放。
var memoryRefreshFamilies = struct {
	sync.Mutex
	m map[string]time.Time
}{m: make(map[string]time.Time)}

// BlacklistToken 将 JTI 加入黑名单，TTL 设为令牌剩余有效期。
// Redis 未连接时为 no-op，登出操作采用 best-effort 策略。
func BlacklistToken(jti string, remaining time.Duration) error {
	if jti == "" || remaining <= 0 {
		return nil
	}
	if Client != nil {
		ctx, cancel := redisTimeoutCtx()
		defer cancel()
		return Client.Set(ctx, blacklistPrefix+jti, "1", remaining).Err()
	}
	if secondaryAuth != nil {
		return secondaryAuth.BlacklistToken(jti, remaining)
	}
	return nil
}

// IsBlacklisted 检查 JTI 是否已被注销。
// 存储不可用或读取出错时按“未黑名单”处理，是可用性优先的 fail-open 策略；
// 安全敏感调用方应使用可返回错误的 AuthBackend.IsBlacklistedErr 自行决定策略。
func IsBlacklistedErr(jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	if Client != nil {
		ctx, cancel := redisTimeoutCtx()
		defer cancel()
		n, err := Client.Exists(ctx, blacklistPrefix+jti).Result()
		if err != nil {
			return false, err
		}
		return n > 0, nil
	}
	if secondaryAuth != nil {
		return secondaryAuth.IsBlacklistedErr(jti)
	}
	return false, nil
}

func IsBlacklisted(jti string) bool {
	if jti == "" {
		return false
	}
	if Client != nil {
		ok, err := IsBlacklistedErr(jti)
		if err != nil {
			logger.WithComponent("Redis").Warnf("IsBlacklisted backend error, treating as not blacklisted (fail-open): %v", err)
			return false
		}
		return ok
	}
	if secondaryAuth != nil {
		ok, err := secondaryAuth.IsBlacklistedErr(jti)
		if err != nil {
			logger.WithComponent("AuthKV").Warnf("IsBlacklisted backend error, treating as not blacklisted (fail-open): %v", err)
			return false
		}
		return ok
	}
	return false
}

// IsRefreshFamilyUsed 检查 refresh family 是否已被使用或吊销。
// 与黑名单不同，family 状态是安全敏感数据，底层错误会上抛由调用方 fail-closed。
func IsRefreshFamilyUsed(family string) (bool, error) {
	if family == "" {
		return false, nil
	}
	if Client != nil {
		ctx, cancel := redisTimeoutCtx()
		defer cancel()
		n, err := Client.Exists(ctx, refreshFamilyPrefix+family).Result()
		if err != nil {
			return false, err
		}
		return n > 0, nil
	}
	if backend, ok := secondaryAuth.(RefreshFamilyBackend); ok {
		return backend.IsRefreshFamilyUsed(family)
	}
	return isMemoryRefreshFamilyUsed(family), nil
}

// MarkRefreshFamilyUsed 原子标记 family 已被使用（SetNX 语义）。
// 返回 true 表示本次调用赢得标记，false 表示 family 已被并发或重放使用。
func MarkRefreshFamilyUsed(family string) (bool, error) {
	if family == "" {
		return false, nil
	}
	if Client != nil {
		ctx, cancel := redisTimeoutCtx()
		defer cancel()
		return Client.SetNX(ctx, refreshFamilyPrefix+family, "1", refreshFamilyTTL).Result()
	}
	if backend, ok := secondaryAuth.(RefreshFamilyBackend); ok {
		return backend.MarkRefreshFamilyUsed(family, refreshFamilyTTL)
	}
	return markMemoryRefreshFamilyUsed(family), nil
}

// RevokeRefreshFamily 吊销整个 refresh family，使该 family 下所有 refresh token 立即失效。
func RevokeRefreshFamily(family string) error {
	if family == "" {
		return nil
	}
	if Client != nil {
		ctx, cancel := redisTimeoutCtx()
		defer cancel()
		return Client.Set(ctx, refreshFamilyPrefix+family, "revoked", refreshFamilyTTL).Err()
	}
	if backend, ok := secondaryAuth.(RefreshFamilyBackend); ok {
		return backend.RevokeRefreshFamily(family)
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

// redisTimeoutCtx 返回统一的 Redis 操作超时上下文，避免依赖 go-redis 默认超时。
func redisTimeoutCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}
