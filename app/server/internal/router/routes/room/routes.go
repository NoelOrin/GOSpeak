package room

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.RoomHandler) {
	r.POST("/create", middleware.RequirePermission(permcode.PermRoomCreate), middleware.RequireGuildMemberIfProvided(), h.Create)
	r.POST("/list", middleware.RequirePermission(permcode.PermRoomRead), middleware.RequireGuildMemberIfProvided(), h.List)
	r.POST("/get", middleware.RequirePermission(permcode.PermRoomRead), h.Get)
	r.POST("/update", middleware.RequirePermission(permcode.PermRoomUpdate), h.Update)
	r.POST("/delete", middleware.RequirePermission(permcode.PermRoomDelete), h.Delete)
}
