package announcement

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.AnnouncementHandler) {
	r.POST("/list", h.List)
	r.POST("/unread-count", h.UnreadCount)
	r.POST("/mark-read", h.MarkRead)

	admin := r.Group("/admin")
	admin.POST("/list", h.AdminList)
	admin.POST("/create", h.AdminCreate)
	admin.POST("/update", h.AdminUpdate)
	admin.POST("/delete", h.AdminDelete)
}
