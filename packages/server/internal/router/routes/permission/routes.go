package permission

import (
	"go_rtc/internal/handler"
	"go_rtc/internal/middleware"
	"go_rtc/internal/model"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.PermissionHandler) {
	r.GET("/list", middleware.RequirePermission(model.PermRoleRead), h.ListPermissions)
	r.GET("/role", middleware.RequirePermission(model.PermRoleRead), h.GetRolePermissions)
	r.POST("/sync", middleware.RequirePermission(model.PermRoleManage), h.SyncRolePermissions)
}
