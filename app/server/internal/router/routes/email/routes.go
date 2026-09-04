package email

import (
	"time"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.EmailVerificationHandler) {
	public := r.Group("")
	public.Use(middleware.RateLimit(5, time.Minute))
	public.POST("/send_code", h.SendCode)
	public.POST("/verify_code", h.VerifyCode)
}
