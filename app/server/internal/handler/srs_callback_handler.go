package handler

import (
	"net/http"
	"strings"

	"GOSpeak/internal/signal"
	"GOSpeak/internal/srs"

	"github.com/gin-gonic/gin"
)

type SRSCallbackHandler struct {
	hub           *signal.Hub
	secret        string
	resolveSecret func() string
}

func NewSRSCallbackHandler(hub *signal.Hub, secret string) *SRSCallbackHandler {
	return &SRSCallbackHandler{hub: hub, secret: secret}
}

func NewSRSCallbackHandlerWithResolver(hub *signal.Hub, resolve func() string) *SRSCallbackHandler {
	return &SRSCallbackHandler{hub: hub, resolveSecret: resolve}
}

func (h *SRSCallbackHandler) currentSecret() string {
	if h.resolveSecret != nil {
		if secret := strings.TrimSpace(h.resolveSecret()); secret != "" {
			return secret
		}
	}
	return h.secret
}

func (h *SRSCallbackHandler) HandleCallback(c *gin.Context) {
	var p srsCallbackPayload
	if err := c.ShouldBind(&p); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 403})
		return
	}
	stream := stripAppPrefix(p.Stream)
	params := parseCallbackParams(p.Param)
	secret := h.currentSecret()

	switch p.Action {
	case "on_publish":
		token := params["token"]
		if token == "" || !srs.ValidateStreamToken(stream, token, secret) {
			c.JSON(http.StatusOK, gin.H{"code": 403})
			return
		}
		h.hub.RegisterStream(stream)
		c.JSON(http.StatusOK, gin.H{"code": 0})
	case "on_unpublish":
		h.hub.UnregisterStream(stream)
		c.JSON(http.StatusOK, gin.H{"code": 0})
	case "on_stop":
		c.JSON(http.StatusOK, gin.H{"code": 0})
	case "on_play":
		if !h.hub.IsStreamActive(stream) {
			c.JSON(http.StatusOK, gin.H{"code": 403})
			return
		}
		if !h.authorizePlay(stream, params["token"], secret) {
			c.JSON(http.StatusOK, gin.H{"code": 403})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0})
	default:
		c.JSON(http.StatusOK, gin.H{"code": 0})
	}
}

func (h *SRSCallbackHandler) authorizePlay(stream, token, secret string) bool {
	return authorizeSRSPlay(stream, token, secret, h.hub.RoomForStream)
}

func authorizeSRSPlay(
	stream, token, secret string,
	roomForStream func(string) (string, bool),
) bool {
	if token == "" || secret == "" {
		return false
	}
	if srs.ValidateStreamToken(stream, token, secret) {
		return true
	}
	room, _, err := srs.ParseToken(token, secret)
	if err != nil || room == "" {
		return false
	}
	if roomForStream == nil {
		return false
	}
	streamRoom, ok := roomForStream(stream)
	if !ok {
		return false
	}
	return streamRoom == room
}

type srsCallbackPayload struct {
	Action string `json:"action" form:"action"`
	Stream string `json:"stream" form:"stream"`
	Param  string `json:"param" form:"param"`
}

func stripAppPrefix(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func parseCallbackParams(param string) map[string]string {
	out := map[string]string{}
	if param == "" {
		return out
	}
	for _, kv := range strings.Split(param, "&") {
		if i := strings.Index(kv, "="); i >= 0 {
			out[kv[:i]] = kv[i+1:]
		}
	}
	return out
}
