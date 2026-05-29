package role

import (
	"go_rtc/internal/handler"
	"go_rtc/internal/middleware"
	"go_rtc/internal/model"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.RoleHandler) {
	r.GET("/list", middleware.RequirePermission(model.PermRoleRead), h.List)
	r.POST("/create", middleware.RequirePermission(model.PermRoleManage), h.Create)
	r.DELETE("/:id", middleware.RequirePermission(model.PermRoleManage), h.Delete)
}
