package storage

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.StorageHandler) {
	r.POST("/presign", h.PresignUpload)
	r.POST("/confirm", h.ConfirmUpload)
	r.POST("/upload", h.Upload)

	// 管理员路由
	admin := r.Group("", middleware.RequireRole("admin"))
	admin.POST("/delete", h.DeleteObject)
	admin.POST("/config", h.GetConfig)
	admin.POST("/update-config", h.UpdateConfig)
}
