package oauth

import (
	"go_rtc/internal/handler"
	"go_rtc/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.OAuthHandler) {
	r.GET("/login/:provider", h.Login)
	r.GET("/callback/:provider", h.Callback)
}

func RegisterAdmin(r *gin.RouterGroup, h *handler.OAuthHandler) {
	r.GET("/providers", middleware.RequireRole("admin"), h.ListProviders)
	r.POST("/providers", middleware.RequireRole("admin"), h.CreateProvider)
	r.PUT("/providers", middleware.RequireRole("admin"), h.UpdateProvider)
	r.DELETE("/providers/:id", middleware.RequireRole("admin"), h.DeleteProvider)
}
