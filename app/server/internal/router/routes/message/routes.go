package message

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.MessageHandler) {
	r.POST("/messages/list", h.List)
	r.POST("/messages/search", h.Search)
	r.POST("/messages/send", h.Send)
	r.POST("/messages/edit", h.Edit)
	r.POST("/messages/delete", h.Delete)
	r.POST("/messages/react", h.React)
	r.POST("/messages/unreact", h.Unreact)
}
