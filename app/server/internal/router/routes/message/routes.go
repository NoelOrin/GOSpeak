package message

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.MessageHandler) {
	r.POST("/list", middleware.RequirePermission(permcode.PermRoomRead), h.List)
}
