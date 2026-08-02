package room

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.RoomHandler) {
	r.POST("/create", middleware.RequirePermission(permcode.PermRoomCreate), middleware.RequireDomainMember(), h.Create)
	r.POST("/list", middleware.RequirePermission(permcode.PermRoomRead), middleware.RequireDomainMemberIfProvided(), h.List)
	r.POST("/get", middleware.RequirePermission(permcode.PermRoomRead), h.Get)
	r.POST("/update", middleware.RequirePermission(permcode.PermRoomUpdate), h.Update)
	r.POST("/delete", middleware.RequirePermission(permcode.PermRoomDelete), h.Delete)
}
