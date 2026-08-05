// Package redis — JWT 签名密钥轮换子模块。
// 基于 Redis 主动轮换签名密钥：密钥本身不设 TTL，由定时任务在 JWT_KEY_TTL 到期后
// 主动执行轮换，确保旧密钥先备份到历史集合再写入新密钥，避免密钥丢失导致旧 Token 失效。
package redis

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"GOSpeak/internal/config"
)

const (
	jwtKeyRedisKey     = "jwt:signing_key"
	jwtHistoryKey      = "jwt:signing_key:history"
	jwtKeyCreatedAtKey = "jwt:signing_key:created_at"
)

// production 标记生产模式，由 SetProductionMode 在启动时设置。
// 生产模式下未配置 JWT_KEY 且未连接 Redis 时拒绝启动，避免使用公开的默认密钥签发 token。
var production bool

// keyMu 串行化签名密钥的读取与轮换，避免并发首启时各自生成并写入不同密钥。
var keyMu sync.Mutex

// SetProductionMode 标记当前为生产环境，应在 gin.go 启动时调用。
func SetProductionMode() {
	production = true
}

// GetSigningKey 获取当前有效的 JWT 签名密钥。
// 密钥永不过期（无 TTL），由 RotateSigningKey 主动轮换。
// 开发环境且未接 Redis 时固定使用 JWT_KEY，避免内嵌 NATS 重启导致随机密钥丢失、强制重新登录。
func GetSigningKey() []byte {
	keyMu.Lock()
	defer keyMu.Unlock()
	if Client == nil {
		// 开发态：静态密钥优先，不把随机密钥写进易失 AuthStore。
		if !production {
			return staticKey()
		}
		if secondaryAuth != nil {
			if val, ok, err := secondaryAuth.GetSigningKey(); err == nil && ok && val != "" {
				return []byte(val)
			}
			// first start on secondary store
			newKey := randomKey()
			now := time.Now().Unix()
			if err := secondaryAuth.SetSigningKey(newKey, now); err != nil {
				// 并发首启时可能是另一实例抢占成功，先重读 active key。
				if val, ok, err2 := secondaryAuth.GetSigningKey(); err2 == nil && ok && val != "" {
					return []byte(val)
				}
				fmt.Printf("[AuthKV] failed to store JWT signing key: %v\n", err)
				return staticKey()
			}
			_ = secondaryAuth.AddHistoryKey(newKey)
			return []byte(newKey)
		}
		return staticKey()
	}

	ctx := context.Background()
	val, err := Client.Get(ctx, jwtKeyRedisKey).Result()
	if err == nil {
		return []byte(val)
	}

	// key 不存在（首次启动），创建新密钥
	newKey := randomKey()
	if setErr := Client.Set(ctx, jwtKeyRedisKey, newKey, 0).Err(); setErr != nil {
		fmt.Printf("[Redis] failed to store JWT signing key: %v; fallback to static key\n", setErr)
		return staticKey()
	}
	now := time.Now().Unix()
	Client.Set(ctx, jwtKeyCreatedAtKey, now, 0)
	Client.SAdd(ctx, jwtHistoryKey, newKey)
	Client.Expire(ctx, jwtHistoryKey, histTTL)
	fmt.Printf("[Redis] JWT signing key created\n")
	return []byte(newKey)
}

// ShouldRotateKey 检查签名密钥是否需要轮换。
// 基于 jwt:signing_key:created_at 与当前时间的差值判断。
func ShouldRotateKey() bool {
	if Client == nil {
		// 开发态静态密钥不参与轮换。
		if !production || secondaryAuth == nil {
			return false
		}
		createdAt, ok, err := secondaryAuth.GetCreatedAt()
		if err != nil || !ok {
			return true
		}
		ttl := int64(keyTTL().Seconds())
		return time.Now().Unix()-createdAt >= ttl
	}
	ctx := context.Background()
	createdAt, err := Client.Get(ctx, jwtKeyCreatedAtKey).Int64()
	if err != nil {
		return true // 无法读取创建时间，保守触发轮换
	}
	ttl := int64(keyTTL().Seconds())
	return time.Now().Unix()-createdAt >= ttl
}

