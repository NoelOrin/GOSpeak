package role

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.RoleHandler) {
	r.POST("/list", middleware.RequirePermission(permcode.PermRoleRead), h.List)
	r.POST("/create", middleware.RequirePermission(permcode.PermRoleManage), h.Create)
	r.POST("/update", middleware.RequirePermission(permcode.PermRoleManage), h.Update)
	r.POST("/delete", middleware.RequirePermission(permcode.PermRoleManage), h.Delete)
}
