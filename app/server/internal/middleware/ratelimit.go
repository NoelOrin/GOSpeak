package middleware

import (
	"sync"
	"time"

	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

// rateBucket 是固定窗口限流桶。
type rateBucket struct {
	count int
	reset time.Time
}

// rateLimiter 是进程内固定窗口限流器；多实例部署时应由前置网关或 Redis 补充全局策略。
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
	max     int
	window  time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*rateBucket),
		max:     max,
		window:  window,
	}
}

func (l *rateLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buckets) > 10000 {
		for k, b := range l.buckets {
			if now.After(b.reset) {
				delete(l.buckets, k)
			}
		}
	}

	b := l.buckets[key]
	if b == nil || now.After(b.reset) {
		l.buckets[key] = &rateBucket{count: 1, reset: now.Add(l.window)}
		return true
	}
	b.count++
	return b.count <= l.max
}

// RateLimit 按客户端 IP 限制请求频率，供登录、注册和验证码等敏感端点使用。
func RateLimit(max int, window time.Duration) gin.HandlerFunc {
	limiter := newRateLimiter(max, window)
	return func(c *gin.Context) {
		if !limiter.allow(c.ClientIP()) {
			pkg.Fail(c, pkg.RATE_LIMITED)
			c.Abort()
			return
		}
		c.Next()
	}
}
