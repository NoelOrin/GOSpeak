// Package redis 提供可选的 Redis 客户端，未配置时优雅降级，不影响主流程。
package redis

import (
	"context"
	"fmt"
	"os"
	"strconv"

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
