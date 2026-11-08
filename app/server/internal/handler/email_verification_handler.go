package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type EmailVerificationHandler struct {
	svc *service.EmailVerificationService
}

func NewEmailVerificationHandler(svc *service.EmailVerificationService) *EmailVerificationHandler {
	return &EmailVerificationHandler{svc: svc}
}

type SendEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Scene string `json:"scene" binding:"required,oneof=register reset_password bind_email change_email"`
}

type VerifyEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Scene string `json:"scene" binding:"required,oneof=register reset_password bind_email change_email"`
	Code  string `json:"code" binding:"required,len=6"`
}

func (h *EmailVerificationHandler) SendCode(c *gin.Context) {
	var req SendEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	expiresIn, err := h.svc.SendCode(req.Email, req.Scene, c.ClientIP(), nil)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"expires_in": expiresIn})
}

func (h *EmailVerificationHandler) VerifyCode(c *gin.Context) {
	var req VerifyEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.VerifyCode(req.Email, req.Scene, req.Code); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"verified": true})
}
