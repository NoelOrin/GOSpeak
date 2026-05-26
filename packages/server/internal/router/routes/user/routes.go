package user

import (
	"go_rtc/internal/handler"
	"go_rtc/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.UserHandler) {
	r.GET("/profile", h.GetProfile)
	r.GET("/list", h.List)
	r.GET("/:id", h.GetByID)
	r.DELETE("/:id", middleware.RequireRole("admin"), h.Delete)
	r.PUT("/:id/role", middleware.RequireRole("admin"), h.UpdateRole)
}
