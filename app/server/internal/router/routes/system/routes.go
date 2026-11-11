package system

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterProtected 将监控 SSE 与更新检查移入统一 JWT 鉴权组，并仅放行 admin。
func RegisterProtected(r *gin.RouterGroup, h *handler.MonitorHandler, u *handler.SystemUpdateHandler) {
	r.GET("/stream", middleware.RequireRole("admin"), h.HealthStream)

	// 更新检查：仅 admin 可见可用（服务层再加一层防御性校验）。
	update := r.Group("/update")
	{
		update.GET("/current", middleware.RequireRole("admin"), u.CurrentVersion)
		update.GET("/check", middleware.RequireRole("admin"), u.CheckUpdate)
	}
}
