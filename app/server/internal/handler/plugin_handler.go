package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type PluginHandler struct {
	svc *service.PluginService
}

func NewPluginHandler(svc *service.PluginService) *PluginHandler {
	return &PluginHandler{svc: svc}
}

// List
// @Summary      插件列表
// @Tags         Plugin
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} pkg.Response
// @Router       /plugins/list [post]
func (h *PluginHandler) List(c *gin.Context) {
	pkg.Success(c, h.svc.List())
}

// Get
// @Summary      插件详情
// @Tags         Plugin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body object{name=string} true "插件名"
// @Success      200 {object} pkg.Response
// @Router       /plugins/get [post]
func (h *PluginHandler) Get(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	info, err := h.svc.Get(req.Name)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, info)
}

// Update
// @Summary      更新插件配置
// @Tags         Plugin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body service.UpdatePluginConfigInput true "配置"
// @Success      200 {object} pkg.Response
// @Router       /plugins/update [post]
func (h *PluginHandler) Update(c *gin.Context) {
	var req service.UpdatePluginConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	info, err := h.svc.Update(req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, info)
}
