package cluster

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

// RegisterProtected 注册 Agent 控制面 API。
func RegisterProtected(r *gin.RouterGroup, h *handler.ClusterHandler) {
	r.POST("/nodes/register", middleware.RequirePermission(permcode.PermClusterManage), h.Register)
	r.POST("/nodes/heartbeat", middleware.RequirePermission(permcode.PermClusterManage), h.Heartbeat)
	r.POST("/nodes/deregister", middleware.RequirePermission(permcode.PermClusterManage), h.Deregister)
	r.POST("/nodes/drain", middleware.RequirePermission(permcode.PermClusterManage), h.Drain)
	r.POST("/nodes/undrain", middleware.RequirePermission(permcode.PermClusterManage), h.Undrain)
	r.POST("/nodes/list", middleware.RequirePermission(permcode.PermClusterRead), h.List)
	r.POST("/servers/scale", middleware.RequirePermission(permcode.PermClusterManage), h.Scale)
	r.POST("/servers/resolve", middleware.RequirePermission(permcode.PermClusterRead), h.Resolve)
}
