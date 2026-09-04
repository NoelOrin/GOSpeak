package email_config

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.EmailConfigHandler) {
	r.POST("/config", middleware.RequirePermission(permcode.PermEmailConfigRead), h.Get)
	r.POST("/update-config", middleware.RequirePermission(permcode.PermEmailConfigManage), h.Update)
}
