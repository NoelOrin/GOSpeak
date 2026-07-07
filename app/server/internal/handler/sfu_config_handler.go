package handler

import (
	"GOSpeak/internal/model"
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

// Get 返回当前激活 provider 的配置。
func (h *SFUConfigHandler) Get(c *gin.Context) {
	cfg, err := h.svc.Get()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, cfg)
}

// GetProvider 返回指定 provider 的配置（从 URL 路径参数读取）。
func (h *SFUConfigHandler) GetProvider(c *gin.Context) {
	provider := c.Param("provider")
	cfg, err := h.svc.GetProviderConfig(provider)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, cfg)
}

// Update 更新指定 provider 的配置，并将其设为当前激活。
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
	pkg.Success(c, cfg)
}

// ListProviders 返回所有已配置 provider 的列表 + 当前激活的 provider。
func (h *SFUConfigHandler) ListProviders(c *gin.Context) {
	cfgs, active, err := h.svc.ListProviders()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	if cfgs == nil {
		cfgs = []model.SFUConfig{}
	}
	pkg.Success(c, gin.H{
		"providers": cfgs,
		"active":    active,
	})
}
