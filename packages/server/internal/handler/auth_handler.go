package handler

import (
	"go_rtc/internal/model"
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

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"john"`
	Password string `json:"password" binding:"required" example:"123456"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required" example:"john"`
	Password string `json:"password" binding:"required" example:"123456"`
}

// RefreshTokenRequest 刷新 Token 请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required" example:"eyJ..."`
}

// AuthResponse 认证响应
type AuthResponse struct {
	Code    pkg.ErrCode        `json:"code" example:"0"`
	Message string             `json:"message" example:"success"`
	Data    AuthResponseData   `json:"data,omitempty"`
}

type AuthResponseData struct {
	Token        string      `json:"token" example:"eyJhbGciOiJIUzI1NiIs..."`
	RefreshToken string      `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIs..."`
	User         model.User  `json:"user"`
}

// Login
// @Summary      User login
// @Description  Authenticate user and return JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      LoginRequest  true  "Login credentials"
// @Success      200      {object}  AuthResponse
// @Failure      400      {object}  pkg.Response
// @Router       /auth/login [post]
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

// Register
// @Summary      User registration
// @Description  Register a new user account
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      RegisterRequest  true  "Registration info"
// @Success      200      {object}  AuthResponse
// @Failure      400      {object}  pkg.Response
// @Router       /auth/register [post]
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

// RefreshToken
// @Summary      Refresh JWT token
// @Description  Get a new access token using refresh token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      RefreshTokenRequest  true  "Refresh token"
// @Success      200      {object}  pkg.Response
// @Failure      401      {object}  pkg.Response
// @Router       /auth/refresh_token [post]
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

// Logout
// @Summary      User logout
// @Description  Invalidate current session (requires Bearer token)
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	pkg.Success(c, nil)
}

// RefreshTokenInternal
// @Summary      Refresh current user token
// @Description  Refresh the JWT token for the authenticated user
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /auth/refresh [post]
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