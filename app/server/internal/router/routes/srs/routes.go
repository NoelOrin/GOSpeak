package srs

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.SRSCallbackHandler) {
	r.POST("/callback", h.HandleCallback)
}
