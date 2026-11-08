package permission

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.PermissionHandler) {
	r.POST("/list", middleware.RequirePermission(permcode.PermRoleRead), h.ListPermissions)
	r.POST("/role", middleware.RequirePermission(permcode.PermRoleRead), h.GetRolePermissions)
	r.POST("/sync", middleware.RequirePermission(permcode.PermRoleManage), h.SyncRolePermissions)
}
