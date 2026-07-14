package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// SFUSwitchNotifier SFU 热切换后通知信令层强制断连。
type SFUSwitchNotifier interface {
	ForceSFUProviderSwitch(provider string)
}

type SFUConfigHandler struct {
	svc      *service.SFUConfigService
	notifier SFUSwitchNotifier
}

func NewSFUConfigHandler(svc *service.SFUConfigService, notifier SFUSwitchNotifier) *SFUConfigHandler {
	return &SFUConfigHandler{svc: svc, notifier: notifier}
}

// Get 返回当前激活 provider 的配置（密钥已脱敏）。
func (h *SFUConfigHandler) Get(c *gin.Context) {
	cfg, err := h.svc.Get()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, service.ToPublicSFUConfig(cfg))
}

// GetProvider 返回指定 provider 的配置（密钥已脱敏）。
func (h *SFUConfigHandler) GetProvider(c *gin.Context) {
	provider := c.Param("provider")
	cfg, err := h.svc.GetProviderConfig(provider)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, service.ToPublicSFUConfig(cfg))
}

// Update 更新指定 provider 的配置，并将其设为当前激活。
// 请求中密钥字段为空时保留旧值，响应中密钥已脱敏。
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
	if h.notifier != nil {
		h.notifier.ForceSFUProviderSwitch(cfg.Provider)
	}
	pkg.Success(c, service.ToPublicSFUConfig(cfg))
}

// SwitchProvider 切换当前激活的 provider，不修改配置。
func (h *SFUConfigHandler) SwitchProvider(c *gin.Context) {
	var req struct {
		Provider string `json:"provider" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	cfg, err := h.svc.SwitchProvider(req.Provider)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	if h.notifier != nil {
		h.notifier.ForceSFUProviderSwitch(req.Provider)
	}
	pkg.Success(c, service.ToPublicSFUConfig(cfg))
}

// ListProviders 返回所有已配置 provider 的列表 + 当前激活的 provider。
func (h *SFUConfigHandler) ListProviders(c *gin.Context) {
	cfgs, active, err := h.svc.ListProviders()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{
		"providers": service.ToPublicSFUConfigs(cfgs),
		"active":    active,
	})
}
