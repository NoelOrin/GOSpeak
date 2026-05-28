package role

import (
	"go_rtc/internal/handler"
	"go_rtc/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.RoleHandler) {
	r.GET("/list", h.List)
	r.POST("/create", middleware.RequireRole("admin"), h.Create)
	r.DELETE("/:id", middleware.RequireRole("admin"), h.Delete)
}
