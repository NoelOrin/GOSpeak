package message

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.MessageHandler) {
	r.POST("/messages/list", middleware.RequirePermission(permcode.PermMessageRead), h.List)
	r.POST("/messages/send", middleware.RequirePermission(permcode.PermMessageSend), h.Send)
	r.POST("/messages/edit", middleware.RequirePermission(permcode.PermMessageSend), h.Edit)
	r.POST("/messages/delete", middleware.RequirePermission(permcode.PermMessageSend), h.Delete)
	r.POST("/messages/react", middleware.RequirePermission(permcode.PermMessageSend), h.React)
	r.POST("/messages/unreact", middleware.RequirePermission(permcode.PermMessageSend), h.Unreact)
}
