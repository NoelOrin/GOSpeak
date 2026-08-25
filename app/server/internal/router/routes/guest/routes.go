package guest

import (
	"time"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Register 注册访客公开接口（挂载在 /auth 组下），带签发限流。
func Register(r *gin.RouterGroup, h *handler.GuestHandler) {
	r.POST("/guest", middleware.RateLimit(10, time.Hour), h.Join)
}

// RegisterProtected 注册访客管理端与续约接口（JWT 保护组）。
func RegisterProtected(r *gin.RouterGroup, h *handler.GuestHandler) {
	r.POST("/domain/guest/config", h.Config)
	r.POST("/domain/guest/ban", h.Ban)
	r.POST("/domain/guest/unban", h.Unban)
	r.POST("/domain/guest/ban-list", h.BanList)
	r.POST("/domain/guest/cleanup", h.Cleanup)
	r.POST("/auth/guest/renew", h.Renew)
}
