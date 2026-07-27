package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	svc *service.MessageService
}

func NewMessageHandler(svc *service.MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
}

func (h *MessageHandler) List(c *gin.Context) {
	var req struct {
		Room   string `json:"room" binding:"required"`
		Before string `json:"before"`
		Limit  int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	out, err := h.svc.List(c.Request.Context(), req.Room, req.Before, req.Limit)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}
