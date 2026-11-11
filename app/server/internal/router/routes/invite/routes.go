package invite

import (
	"time"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Register 注册邀请码管理端点（protected 组，权限在 handler 内按域校验）。
func Register(r *gin.RouterGroup, h *handler.InviteHandler) {
	r.POST("/domain/invites/list", h.List)
	r.POST("/domain/invites/create", h.Create)
	r.POST("/domain/invites/disable", h.Disable)
	r.POST("/domain/invites/redemptions", h.Redemptions)
}

// RegisterPublic 注册公开预览端点（无需登录），按 IP 限流 20 次/小时，
// 防止用无效码枚举域存在性。
func RegisterPublic(r *gin.RouterGroup, h *handler.InviteHandler) {
	r.POST("/preview", middleware.RateLimit(20, time.Hour), h.Preview)
}
