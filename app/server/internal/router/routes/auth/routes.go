package auth

import (
	"time"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.AuthHandler) {
	public := r.Group("")
	public.Use(middleware.RateLimit(10, time.Minute))
	public.POST("/login", h.Login)
	public.POST("/register", h.Register)
	public.POST("/refresh_token", h.GetRefreshToken)
	public.POST("/reset_password", h.ResetPassword)
}

func RegisterProtected(r *gin.RouterGroup, h *handler.AuthHandler) {
	r.POST("/logout", h.Logout)
	r.POST("/refresh", h.RefreshToken)
	r.POST("/change_password", h.ChangePassword)
	r.POST("/first_change_password", h.FirstChangePassword)
}
