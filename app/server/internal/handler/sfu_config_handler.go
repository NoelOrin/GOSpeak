package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type SFUConfigHandler struct {
	svc *service.SFUConfigService
}

func NewSFUConfigHandler(svc *service.SFUConfigService) *SFUConfigHandler {
	return &SFUConfigHandler{svc: svc}
}

func (h *SFUConfigHandler) Get(c *gin.Context) {
	cfg, err := h.svc.Get()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, cfg)
}

func (h *SFUConfigHandler) Update(c *gin.Context) {
	var req service.UpdateSFUConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	cfg, err := h.svc.UpdateFromDTO(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, cfg)
}
