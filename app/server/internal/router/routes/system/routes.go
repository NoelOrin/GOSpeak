package system

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterPublic 注册无需鉴权的版本查询端点。
//
// 只暴露「当前是什么版本」，供客户端「关于」页展示；是否可更新、最新版是
// 多少仍留在下面的 admin-only 组里。
func RegisterPublic(r *gin.RouterGroup, v *handler.SystemVersionHandler) {
	r.GET("/version", v.Version)
}

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
