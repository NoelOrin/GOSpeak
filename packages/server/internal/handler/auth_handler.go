package handler

import (
	"go_rtc/internal/pkg"
	"go_rtc/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Login
// @Summary      用户登录
// @Description  验证用户并返回 JWT token
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body      service.LoginRequest  true  "登录凭据"
// @Success      200      {object}  pkg.Response
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	resp, err := h.authService.Login(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, resp)
}

// Register
// @Summary      用户注册
// @Description  注册新用户账号
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body      service.RegisterRequest  true  "注册信息"
// @Success      200      {object}  pkg.Response
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	resp, err := h.authService.Register(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, resp)
}

// GetRefreshToken
// @Summary      刷新 JWT token
// @Description  使用刷新 token 获取新的访问 token
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body      object{refresh_token=string}  true  "刷新 token"
// @Success      200      {object}  pkg.Response
// @Router       /auth/refresh_token [post]
func (h *AuthHandler) GetRefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	claims, err := pkg.ParseToken(req.RefreshToken)
	if err != nil {
		pkg.Fail(c, pkg.TOKEN_WRONG, "invalid refresh token")
		return
	}

	newToken, err := h.authService.RefreshToken(claims.Username)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{"access_token": newToken})
}

// Logout
// @Summary      用户登出
// @Description  使当前会话失效（需要 Bearer token）
// @Tags         认证
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	pkg.Success(c, nil)
}

// RefreshToken
// @Summary      刷新当前用户 token
// @Description  为已认证用户刷新 JWT token
// @Tags         认证
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	username, _ := c.Get("username")
	usernameStr, ok := username.(string)
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	token, err := h.authService.RefreshToken(usernameStr)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{"access_token": token})
}
