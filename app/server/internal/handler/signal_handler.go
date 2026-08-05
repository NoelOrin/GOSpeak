package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type livekitJobPublisher interface {
	PublishLiveKit(ctx context.Context, raw []byte) error
}

const (
	livekitSignatureHeader = "Livekit-Signature"
	livekitSignatureMaxAge = 5 * time.Minute
)

type SignalHandler struct {
	sfuSvc *service.SFUService

	jobs livekitJobPublisher

	clusterResolver func(domainUUID string) (string, error)

	resolveLiveKitSecret func() string
}

func NewSignalHandler(sfuSvc *service.SFUService) *SignalHandler {
	return &SignalHandler{sfuSvc: sfuSvc}
}

// SetClusterResolver 注入 Domain/Server → workerUrl 的解析器，用于多副本信令路由。
func (h *SignalHandler) SetClusterResolver(fn func(domainUUID string) (string, error)) {
	h.clusterResolver = fn
}

func (h *SignalHandler) SetJobs(j livekitJobPublisher) {
	h.jobs = j
}

// SetLiveKitSecretResolver 注入当前 LiveKit API secret 解析器（env/DB 配置）。
func (h *SignalHandler) SetLiveKitSecretResolver(fn func() string) {
	h.resolveLiveKitSecret = fn
}

func (h *SignalHandler) currentLiveKitSecret() string {
	if h.resolveLiveKitSecret == nil {
		return ""
	}
	return strings.TrimSpace(h.resolveLiveKitSecret())
}

// verifyLiveKitSignature 校验 LiveKit-Signature: t=<unix ts>,v1=<hex hmac-sha256>。
func verifyLiveKitSignature(header string, body []byte, secret string, now time.Time) bool {
	if secret == "" {
		return false
	}
	var tsStr, sigHex string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "t":
			tsStr = strings.TrimSpace(value)
		case "v1":
			sigHex = strings.TrimSpace(value)
		}
	}
	if tsStr == "" || sigHex == "" {
		return false
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil || ts <= 0 {
		return false
	}
	diff := now.Unix() - ts
	if diff < 0 {
		diff = -diff
	}
	if diff > int64(livekitSignatureMaxAge/time.Second) {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(tsStr + "." + string(body)))
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != sha256.Size {
		return false
	}
	return subtle.ConstantTimeCompare(sig, mac.Sum(nil)) == 1
}

// GetWSTicket
// @Summary      获取 WebSocket 短时 ticket
// @Description  签发只用于 WS 握手的短时 ticket，避免 JWT 出现在 URL query 和访问日志中
// @Tags         信令
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /signal/ws-ticket [get]
func (h *SignalHandler) GetWSTicket(c *gin.Context) {
	claimsVal, ok := c.Get("claims")
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}
	claims, ok := claimsVal.(*pkg.Claims)
	if !ok || claims == nil || claims.Username == "" {
		pkg.Fail(c, pkg.TOKEN_WRONG)
		return
	}
	ticket, err := pkg.GenerateWSTicket(claims.Username, claims.DisplayName, claims.UserUUID, claims.Role, claims.TokenVersion)
	if err != nil {
		pkg.Fail(c, pkg.INTERNAL_ERROR)
		return
	}
	pkg.Success(c, gin.H{"ticket": ticket})
}

// JoinRoomRequest 加入房间请求
type JoinRoomRequest struct {
	Room       string `json:"room" binding:"required" example:"my-room"`
	DomainUUID string `json:"domain_uuid,omitempty"`
	Identity   string `json:"identity,omitempty" example:"user-123"` // 兼容字段，服务端以 JWT username 覆盖
	Password   string `json:"password,omitempty"`
}

// GetJoinToken
// @Summary      获取加入 token
// @Description  生成用于加入房间的访问 token（禁言/限流/密码校验在 service 层）
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

	// 身份以 JWT 为准，拒绝客户端伪造他人 identity
	username, ok := c.Get("username")
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}
	identity, ok := username.(string)
	if !ok || identity == "" {
		pkg.Fail(c, pkg.TOKEN_WRONG, "invalid token identity")
		return
	}
	req.Identity = identity

	result, err := h.sfuSvc.GetJoinToken(req.DomainUUID, req.Room, req.Identity, currentUserUUID(c), req.Password)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	data := gin.H{
		"token":       result.Token,
		"serverUrl":   result.ServerURL,
		"room":        req.Room,
		"identity":    req.Identity,
		"sfuRoom":     result.SFURoom,
		"domain_uuid": req.DomainUUID,
	}
	if req.DomainUUID != "" && h.clusterResolver != nil {
		if workerURL, resolveErr := h.clusterResolver(req.DomainUUID); resolveErr == nil && workerURL != "" {
			data["workerUrl"] = workerURL
		}
	}
	if result.Provider != "" {
		data["provider"] = result.Provider
	}
	data["capabilities"] = result.Capabilities
	for key, value := range result.ClientInfo {
		data[key] = value
	}
	if result.Stream != "" {
		data["stream"] = result.Stream
		data["streamToken"] = result.StreamToken
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
// @Description  从 SFU 获取所有活跃房间
// @Tags         信令
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Router       /signal/rooms [get]
func (h *SignalHandler) ListRooms(c *gin.Context) {
	rooms, err := h.sfuSvc.ListRooms()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, rooms)
}

// ListParticipants
// @Summary      获取房间参与者
// @Description  获取指定房间中的所有参与者
// @Tags         信令
// @Produce      json
// @Param        room  query     string  true  "房间名称"
// @Success      200   {object}  pkg.Response
// @Router       /signal/participants [get]
func (h *SignalHandler) ListParticipants(c *gin.Context) {
	room := pkg.RoomKey(c.Query("domain_uuid"), c.Query("room"))
	if room == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "room is required")
		return
	}

	participants, err := h.sfuSvc.ListParticipants(room)
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
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	secret := h.currentLiveKitSecret()
	if secret == "" {
		pkg.Fail(c, pkg.SFU_NOT_CONFIGURED)
		return
	}
	if !verifyLiveKitSignature(c.GetHeader(livekitSignatureHeader), body, secret, time.Now()) {
		pkg.Fail(c, pkg.TOKEN_WRONG, "invalid webhook signature")
		return
	}
	var event map[string]interface{}
	if err := json.Unmarshal(body, &event); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if h.jobs != nil {
		if err := h.jobs.PublishLiveKit(c.Request.Context(), body); err != nil {
			log.Printf("[Webhook] enqueue livekit event failed: %v", err)
			pkg.Fail(c, pkg.INTERNAL_ERROR, "webhook enqueue failed")
			return
		}
	} else {
		eventType, _ := event["event"].(string)
		log.Printf("[Webhook] livekit event (sync no-queue): %v", eventType)
	}
	pkg.Success(c, "ok")
}
