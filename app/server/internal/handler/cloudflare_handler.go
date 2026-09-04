package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu/providers/cloudflare"

	"github.com/gin-gonic/gin"
)

// cloudflareMediaService is the Cloudflare media service surface used by handlers.
type cloudflareMediaService interface {
	AddTracks(sessionID, userUUID string, req *cloudflare.TrackRequest) (*cloudflare.TracksResponse, error)
	Renegotiate(sessionID, userUUID string, req *cloudflare.RenegotiateRequest) error
	CloseTracks(sessionID, userUUID string, req *cloudflare.CloseTrackRequest) (*cloudflare.CloseTrackResponse, error)
	DeleteSession(sessionID, userUUID string) error
}

type CloudflareHandler struct {
	mediaSvc cloudflareMediaService
}

func NewCloudflareHandler(mediaSvc cloudflareMediaService) *CloudflareHandler {
	return &CloudflareHandler{mediaSvc: mediaSvc}
}

func (h *CloudflareHandler) AddTracks(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var req cloudflare.TrackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.mediaSvc.AddTracks(sessionID, currentUserUUID(c), &req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, resp)
}

func (h *CloudflareHandler) Renegotiate(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var req cloudflare.RenegotiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.mediaSvc.Renegotiate(sessionID, currentUserUUID(c), &req); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *CloudflareHandler) CloseTracks(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var req cloudflare.CloseTrackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.mediaSvc.CloseTracks(sessionID, currentUserUUID(c), &req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, resp)
}

func (h *CloudflareHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if err := h.mediaSvc.DeleteSession(sessionID, currentUserUUID(c)); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}
