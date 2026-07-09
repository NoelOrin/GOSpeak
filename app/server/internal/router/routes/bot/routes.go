package bot

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

// RegisterProtected 注册 Bot 密钥管理接口，仅 bot:manage 权限可访问（admin 专属）。
func RegisterProtected(r *gin.RouterGroup, h *handler.BotAPIKeyHandler) {
	r.POST("/key/create", middleware.RequirePermission(permcode.PermBotManage), h.Create)
	r.POST("/key/list", middleware.RequirePermission(permcode.PermBotManage), h.List)
	r.POST("/key/revoke", middleware.RequirePermission(permcode.PermBotManage), h.Revoke)
}