// RotateSigningKey 主动轮换签名密钥。
// 原子性地：备份旧密钥 → 写入新密钥 → 更新创建时间。
// 返回新密钥。
func RotateSigningKey() []byte {
	keyMu.Lock()
	defer keyMu.Unlock()
	if Client == nil {
		// 开发态始终返回静态 JWT_KEY。
		if !production || secondaryAuth == nil {
			return staticKey()
		}
		old := ""
		if val, ok, err := secondaryAuth.GetSigningKey(); err == nil && ok && val != "" {
			old = val
			_ = secondaryAuth.AddHistoryKey(old)
		}
		newKey := randomKey()
		now := time.Now().Unix()
		if err := secondaryAuth.UpdateSigningKey(newKey, now); err != nil {
			fmt.Printf("[AuthKV] JWT signing key rotate failed: %v; keep old key\n", err)
			if old != "" {
				return []byte(old)
			}
			return staticKey()
		}
		_ = secondaryAuth.AddHistoryKey(newKey)
		fmt.Printf("[AuthKV] JWT signing key rotated, next rotation in %v\n", keyTTL())
		return []byte(newKey)
	}

	ctx := context.Background()

	// 1. 读取旧密钥并备份
	oldVal, err := Client.Get(ctx, jwtKeyRedisKey).Result()
	if err == nil && oldVal != "" {
		Client.SAdd(ctx, jwtHistoryKey, oldVal)
	}

	// 2. 生成新密钥并写入（不带 TTL）；主键写失败时保留旧密钥，避免本进程与共享存储分叉。
	newKey := randomKey()
	if err := Client.Set(ctx, jwtKeyRedisKey, newKey, 0).Err(); err != nil {
		fmt.Printf("[Redis] JWT signing key rotate failed: %v; keep old key\n", err)
		if oldVal != "" {
			return []byte(oldVal)
		}
		return staticKey()
	}
	if err := Client.Set(ctx, jwtKeyCreatedAtKey, time.Now().Unix(), 0).Err(); err != nil {
		fmt.Printf("[Redis] JWT signing key rotate createdAt failed: %v\n", err)
	}

	// 3. 新密钥也加入历史集合
	Client.SAdd(ctx, jwtHistoryKey, newKey)
	// 历史保留 7 天，精确覆盖 refresh_token 最大有效期；超出 7 天的密钥对过期 token 已无用
	Client.Expire(ctx, jwtHistoryKey, histTTL)

	fmt.Printf("[Redis] JWT signing key rotated, next rotation in %v\n", keyTTL())
	return []byte(newKey)
}

// GetAllSigningKeys 返回当前签名密钥 + 历史密钥集合。
// 用于 Token 校验时逐一尝试，解决密钥轮换后旧 Token 无法验签的问题。
func GetAllSigningKeys() [][]byte {
	if Client == nil {
		// 开发态仅校验静态密钥，避免读到重启前残留的随机密钥集合。
		if !production {
			return [][]byte{staticKey()}
		}
		if secondaryAuth != nil {
			var keys [][]byte
			active := ""
			if val, ok, err := secondaryAuth.GetSigningKey(); err == nil && ok {
				active = val
				if active != "" {
					keys = append(keys, []byte(active))
				}
			}
			for _, k := range secondaryAuth.HistoryKeys() {
				if k != "" && k != active {
					keys = append(keys, []byte(k))
				}
			}
			if len(keys) > 0 {
				return keys
			}
		}
		return [][]byte{staticKey()}
	}

	ctx := context.Background()
	var keys [][]byte

	active, err := Client.Get(ctx, jwtKeyRedisKey).Result()
	if err == nil {
		keys = append(keys, []byte(active))
	}

	history, err := Client.SMembers(ctx, jwtHistoryKey).Result()
	if err == nil {
		for _, k := range history {
			if k != active {
				keys = append(keys, []byte(k))
			}
		}
	}

	return keys
}

// staticKey 从环境变量读取静态签名密钥。
// 开发环境未配置 JWT_KEY 时回退到硬编码默认值；
// 生产环境未配置且未连接 Redis 时直接 panic，避免使用公开的默认密钥签发任意角色的合法 token。
func staticKey() []byte {
	key := ""
	if cfg := config.Current(); cfg != nil {
		key = cfg.JWTKey
	}
	if key == "" || key == "default-secret" {
		if production {
			panic("JWT_KEY must be set in production (or connect Redis for key rotation)")
		}
		if key == "" {
			key = "default-secret"
		}
	}
	return []byte(key)
}

// randomKey 生成 32 字节随机密钥并做 base64 编码。
// crypto/rand.Read 失败概率极低，此时退化为环境变量值保证启动不中断。
func randomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		if cfg := config.Current(); cfg != nil && cfg.JWTKey != "" {
			return cfg.JWTKey
		}
		return "default-secret"
	}
	return base64.StdEncoding.EncodeToString(b)
}

// keyTTL 解析 JWT_KEY_TTL 环境变量，非法或空值时默认 24 小时。
func keyTTL() time.Duration {
	if cfg := config.Current(); cfg != nil {
		return cfg.JWTKeyTTLDuration()
	}
	return 24 * time.Hour
}

// histTTL 历史密钥集合的保留时长，对齐 refresh_token 的 7 天最大有效期。
const histTTL = 7 * 24 * time.Hour

// KeyRotationLoop 后台定时检查密钥是否需要轮换。
// 每分钟检查一次，到达 JWT_KEY_TTL 时主动执行 RotateSigningKey。
func KeyRotationLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if ShouldRotateKey() {
			RotateSigningKey()
		}
	}
}
