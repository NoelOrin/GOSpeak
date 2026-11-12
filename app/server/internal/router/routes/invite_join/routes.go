package invite_join

import (
	"time"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterPublic 注册公开的内联注册端点，按 IP 限流 20 次/小时。
func RegisterPublic(r *gin.RouterGroup, h *handler.InviteJoinHandler) {
	r.POST("/register", middleware.RateLimit(20, time.Hour), h.Register)
}

// RegisterProtected 注册已登录用户的加入与申请端点。
func RegisterProtected(r *gin.RouterGroup, h *handler.InviteJoinHandler) {
	r.POST("/join", h.Join)
	r.POST("/my-request", h.MyRequest)
	r.POST("/cancel-request", h.CancelRequest)
}

// RegisterDomainAdmin 注册域管理端的加入审核端点（protected 组，权限在 handler 内按域校验）。
func RegisterDomainAdmin(r *gin.RouterGroup, h *handler.JoinRequestHandler) {
	r.POST("/join-requests/list", h.List)
	r.POST("/join-requests/review", h.Review)
	r.POST("/join-approval/config", h.Config)
}
