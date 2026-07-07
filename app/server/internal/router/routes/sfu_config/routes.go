package sfu_config

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.SFUConfigHandler) {
	// 获取当前激活 provider 的配置
	r.POST("/config", middleware.RequirePermission(permcode.PermSFUManage), h.Get)
	// 获取指定 provider 的配置
	r.POST("/config/:provider", middleware.RequirePermission(permcode.PermSFUManage), h.GetProvider)
	// 更新指定 provider 的配置并激活
	r.POST("/update-config", middleware.RequirePermission(permcode.PermSFUManage), h.Update)
	// 切换激活的 provider（不改配置）
	r.POST("/switch-provider", middleware.RequirePermission(permcode.PermSFUManage), h.SwitchProvider)
	// 列出所有已配置的 provider
	r.POST("/providers", middleware.RequirePermission(permcode.PermSFUManage), h.ListProviders)
}
