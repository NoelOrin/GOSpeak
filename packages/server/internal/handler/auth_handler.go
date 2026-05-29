package handler

import (
	"go_rtc/internal/pkg"
	"go_rtc/internal/redis"
	"go_rtc/internal/service"
	"time"

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

	newToken, err := h.authService.RefreshToken(claims.Username, claims.DisplayName, claims.UserUUID, claims.Role)
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
	claims, ok := c.Get("claims")
	if ok {
		if cl, ok := claims.(*pkg.Claims); ok && cl.ID != "" {
			remaining := time.Until(cl.ExpiresAt.Time)
			_ = redis.BlacklistToken(cl.ID, remaining)
		}
	}
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

	userUUID, _ := c.Get("user_uuid")
	uuidStr, ok := userUUID.(string)
	if !ok || uuidStr == "" {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	role, _ := c.Get("role")
	roleStr, ok := role.(string)
	if !ok {
		roleStr = "user"
	}

	displayName, _ := c.Get("display_name")
	displayNameStr, _ := displayName.(string)

	token, err := h.authService.RefreshToken(usernameStr, displayNameStr, uuidStr, roleStr)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{"access_token": token})
}

// ChangePassword
// @Summary      修改密码
// @Description  已登录用户修改密码（需验证旧密码）
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{old_password=string,new_password=string}  true  "密码信息"
// @Success      200  {object}  pkg.Response
// @Router       /auth/change_password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	username, _ := c.Get("username")
	usernameStr, ok := username.(string)
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	if err := h.authService.ChangePassword(usernameStr, req.OldPassword, req.NewPassword); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, nil)
}

// FirstChangePassword
// @Summary      首次登录修改密码
// @Description  管理员首次登录时修改默认密码（无需旧密码），可选同时修改用户名
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{new_password=string,name=string}  true  "新密码，name可选"
// @Success      200  {object}  pkg.Response
// @Router       /auth/first_change_password [post]
func (h *AuthHandler) FirstChangePassword(c *gin.Context) {
	var req struct {
		NewPassword string  `json:"new_password" binding:"required"`
		Name        *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	username, _ := c.Get("username")
	usernameStr, ok := username.(string)
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	user, err := h.authService.FirstChangePassword(usernameStr, req.NewPassword, req.Name)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, user)
}

// ResetPassword
// @Summary      重置密码
// @Description  通过用户名重置密码（忘记密码场景，无需旧密码）
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body  object{username=string,new_password=string}  true  "用户名和新密码"
// @Success      200  {object}  pkg.Response
// @Router       /auth/reset_password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.authService.ResetPassword(req.Username, req.NewPassword); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, nil)
}
