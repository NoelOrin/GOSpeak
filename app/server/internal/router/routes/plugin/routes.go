package plugin

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.PluginHandler) {
	r.POST("/list", middleware.RequirePermission(permcode.PermPluginRead), h.List)
	r.POST("/get", middleware.RequirePermission(permcode.PermPluginRead), h.Get)
	r.POST("/update", middleware.RequirePermission(permcode.PermPluginManage), h.Update)
}
