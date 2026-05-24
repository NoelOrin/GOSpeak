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

// JoinRoomRequest 加入房间请求
type JoinRoomRequest struct {
	Room     string `json:"room" binding:"required" example:"my-room"`
	Identity string `json:"identity" binding:"required" example:"user-123"`
}

// GetJoinToken
// @Summary      Get LiveKit join token
// @Description  Generate a LiveKit access token for joining a room
// @Tags         Signal
// @Accept       json
// @Produce      json
// @Param        request  body      JoinRoomRequest  true  "Room and identity"
// @Success      200      {object}  pkg.Response
// @Failure      400      {object}  pkg.Response
// @Router       /signal/token [post]
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

// SignalRequest 信令请求
type SignalRequest struct {
	Type     string      `json:"type" binding:"required" example:"offer"`
	Room     string      `json:"room,omitempty" example:"my-room"`
	Identity string      `json:"identity,omitempty" example:"user-123"`
	Data     interface{} `json:"data,omitempty"`
}

// Signal
// @Summary      Exchange signaling message
// @Description  Relay WebRTC signaling messages (offer/answer/ICE candidate)
// @Tags         Signal
// @Accept       json
// @Produce      json
// @Param        request  body      SignalRequest  true  "Signaling message"
// @Success      200      {object}  pkg.Response
// @Failure      400      {object}  pkg.Response
// @Router       /signal/signal [post]
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

// ListRooms
// @Summary      List LiveKit rooms
// @Description  Get all active rooms from LiveKit
// @Tags         Signal
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Failure      500  {object}  pkg.Response
// @Router       /signal/rooms [get]
func (h *SignalHandler) ListRooms(c *gin.Context) {
	rooms, err := h.liveKitSvc.ListRooms()
	if err != nil {
		pkg.FailWithMsg(c, pkg.INTERNAL_ERROR, err.Error())
		return
	}

	pkg.Success(c, rooms)
}

// ListParticipants
// @Summary      List room participants
// @Description  Get all participants in a specific LiveKit room
// @Tags         Signal
// @Produce      json
// @Param        room  query     string  true  "Room name"
// @Success      200   {object}  pkg.Response
// @Failure      400   {object}  pkg.Response
// @Router       /signal/participants [get]
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