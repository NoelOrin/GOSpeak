// Package redis 提供可选的 Redis 客户端，未配置时优雅降级，不影响主流程。
package redis

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client 全局 Redis 客户端。REDIS_HOST 为空时为 nil，所有使用者需判断。
var Client *redis.Client

// InitRedis 根据环境变量初始化 Redis 连接。
// REDIS_HOST 为空时跳过连接，不 panic、不报错。
func InitRedis() {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		fmt.Println("[Redis] REDIS_HOST not set, skipping Redis connection")
		return
	}

	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}

	password := os.Getenv("REDIS_PASSWORD")

	db := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if n, err := strconv.Atoi(dbStr); err == nil {
			db = n
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})

	if _, err := client.Ping(context.Background()).Result(); err != nil {
		fmt.Printf("[Redis] connection failed: %v\n", err)
		return
	}

	Client = client
	fmt.Printf("[Redis] connected: %s:%s db=%d\n", host, port, db)
}

// IsConnected 检查 Redis 是否已成功连接。调用方应优先判断此值再执行 Redis 操作。
func IsConnected() bool {
	return Client != nil
}

// RedisStats Redis 详细状态，用于监控面板。
type RedisStats struct {
	Connected    bool    `json:"connected"`
	PingMs       int64   `json:"ping_ms"`
	DBSize       int64   `json:"db_size"`
	UsedMemoryMB float64 `json:"used_memory_mb"`
	UsedMemoryPeakMB float64 `json:"used_memory_peak_mb"`
	ConnectedClients int64 `json:"connected_clients"`
}

// GetStats 返回 Redis 详细状态。Client 为 nil 时仅返回 Connected=false。
func GetStats() RedisStats {
	if Client == nil {
		return RedisStats{Connected: false}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stats := RedisStats{Connected: true}

	start := time.Now()
	if _, err := Client.Ping(ctx).Result(); err == nil {
		stats.PingMs = time.Since(start).Milliseconds()
	}

	if n, err := Client.DBSize(ctx).Result(); err == nil {
		stats.DBSize = n
	}

	if info, err := Client.Info(ctx, "memory", "clients").Result(); err == nil {
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
