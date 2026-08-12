package room

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.RoomHandler) {
	r.POST("/create", middleware.RequireDomainMember(), h.Create)
	r.POST("/list", middleware.RequireDomainMemberIfProvided(), h.List)
	r.POST("/get", h.Get)
	r.POST("/update", h.Update)
	r.POST("/delete", h.Delete)
}
