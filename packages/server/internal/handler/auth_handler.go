package handler

import (
	"go_rtc/internal/pkg"
	"go_rtc/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.FailWithMsg(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	resp, err := h.authService.Login(&req)
	if err != nil {
		pkg.FailWithMsg(c, pkg.UNAUTHORIZED, err.Error())
		return
	}

	pkg.Success(c, resp)
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.FailWithMsg(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	resp, err := h.authService.Register(&req)
	if err != nil {
		pkg.FailWithMsg(c, pkg.ERROR, err.Error())
		return
	}

	pkg.Success(c, resp)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	username, _ := c.Get("username")
	usernameStr, ok := username.(string)
	if !ok {
		pkg.Fail(c, pkg.UNAUTHORIZED)
		return
	}

	token, err := h.authService.RefreshToken(usernameStr)
	if err != nil {
		pkg.FailWithMsg(c, pkg.INTERNAL_ERROR, err.Error())
		return
	}

	pkg.Success(c, gin.H{"token": token})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	pkg.Success(c, nil)
}

func (h *AuthHandler) GetRefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.FailWithMsg(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	claims, err := pkg.ParseToken(req.RefreshToken)
	if err != nil {
		pkg.FailWithMsg(c, pkg.TOKEN_WRONG, "invalid refresh token")
		return
	}

	newToken, err := h.authService.RefreshToken(claims.Username)
	if err != nil {
		pkg.Error(c, http.StatusInternalServerError, pkg.INTERNAL_ERROR)
		return
	}

	pkg.Success(c, gin.H{"token": newToken})
}