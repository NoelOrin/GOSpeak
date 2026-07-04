package email_config

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.EmailConfigHandler) {
	admin := r.Group("", middleware.RequireRole("admin"))
	admin.POST("/config", h.Get)
	admin.POST("/update-config", h.Update)
}
