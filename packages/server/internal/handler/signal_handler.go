package handler

import (
	"go_rtc/internal/pkg"
	"go_rtc/internal/sfu"

	"github.com/gin-gonic/gin"
)

type SignalHandler struct {
	sfuProvider sfu.Provider
}

func NewSignalHandler(provider sfu.Provider) *SignalHandler {
	return &SignalHandler{sfuProvider: provider}
}

// JoinRoomRequest 加入房间请求
type JoinRoomRequest struct {
	Room     string `json:"room" binding:"required" example:"my-room"`
	Identity string `json:"identity" binding:"required" example:"user-123"`
}

// GetJoinToken
// @Summary      获取 LiveKit 加入 token
// @Description  生成用于加入房间的 LiveKit 访问 token
// @Tags         信令
// @Accept       json
// @Produce      json
// @Param        request  body      JoinRoomRequest  true  "房间和身份标识"
// @Success      200      {object}  pkg.Response
// @Router       /signal/token [post]
func (h *SignalHandler) GetJoinToken(c *gin.Context) {
	var req JoinRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	token, err := h.sfuProvider.GenerateToken(req.Room, req.Identity)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{
		"access_token": token,
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
// @Summary      交换信令消息
// @Description  中继 WebRTC 信令消息（offer/answer/ICE candidate）
// @Tags         信令
// @Accept       json
// @Produce      json
// @Param        request  body      SignalRequest  true  "信令消息"
// @Success      200      {object}  pkg.Response
// @Router       /signal/signal [post]
func (h *SignalHandler) Signal(c *gin.Context) {
	var req SignalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	pkg.Success(c, gin.H{
		"type": req.Type,
		"room": req.Room,
	})
}

// ListRooms
// @Summary      获取房间列表
// @Description  从 LiveKit 获取所有活跃房间
// @Tags         信令
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Router       /signal/rooms [get]
func (h *SignalHandler) ListRooms(c *gin.Context) {
	rooms, err := h.sfuProvider.ListRooms()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, rooms)
}

// ListParticipants
// @Summary      获取房间参与者
// @Description  获取指定 LiveKit 房间中的所有参与者
// @Tags         信令
// @Produce      json
// @Param        room  query     string  true  "房间名称"
// @Success      200   {object}  pkg.Response
// @Router       /signal/participants [get]
func (h *SignalHandler) ListParticipants(c *gin.Context) {
	room := c.Query("room")
	if room == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "room is required")
		return
	}

	participants, err := h.sfuProvider.ListParticipants(room)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, participants)
}
