package oauth

import (
	"go_rtc/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.OAuthHandler) {
	r.GET("/login/:provider", h.Login)
	r.GET("/callback/:provider", h.Callback)
}

func RegisterAdmin(r *gin.RouterGroup, h *handler.OAuthHandler) {
	r.GET("/providers", h.ListProviders)
	r.POST("/providers", h.CreateProvider)
	r.PUT("/providers", h.UpdateProvider)
	r.DELETE("/providers/:id", h.DeleteProvider)
}
