package handler

import (
	"go_rtc/internal/livekit"
	"go_rtc/internal/pkg"

	"github.com/gin-gonic/gin"
)

type SignalHandler struct {
	liveKitSvc *livekit.Service
}

func NewSignalHandler(liveKitSvc *livekit.Service) *SignalHandler {
	return &SignalHandler{liveKitSvc: liveKitSvc}
}

type JoinRoomRequest struct {
	Room     string `json:"room" binding:"required"`
	Identity string `json:"identity" binding:"required"`
}

func (h *SignalHandler) GetJoinToken(c *gin.Context) {
	var req JoinRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.FailWithMsg(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	token, err := h.liveKitSvc.GenerateToken(req.Room, req.Identity)
	if err != nil {
		pkg.FailWithMsg(c, pkg.INTERNAL_ERROR, err.Error())
		return
	}

	pkg.Success(c, gin.H{
		"token":    token,
		"room":     req.Room,
		"identity": req.Identity,
	})
}

type SignalRequest struct {
	Type     string `json:"type" binding:"required"`
	Room     string `json:"room,omitempty"`
	Identity string `json:"identity,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

func (h *SignalHandler) Signal(c *gin.Context) {
	var req SignalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.FailWithMsg(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	pkg.Success(c, gin.H{
		"type": req.Type,
		"room": req.Room,
	})
}

func (h *SignalHandler) ListRooms(c *gin.Context) {
	rooms, err := h.liveKitSvc.ListRooms()
	if err != nil {
		pkg.FailWithMsg(c, pkg.INTERNAL_ERROR, err.Error())
		return
	}

	pkg.Success(c, rooms)
}

type RoomParticipantRequest struct {
	Room string `form:"room" binding:"required"`
}

func (h *SignalHandler) ListParticipants(c *gin.Context) {
	room := c.Query("room")
	if room == "" {
		pkg.FailWithMsg(c, pkg.INVALID_PARAMS, "room is required")
		return
	}

	participants, err := h.liveKitSvc.ListParticipants(room)
	if err != nil {
		pkg.FailWithMsg(c, pkg.INTERNAL_ERROR, err.Error())
		return
	}

	pkg.Success(c, participants)
}