package handler

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"GOSpeak/internal/livekitproxy"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/livekit/protocol/auth"
)

type livekitJobPublisher interface {
	PublishLiveKit(ctx context.Context, raw []byte) error
}

const (
	// LiveKit 用 Authorization: <JWT> 推送 webhook，JWT 内含 body 的 sha256，
	// 由 verifyLiveKitWebhook 校验。此前按一个 LiveKit 从不发送的
	// HMAC 头校验，导致所有 webhook 都被判 401。
	livekitAuthHeader = "Authorization"
)

type SignalHandler struct {
	sfuSvc *service.SFUService

	jobs livekitJobPublisher

	resolveLiveKitSecret func() string
	resolveLiveKitKey    func() string
}

func NewSignalHandler(sfuSvc *service.SFUService) *SignalHandler {
	return &SignalHandler{sfuSvc: sfuSvc}
}

func (h *SignalHandler) SetJobs(j livekitJobPublisher) {
	h.jobs = j
}

// SetLiveKitSecretResolver 注入当前 LiveKit API secret 解析器（env/DB 配置）。
func (h *SignalHandler) SetLiveKitSecretResolver(fn func() string) {
	h.resolveLiveKitSecret = fn
}

// SetLiveKitKeyResolver 注入当前 LiveKit API key 解析器（env/DB 配置）。
// webhook JWT 的 iss 是 api key，必须与 secret 配对才能验签。
func (h *SignalHandler) SetLiveKitKeyResolver(fn func() string) {
	h.resolveLiveKitKey = fn
}

func (h *SignalHandler) currentLiveKitSecret() string {
	if h.resolveLiveKitSecret == nil {
		return ""
	}
	return strings.TrimSpace(h.resolveLiveKitSecret())
}

func (h *SignalHandler) currentLiveKitKey() string {
	if h.resolveLiveKitKey == nil {
		return ""
	}
	return strings.TrimSpace(h.resolveLiveKitKey())
}

// verifyLiveKitWebhook 校验 LiveKit webhook：Authorization 头是 LiveKit 用
// API key/secret 签发的 JWT，claims.sha256 是 body 摘要。
// 与 protocol/webhook.Receive 等价，但只依赖已在用的 auth 包。
func verifyLiveKitWebhook(r *http.Request, apiKey, apiSecret string) ([]byte, error) {
	if apiKey == "" || apiSecret == "" {
		return nil, errLiveKitNotConfigured
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	_ = r.Body.Close()

	token := r.Header.Get(livekitAuthHeader)
	if token == "" {
		return nil, errLiveKitNoAuthHeader
	}
	verifier, err := auth.ParseAPIToken(token)
	if err != nil {
		return nil, err
	}
	if verifier.APIKey() != apiKey {
		return nil, errLiveKitWrongAPIKey
	}
	_, claims, err := verifier.Verify(apiSecret)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	if subtle.ConstantTimeCompare([]byte(claims.Sha256), []byte(base64.StdEncoding.EncodeToString(sum[:]))) != 1 {
		return nil, errLiveKitChecksum
	}
	return body, nil
}

var (
	errLiveKitNotConfigured = errors.New("livekit not configured")
	errLiveKitNoAuthHeader  = errors.New("webhook authorization header missing")
	errLiveKitWrongAPIKey   = errors.New("webhook api key mismatch")
	errLiveKitChecksum      = errors.New("webhook body checksum mismatch")
)

// GetWSEndpoint
// @Summary      获取 WebSocket 连接地址
// @Description  按域解析 Worker 节点地址；WS 鉴权统一使用 HttpOnly access cookie
// @Tags         信令
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /signal/ws-endpoint [get]
func (h *SignalHandler) GetWSEndpoint(c *gin.Context) {
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
	data := gin.H{}
	pkg.Success(c, data)
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

	// 浏览器侧地址留空 = 走应用内置同源反代：按当前请求推导 wss://<域名>/livekit。
	// LiveKit 因此不需要独立域名/证书，反代层也只需面对应用一个上游。
	serverURL := result.ServerURL
	if strings.TrimSpace(serverURL) == "" {
		serverURL = livekitproxy.ServerURLForRequest(c.Request)
	}

	data := gin.H{
		"token":       result.Token,
		"serverUrl":   serverURL,
		"room":        req.Room,
		"identity":    req.Identity,
		"sfuRoom":     result.SFURoom,
		"domain_uuid": req.DomainUUID,
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
	body, err := verifyLiveKitWebhook(c.Request, h.currentLiveKitKey(), h.currentLiveKitSecret())
	if err != nil {
		if errors.Is(err, errLiveKitNotConfigured) {
			pkg.Fail(c, pkg.SFU_NOT_CONFIGURED)
			return
		}
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
