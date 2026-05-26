// Package redis — JWT 签名密钥轮换子模块。
// 基于 Redis Key TTL 自动轮换签名密钥，旧密钥过期后新密钥自动生效，所有旧 Token 立即失效。
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

// GetOrRotateSigningKey 获取当前有效的 JWT 签名密钥。
//
//   - Redis 已连接：从 Redis 读取密钥，若 key 不存在或 TTL 已过期则生成随机新密钥并写入，
//     TTL 由 JWT_KEY_TTL （默认 24h）控制。TTL 到期即触发密钥轮换，旧 Token 自动失效。
//   - Redis 未连接：退化为静态 JWT_KEY 环境变量，未设置时使用 "default-secret"。
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

// staticKey 从环境变量读取静态签名密钥，未设置时使用硬编码默认值（仅开发环境）。
func staticKey() []byte {
	key := os.Getenv("JWT_KEY")
	if key == "" {
		key = "default-secret"
	}
	return []byte(key)
}

// randomKey 生成 32 字节随机密钥并做 base64 编码。
// crypto/rand.Read 失败概率极低，此时退化为环境变量值保证启动不中断。
func randomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return os.Getenv("JWT_KEY")
	}
	return base64.StdEncoding.EncodeToString(b)
}

// keyTTL 解析 JWT_KEY_TTL 环境变量，非法或空值时默认 24 小时。
func keyTTL() time.Duration {
	if s := os.Getenv("JWT_KEY_TTL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}
	return 24 * time.Hour
}
