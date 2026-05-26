package redis

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// Client is the global Redis client. Nil if Redis is not configured.
var Client *redis.Client

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

func IsConnected() bool {
	return Client != nil
}
