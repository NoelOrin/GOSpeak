package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/redis"
	"GOSpeak/internal/repository"

	"github.com/gin-gonic/gin"
)

// MonitorHandler 服务监控 SSE 处理器
type MonitorHandler struct {
	startTime time.Time
}

func NewMonitorHandler() *MonitorHandler {
	return &MonitorHandler{startTime: time.Now()}
}

// HealthStream SSE 流式推送服务健康指标
func (h *MonitorHandler) HealthStream(c *gin.Context) {
	// 从 query param 获取 token 并校验（EventSource 不支持自定义 header）
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
		return
	}
	claims, err := pkg.ParseToken(tokenStr)
	if err != nil || pkg.IsTokenExpired(claims) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}
	// 仅 admin 可访问监控 SSE
	if claims.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			snap := h.collect()
			data, _ := json.Marshal(snap)
			if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

type healthSnapshot struct {
	Timestamp      string  `json:"timestamp"`
	Uptime         string  `json:"uptime"`
	NumGoroutine   int     `json:"num_goroutine"`
	NumCPU         int     `json:"num_cpu"`
	AllocMB        float64 `json:"alloc_mb"`
	TotalAllocMB   float64 `json:"total_alloc_mb"`
	SysMB          float64 `json:"sys_mb"`
	NumGC          uint32  `json:"num_gc"`
	DBConnected    bool    `json:"db_connected"`
	RedisConnected bool    `json:"redis_connected"`
}

func (h *MonitorHandler) collect() healthSnapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	dbOk := false
	if repository.DB != nil {
		sqlDB, err := repository.DB.DB()
		dbOk = err == nil && sqlDB.Ping() == nil
	}

	return healthSnapshot{
		Timestamp:      time.Now().Format("15:04:05"),
		Uptime:         time.Since(h.startTime).Truncate(time.Second).String(),
		NumGoroutine:   runtime.NumGoroutine(),
		NumCPU:         runtime.NumCPU(),
		AllocMB:        roundMB(m.Alloc),
		TotalAllocMB:   roundMB(m.TotalAlloc),
		SysMB:          roundMB(m.Sys),
		NumGC:          m.NumGC,
		DBConnected:    dbOk,
		RedisConnected: redis.IsConnected(),
	}
}

func roundMB(bytes uint64) float64 {
	return float64(int(float64(bytes)/1024/1024*100)) / 100
}
