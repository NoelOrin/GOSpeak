package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// RoomMemberChecker checks if identity is a room member.
type RoomMemberChecker interface {
	IsRoomMember(room, identity string) bool
}

type MessageHandler struct {
	svc         *service.MessageService
	memberCheck RoomMemberChecker
}

func NewMessageHandler(svc *service.MessageService, mc RoomMemberChecker) *MessageHandler {
	return &MessageHandler{svc: svc, memberCheck: mc}
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

	if h.memberCheck != nil {
		username, exists := c.Get("username")
		if !exists {
			pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
			return
		}
		name, ok := username.(string)
		if !ok {
			pkg.Fail(c, pkg.INTERNAL_ERROR, "invalid context")
			return
		}
		if !h.memberCheck.IsRoomMember(req.Room, name) {
			pkg.Fail(c, pkg.NOT_FOUND, "not in room")
			return
		}
	}

	out, err := h.svc.List(c.Request.Context(), req.Room, req.Before, req.Limit)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}
