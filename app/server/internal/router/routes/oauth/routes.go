package oauth

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.OAuthHandler) {
	r.GET("/login/:provider", h.Login)
	r.GET("/callback/:provider", h.Callback)
}

func RegisterAdmin(r *gin.RouterGroup, h *handler.OAuthHandler) {
	r.GET("/providers", middleware.RequirePermission(permcode.PermRoleManage), h.ListProviders)
	r.POST("/providers", middleware.RequirePermission(permcode.PermRoleManage), h.CreateProvider)
	r.PUT("/providers", middleware.RequirePermission(permcode.PermRoleManage), h.UpdateProvider)
	r.DELETE("/providers/:id", middleware.RequirePermission(permcode.PermRoleManage), h.DeleteProvider)
}
