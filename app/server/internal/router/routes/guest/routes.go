package guest

import (
	"time"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Register 注册访客公开接口（挂载在 /auth 组下），带签发限流。
func Register(r *gin.RouterGroup, h *handler.GuestHandler) {
	r.POST("/guest", middleware.RateLimit(10, time.Hour), h.Join)
}
