package signal

import (
	"go_rtc/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.SignalHandler) {
	r.POST("/token", h.GetJoinToken)
	r.POST("/signal", h.Signal)
	r.GET("/rooms", h.ListRooms)
	r.GET("/participants", h.ListParticipants)
}
