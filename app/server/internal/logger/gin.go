package logger

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SetupGin 将 gin 默认 writer 接到 logger，并建议用 gin.New()+中间件替代 gin.Default()。
func SetupGin() {
	gin.DefaultWriter = LevelWriter{Level: logrus.InfoLevel}
	gin.DefaultErrorWriter = LevelWriter{Level: logrus.ErrorLevel}
	gin.DisableConsoleColor()
}

// GinLogger 请求访问日志中间件。
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		size := c.Writer.Size()
		if size < 0 {
			size = 0
		}
		if raw != "" {
			path = path + "?" + raw
		}

		entry := WithFields(Fields{
			"component":  "HTTP",
			"status":     status,
			"method":     method,
			"path":       path,
			"ip":         clientIP,
			"latency":    latency.String(),
			"latency_ms": latency.Milliseconds(),
			"size":       size,
		})
		if reqID := c.GetString("request_id"); reqID != "" {
			entry = entry.WithField("request_id", reqID)
		}
		if uid := c.GetString("user_uuid"); uid != "" {
			entry = entry.WithField("user_uuid", uid)
		}

		msg := "request"
		switch {
		case status >= 500:
			entry.Error(msg)
		case status >= 400:
			entry.Warn(msg)
		default:
			entry.Info(msg)
		}
	}
}

// GinRecovery panic 恢复中间件，打印堆栈并返回 500。
func GinRecovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(LevelWriter{Level: logrus.ErrorLevel}, func(c *gin.Context, recovered interface{}) {
		WithFields(Fields{
			"component": "HTTP",
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"panic":     recovered,
		}).Error("panic recovered")
		c.AbortWithStatus(500)
	})
}
