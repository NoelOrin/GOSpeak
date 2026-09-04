package mute

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.MuteHandler) {
	r.POST("/create", middleware.RequirePermission(permcode.PermMuteManage), h.CreateMute)
	r.POST("/cancel", middleware.RequirePermission(permcode.PermMuteManage), h.CancelMute)
	r.POST("/status", middleware.RequirePermission(permcode.PermMuteManage), h.GetMuteStatus)
	r.POST("/list", middleware.RequirePermission(permcode.PermMuteManage), h.ListMutes)
}
