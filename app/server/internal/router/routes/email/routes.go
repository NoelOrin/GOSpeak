package email

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.EmailVerificationHandler) {
	r.POST("/send_code", h.SendCode)
	r.POST("/verify_code", h.VerifyCode)
}
