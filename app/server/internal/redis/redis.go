// Package redis 提供可选的 Redis 客户端，未配置时优雅降级，不影响主流程。
package redis

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/logger"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// client 全局 Redis 客户端。REDIS_HOST 为空时为 nil，所有使用者需判断。
var client *redis.Client

// Client 返回当前全局 Redis 客户端，未连接时为 nil。外部包只读，不能替换实例。
func Client() *redis.Client {
	return client
}

// InitRedis 根据配置初始化 Redis 连接。
// REDIS_HOST 为空时跳过连接，不 panic、不报错。
func InitRedis(cfg *config.Config) {
	if cfg == nil {
		logger.WithComponent("Redis").Warn("config is nil, skipping Redis connection")
		return
	}
	host := cfg.RedisHost
	if host == "" {
		logger.WithComponent("Redis").Info("REDIS_HOST not set, skipping Redis connection")
		return
	}

	port := cfg.RedisPort
	if port == "" {
		port = "6379"
	}

	db := cfg.RedisDBIndex()
	c := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: cfg.RedisPassword,
		DB:       db,
	})

	pingCtx, cancel := redisTimeoutCtx()
	defer cancel()
	if _, err := c.Ping(pingCtx).Result(); err != nil {
		logger.WithComponent("Redis").WithError(err).Warn("connection failed")
		return
	}

	client = c
	jwtCfg = cfg
	logger.WithComponent("Redis").Infof("connected: %s:%s db=%d", host, port, db)
}

// IsConnected 检查 Redis 是否已成功连接。调用方应优先判断此值再执行 Redis 操作。
func IsConnected() bool {
	return client != nil
}

// RedisStats Redis 详细状态，用于监控面板。
type RedisStats struct {
	Connected        bool    `json:"connected"`
	PingMs           int64   `json:"ping_ms"`
	DBSize           int64   `json:"db_size"`
	UsedMemoryMB     float64 `json:"used_memory_mb"`
	UsedMemoryPeakMB float64 `json:"used_memory_peak_mb"`
	ConnectedClients int64   `json:"connected_clients"`
}

// GetStats 返回 Redis 详细状态。client 为 nil 时仅返回 Connected=false。
func GetStats() RedisStats {
	if client == nil {
		return RedisStats{Connected: false}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stats := RedisStats{Connected: true}

	start := time.Now()
	if _, err := client.Ping(ctx).Result(); err == nil {
		stats.PingMs = time.Since(start).Milliseconds()
	}

	if n, err := client.DBSize(ctx).Result(); err == nil {
		stats.DBSize = n
	}

	if info, err := client.Info(ctx, "memory", "clients").Result(); err == nil {
		m := parseRedisInfo(info)
		if v, ok := m["used_memory"]; ok {
			if bytes, err := strconv.ParseInt(v, 10, 64); err == nil {
				stats.UsedMemoryMB = float64(int(float64(bytes)/1024/1024*100)) / 100
			}
		}
		if v, ok := m["used_memory_peak"]; ok {
			if bytes, err := strconv.ParseInt(v, 10, 64); err == nil {
				stats.UsedMemoryPeakMB = float64(int(float64(bytes)/1024/1024*100)) / 100
			}
		}
		if v, ok := m["connected_clients"]; ok {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				stats.ConnectedClients = n
			}
		}
	}

	return stats
}

// parseRedisInfo 解析 INFO 命令的 key:value 文本为 map。
func parseRedisInfo(info string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			result[line[:idx]] = line[idx+1:]
		}
	}
	return result
}
