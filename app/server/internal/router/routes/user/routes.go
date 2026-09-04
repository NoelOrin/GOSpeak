package user

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.UserHandler) {
	r.POST("/profile", h.GetProfile)
	r.POST("/info", h.GetByName)
	r.GET("/preset-avatars", h.PresetAvatars)
	r.POST("/preset-avatars", h.PresetAvatars)
	r.POST("/update-profile", h.UpdateProfile)
	r.POST("/upload-avatar", h.UploadAvatar)
	r.POST("/list", middleware.RequirePermission(permcode.PermUserRead), h.List)
	r.POST("/get", middleware.RequirePermission(permcode.PermUserRead), h.GetByID)
	r.POST("/delete", middleware.RequirePermission(permcode.PermUserDelete), h.Delete)
	r.POST("/update-role", middleware.RequirePermission(permcode.PermUserUpdate), h.UpdateRole)
}
