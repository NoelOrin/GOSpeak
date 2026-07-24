package handler

import (
	"GOSpeak/internal/sfu/providers/cloudflare"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type CloudflareHandler struct {
	mediaSvc *service.CloudflareMediaService
}

func NewCloudflareHandler(mediaSvc *service.CloudflareMediaService) *CloudflareHandler {
	return &CloudflareHandler{mediaSvc: mediaSvc}
}

func (h *CloudflareHandler) AddTracks(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var req cloudflare.TrackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.mediaSvc.AddTracks(sessionID, &req)
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
	if err := h.mediaSvc.Renegotiate(sessionID, &req); err != nil {
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
	resp, err := h.mediaSvc.CloseTracks(sessionID, &req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, resp)
}

func (h *CloudflareHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if err := h.mediaSvc.DeleteSession(sessionID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}
