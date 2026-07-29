package conversation

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.ConversationHandler) {
	r.POST("/list", h.List)
	r.POST("/messages", h.Messages)
	r.POST("/mark-read", h.MarkRead)
}
