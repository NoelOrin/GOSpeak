package user

import (
	"go_rtc/internal/handler"
	"go_rtc/internal/middleware"
	"go_rtc/internal/model"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.UserHandler) {
	r.GET("/profile", h.GetProfile)
	r.GET("/list", middleware.RequirePermission(model.PermUserRead), h.List)
	r.GET("/:id", middleware.RequirePermission(model.PermUserRead), h.GetByID)
	r.DELETE("/:id", middleware.RequirePermission(model.PermUserDelete), h.Delete)
	r.PUT("/:id/role", middleware.RequirePermission(model.PermUserUpdate), h.UpdateRole)
}
