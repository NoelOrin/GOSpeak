package signal

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.SignalHandler) {
	r.POST("/webhook", h.LivekitWebhook)
}

func RegisterProtected(r *gin.RouterGroup, h *handler.SignalHandler) {
	r.GET("/rooms", h.ListRooms)
	r.GET("/participants", h.ListParticipants)
	r.POST("/token", h.GetJoinToken)
	r.GET("/ws-endpoint", h.GetWSEndpoint)
}
