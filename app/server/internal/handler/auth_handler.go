package handler

import (
	"strings"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

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

	if strings.HasPrefix(req.Username, "bot_") {
		pkg.Fail(c, pkg.USERNAME_EXISTS, "username prefix 'bot_' is reserved")
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

	resp, err := h.authService.RefreshFromToken(req.RefreshToken)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, resp)
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
	var accessClaims *pkg.Claims
	if v, ok := c.Get("claims"); ok {
		accessClaims, _ = v.(*pkg.Claims)
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req)

	if err := h.authService.Logout(accessClaims, req.RefreshToken); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

// RefreshToken
// @Summary      刷新当前用户 token
// @Description  为已认证用户刷新 JWT token
// @Tags         认证
// @Produce      json
// @Security     BearerAuth
// @Success      200      {object}  pkg.Response
// @Router       /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	userUUID, _ := c.Get("user_uuid")
	uuidStr, ok := userUUID.(string)
	if !ok || uuidStr == "" {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	// 已鉴权路径也从 DB 重载 role/version，避免 JWT 快照续签旧权限
	token, err := h.authService.RefreshAccessForUserUUID(uuidStr)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{"access_token": token})
}

// ChangePassword
// @Summary      修改密码
// @Description  已登录用户修改密码（需验证旧密码），改密后旧 token 失效
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
// @Description  管理员首次登录时修改默认密码（无需旧密码），可选同时修改用户名，改密后换发新 token
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

	resp, err := h.authService.FirstChangePassword(usernameStr, req.NewPassword, req.Name)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, resp)
}

// ResetPassword
// @Summary      重置密码
// @Description  通过邮箱验证码重置密码；未启用邮箱配置时该能力禁用
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body  object{email=string,code=string,new_password=string}  true  "邮箱验证码和新密码"
// @Success      200  {object}  pkg.Response
// @Router       /auth/reset_password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		Code        string `json:"code" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.authService.ResetPassword(req.Email, req.Code, req.NewPassword); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, nil)
}
