package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/signal"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

type SignalHandler struct {
	sfuProvider sfu.Provider
	hub         *signal.Hub
}

type sfuClientInfoProvider interface {
	ProviderName() string
	ClientInfo() map[string]interface{}
}

type streamInfoProvider interface {
	StreamInfo(room, identity string) (stream, token string)
}

func NewSignalHandler(provider sfu.Provider, hub *signal.Hub) *SignalHandler {
	return &SignalHandler{sfuProvider: provider, hub: hub}
}

// JoinRoomRequest 加入房间请求
type JoinRoomRequest struct {
	Room     string `json:"room" binding:"required" example:"my-room"`
	Identity string `json:"identity" binding:"required" example:"user-123"`
	Password string `json:"password,omitempty"`
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

	// 禁言检查
	if h.hub != nil {
		if muted, _, _ := h.hub.IsIdentityMuted(req.Identity); muted {
			pkg.Fail(c, pkg.USER_MUTED)
			return
		}
	}

	// 服务端校验房间人数上限
	if h.hub != nil {
		if full, limit, count, _ := h.hub.CheckRoomLimit(req.Room); full {
			pkg.Fail(c, pkg.FORBIDDEN, fmt.Sprintf("room is full (%d/%d)", count, limit))
			return
		}
	}

	// 密码校验
	if h.hub != nil {
		if ok, err := h.hub.CheckRoomPassword(req.Room, req.Password); !ok {
			if err != nil {
				pkg.Fail(c, pkg.FORBIDDEN, "room requires password")
				return
			}
			pkg.Fail(c, pkg.FORBIDDEN, "wrong room password")
			return
		}
	}

	token, err := h.sfuProvider.GenerateToken(req.Room, req.Identity)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	data := gin.H{
		"token":     token,
		"serverUrl": h.sfuProvider.GetHost(),
		"room":      req.Room,
		"identity":  req.Identity,
	}
	if provider, ok := h.sfuProvider.(sfuClientInfoProvider); ok {
		data["provider"] = provider.ProviderName()
		for key, value := range provider.ClientInfo() {
			data[key] = value
		}
	}

	if sp, ok := h.sfuProvider.(streamInfoProvider); ok {
		stream, streamToken := sp.StreamInfo(req.Room, req.Identity)
		data["stream"] = stream
		data["streamToken"] = streamToken
	}

	pkg.Success(c, data)
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
		pkg.Success(c, []interface{}{})
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
		pkg.Success(c, []interface{}{})
		return
	}

	pkg.Success(c, participants)
}

// LivekitWebhook
// @Summary      接收 LiveKit Webhook 事件
// @Description  接收 LiveKit 服务端推送的事件（参与者加入/离开、Track 发布/取消等）
// @Tags         信令
// @Accept       json
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Router       /signal/webhook [post]
func (h *SignalHandler) LivekitWebhook(c *gin.Context) {
	var event map[string]interface{}
	if err := c.ShouldBindJSON(&event); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	eventType, _ := event["event"].(string)
	log.Printf("[Webhook] received event: %s", eventType)

	pkg.Success(c, "ok")
}
