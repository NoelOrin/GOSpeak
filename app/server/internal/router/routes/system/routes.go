package system

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterProtected 将监控 SSE 移入统一 JWT 鉴权组，并仅放行 admin。
func RegisterProtected(r *gin.RouterGroup, h *handler.MonitorHandler) {
	r.GET("/stream", middleware.RequireRole("admin"), h.HealthStream)
}
