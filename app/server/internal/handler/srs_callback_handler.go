package handler

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"GOSpeak/internal/sfu"
	"GOSpeak/internal/sfu/providers/srs"
	"GOSpeak/internal/signal"

	"github.com/gin-gonic/gin"
)

type streamJobPublisher interface {
	PublishSRS(ctx context.Context, action, stream string) error
}

type SRSCallbackHandler struct {
	hub           *signal.Hub
	secret        string
	resolveSecret func() string
	jobs          streamJobPublisher
	muteStore     sfu.MuteRuleStore
}

func NewSRSCallbackHandlerWithResolver(hub *signal.Hub, resolve func() string) *SRSCallbackHandler {
	return &SRSCallbackHandler{hub: hub, resolveSecret: resolve}
}

func (h *SRSCallbackHandler) SetJobs(j streamJobPublisher) {
	h.jobs = j
}

// SetMuteRuleStore 注入 SRS 禁推黑名单（与 srs.Service 共用同一 store）。
func (h *SRSCallbackHandler) SetMuteRuleStore(store sfu.MuteRuleStore) {
	h.muteStore = store
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
		c.JSON(http.StatusOK, gin.H{"code": 1})
		return
	}
	stream := stripAppPrefix(p.Stream)
	params := parseCallbackParams(p.Param)
	secret := h.currentSecret()

	switch p.Action {
	case "on_publish":
		token := params["token"]
		if token == "" || !srs.ValidateStreamToken(stream, token, secret) {
			c.JSON(http.StatusOK, gin.H{"code": 1})
			return
		}
		if h.muteStore != nil {
			blocked, _ := h.muteStore.Get(c.Request.Context(), srs.PublishBlockKey(stream))
			if blocked > 0 {
				c.JSON(http.StatusOK, gin.H{"code": 1})
				return
			}
		}
		if h.jobs != nil {
			_ = h.jobs.PublishSRS(c.Request.Context(), "on_publish", stream)
		} else {
			h.hub.RegisterStream(stream)
		}
		c.JSON(http.StatusOK, gin.H{"code": 0})
	case "on_unpublish":
		token := params["token"]
		if token == "" || !srs.ValidateStreamToken(stream, token, secret) {
			c.JSON(http.StatusOK, gin.H{"code": 1})
			return
		}
		if h.jobs != nil {
			_ = h.jobs.PublishSRS(c.Request.Context(), "on_unpublish", stream)
		} else {
			h.hub.UnregisterStream(stream)
		}
		c.JSON(http.StatusOK, gin.H{"code": 0})
	case "on_stop":
		c.JSON(http.StatusOK, gin.H{"code": 0})
	case "on_play":
		if !h.hub.IsStreamActive(stream) {
			c.JSON(http.StatusOK, gin.H{"code": 1})
			return
		}
		if !h.authorizePlay(stream, params["token"], secret) {
			c.JSON(http.StatusOK, gin.H{"code": 1})
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
	values, err := url.ParseQuery(param)
	if err != nil {
		return out
	}
	for k, v := range values {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
