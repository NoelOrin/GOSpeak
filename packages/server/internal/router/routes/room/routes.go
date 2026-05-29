package room

import (
	"go_rtc/internal/handler"
	"go_rtc/internal/middleware"
	"go_rtc/internal/model"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.RoomHandler) {
	r.POST("/create", middleware.RequirePermission(model.PermRoomCreate), h.Create)
	r.GET("/list", middleware.RequirePermission(model.PermRoomRead), h.List)
	r.GET("/:id", middleware.RequirePermission(model.PermRoomRead), h.Get)
	r.PUT("/:id", middleware.RequirePermission(model.PermRoomUpdate), h.Update)
	r.DELETE("/:id", middleware.RequirePermission(model.PermRoomDelete), h.Delete)
}
