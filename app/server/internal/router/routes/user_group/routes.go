package user_group

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.UserGroupHandler) {
	r.POST("/list", h.List)
	r.POST("/create", h.Create)
	r.POST("/update", h.Update)
	r.POST("/delete", h.Delete)
}
