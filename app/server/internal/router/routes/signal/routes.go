package signal

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.SignalHandler) {
	r.POST("/signal", h.Signal)
	r.POST("/webhook", h.LivekitWebhook)
}

func RegisterProtected(r *gin.RouterGroup, h *handler.SignalHandler, cf *handler.CloudflareHandler) {
	r.GET("/rooms", h.ListRooms)
	r.GET("/participants", h.ListParticipants)
	r.POST("/token", h.GetJoinToken)
	r.GET("/ws-ticket", h.GetWSTicket)
	if cf != nil {
		r.POST("/cloudflare/sessions/:sessionId/tracks/new", cf.AddTracks)
		r.PUT("/cloudflare/sessions/:sessionId/renegotiate", cf.Renegotiate)
		r.PUT("/cloudflare/sessions/:sessionId/tracks/close", cf.CloseTracks)
		r.DELETE("/cloudflare/sessions/:sessionId", cf.DeleteSession)
	}
}
