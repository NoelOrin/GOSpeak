package user

import (
	"go_rtc/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.UserHandler) {
	r.GET("/profile", h.GetProfile)
	r.GET("/list", h.List)
	r.GET("/:id", h.GetByID)
	r.DELETE("/:id", h.Delete)
}
