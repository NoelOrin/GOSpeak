// Package redis — Token 黑名单子模块。
// 基于 Redis Key 过期机制实现 JWT 登出注销，Redis 未连接时静默跳过（best-effort）。
package redis

import (
	"context"
	"fmt"
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
// 存储不可用或读取出错时按“未黑名单”处理，是可用性优先的 fail-open 策略；
// 安全敏感调用方应使用可返回错误的 AuthBackend.IsBlacklistedErr 自行决定策略。
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
		ok, err := secondaryAuth.IsBlacklistedErr(jti)
		if err != nil {
			fmt.Printf("[AuthKV] IsBlacklisted backend error, treating as not blacklisted (fail-open): %v\n", err)
			return false
		}
		return ok
	}
	return false
}
