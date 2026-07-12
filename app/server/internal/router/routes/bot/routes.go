package bot

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.BotHandler) {
	r.POST("/create", middleware.RequirePermission(permcode.PermBotManage), h.Create)
	r.POST("/list", middleware.RequirePermission(permcode.PermBotManage), h.List)
	r.POST("/revoke", middleware.RequirePermission(permcode.PermBotManage), h.Revoke)
}
