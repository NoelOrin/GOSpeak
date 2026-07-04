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
	pkg.Success(c, gin.H{
		"enabled":             cfg.Enabled,
		"smtp_host":           cfg.SMTPHost,
		"smtp_port":           cfg.SMTPPort,
		"smtp_username":       cfg.SMTPUsername,
		"smtp_password":       "",
		"smtp_from":           cfg.SMTPFrom,
		"smtp_from_name":      cfg.SMTPFromName,
		"email_code_ttl":      cfg.EmailCodeTTL,
		"email_send_cooldown": cfg.EmailSendCooldown,
		"email_code_secret":   "",
		"available":           h.svc.IsVerificationAvailable(),
	})
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
	pkg.Success(c, gin.H{
		"enabled":             cfg.Enabled,
		"smtp_host":           cfg.SMTPHost,
		"smtp_port":           cfg.SMTPPort,
		"smtp_username":       cfg.SMTPUsername,
		"smtp_password":       "",
		"smtp_from":           cfg.SMTPFrom,
		"smtp_from_name":      cfg.SMTPFromName,
		"email_code_ttl":      cfg.EmailCodeTTL,
		"email_send_cooldown": cfg.EmailSendCooldown,
		"email_code_secret":   "",
		"available":           h.svc.IsVerificationAvailable(),
	})
}
