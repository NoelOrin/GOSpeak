package room

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.RoomHandler) {
	r.POST("/create", middleware.RequirePlatformAdminOrDomainMember(), h.Create)
	r.POST("/list", middleware.RequirePlatformAdminOrDomainMemberIfProvided(), h.List)
	r.POST("/get", h.Get)
	r.POST("/update", h.Update)
	r.POST("/delete", h.Delete)
}
