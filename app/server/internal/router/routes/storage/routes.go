package storage

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.StorageHandler) {
	r.POST("/presign", h.PresignUpload)
	r.POST("/confirm", h.ConfirmUpload)
	r.POST("/upload", h.Upload)

	r.POST("/delete", middleware.RequirePermission(permcode.PermStorageDelete), h.DeleteObject)
	r.POST("/config", middleware.RequirePermission(permcode.PermStorageRead), h.GetConfig)
	r.POST("/update-config", middleware.RequirePermission(permcode.PermStorageManage), h.UpdateConfig)
	r.POST("/test-config", middleware.RequirePermission(permcode.PermStorageRead), h.TestConfig)
}
