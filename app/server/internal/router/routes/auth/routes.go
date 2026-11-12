package auth

import (
	"time"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.AuthHandler) {
	// 登录/注册/重置密码保持 10次/分严格限流；refresh_token 60次/分且独立桶，避免切路由频繁触发 ensureSession 被误踢。
	r.POST("/login", middleware.RateLimit(10, time.Minute), h.Login)
	r.POST("/register", middleware.RateLimit(10, time.Minute), h.Register)
	r.POST("/refresh_token", middleware.RateLimit(60, time.Minute), h.GetRefreshToken)
	r.POST("/reset_password", middleware.RateLimit(10, time.Minute), h.ResetPassword)
}

func RegisterProtected(r *gin.RouterGroup, h *handler.AuthHandler) {
	r.POST("/logout", h.Logout)
	r.POST("/refresh", h.RefreshToken)
	r.POST("/change_password", h.ChangePassword)
	r.POST("/first_change_password", h.FirstChangePassword)
}
