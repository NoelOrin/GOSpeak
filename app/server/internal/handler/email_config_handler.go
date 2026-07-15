package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type EmailConfigHandler struct {
	svc *service.EmailConfigService
}

func NewEmailConfigHandler(svc *service.EmailConfigService) *EmailConfigHandler {
	return &EmailConfigHandler{svc: svc}
}

func (h *EmailConfigHandler) Get(c *gin.Context) {
	cfg, err := h.svc.Get()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, service.ToPublicEmailConfig(cfg, h.svc.IsVerificationAvailable()))
}

func (h *EmailConfigHandler) Update(c *gin.Context) {
	var req service.UpdateEmailConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	cfg, err := h.svc.UpdateFromDTO(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, service.ToPublicEmailConfig(cfg, h.svc.IsVerificationAvailable()))
}
