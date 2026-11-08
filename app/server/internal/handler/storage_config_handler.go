package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// GetConfig
// @Summary      获取存储配置
// @Description  管理员获取存储配置（AK/SK 脱敏）
// @Tags         存储
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /storage/config [post]
func (h *StorageHandler) GetConfig(c *gin.Context) {
	cfg, err := h.storageService.GetConfig()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, service.ToPublicStorageConfig(cfg))
}

// TestConfig
// @Summary      测试存储连接
// @Description  使用提交的配置测试连接，不保存配置
// @Tags         存储
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  service.UpdateStorageConfigRequest  true  "配置请求"
// @Success      200   {object}  pkg.Response
// @Router       /storage/test-config [post]
func (h *StorageHandler) TestConfig(c *gin.Context) {
	var req service.UpdateStorageConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.storageService.TestConnectionFromDTO(req); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{"ok": true})
}

// UpdateConfig
// @Summary      更新存储配置
// @Description  管理员更新存储配置
// @Tags         存储
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  service.UpdateStorageConfigRequest  true  "配置请求"
// @Success      200   {object}  pkg.Response
// @Router       /storage/update-config [post]
func (h *StorageHandler) UpdateConfig(c *gin.Context) {
	var req service.UpdateStorageConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	cfg, err := h.storageService.UpdateConfigFromDTO(req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, cfg)
}
