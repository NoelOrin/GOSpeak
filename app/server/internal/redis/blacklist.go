// Package redis — Token 黑名单子模块。
// 基于 Redis Key 过期机制实现 JWT 登出注销，Redis 未连接时静默跳过（best-effort）。
package redis

import (
	"context"
	"time"
)

// blacklistPrefix 黑名单 key 前缀，完整 key: jwt:blacklist:<jti>
const blacklistPrefix = "jwt:blacklist:"

// BlacklistToken 将 JTI 加入黑名单，TTL 设为令牌剩余有效期。
// Redis 未连接时为 no-op，登出操作采用 best-effort 策略。
func BlacklistToken(jti string, remaining time.Duration) error {
	if jti == "" || remaining <= 0 {
		return nil
	}
	if Client != nil {
		ctx := context.Background()
		return Client.Set(ctx, blacklistPrefix+jti, "1", remaining).Err()
	}
	if secondaryAuth != nil {
		return secondaryAuth.BlacklistToken(jti, remaining)
	}
	return nil
}

// IsBlacklisted 检查 JTI 是否已被注销。
// Redis 未连接时返回 false，保证旧 token 在极端情况下仍能短暂使用。
func IsBlacklisted(jti string) bool {
	if jti == "" {
		return false
	}
	if Client != nil {
		ctx := context.Background()
		n, err := Client.Exists(ctx, blacklistPrefix+jti).Result()
		return err == nil && n > 0
	}
	if secondaryAuth != nil {
		return secondaryAuth.IsBlacklisted(jti)
	}
	return false
}
