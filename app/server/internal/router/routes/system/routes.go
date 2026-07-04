package system

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.MonitorHandler) {
	r.GET("/stream", h.HealthStream)
}
