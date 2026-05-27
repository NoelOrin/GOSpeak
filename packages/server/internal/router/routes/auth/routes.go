package auth

import (
	"go_rtc/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.AuthHandler) {
	r.POST("/login", h.Login)
	r.POST("/register", h.Register)
	r.POST("/refresh_token", h.GetRefreshToken)
	r.POST("/reset_password", h.ResetPassword)
}

func RegisterProtected(r *gin.RouterGroup, h *handler.AuthHandler) {
	r.POST("/logout", h.Logout)
	r.POST("/refresh", h.RefreshToken)
	r.POST("/change_password", h.ChangePassword)
}
