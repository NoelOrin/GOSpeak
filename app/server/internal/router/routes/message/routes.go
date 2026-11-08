package message

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.MessageHandler) {
	r.POST("/messages/list", h.List)
	r.POST("/messages/search", h.Search)
	r.POST("/messages/react", h.React)
}
