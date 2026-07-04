package sfu_config

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.SFUConfigHandler) {
	r.POST("/config", middleware.RequirePermission(permcode.PermSFUManage), h.Get)
	r.POST("/update-config", middleware.RequirePermission(permcode.PermSFUManage), h.Update)
}
