package audit

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

// RegisterProtected 注册审计查询路由，需 audit:read 权限（管理员只读）。
func RegisterProtected(r *gin.RouterGroup, h *handler.AuditHandler) {
	r.POST("/list", middleware.RequirePermission(permcode.PermAuditRead), h.List)
}
