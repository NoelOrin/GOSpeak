// Package service 业务逻辑层，协调 repository 和外部服务完成核心业务。
package service

import (
	"errors"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/redis"
	"GOSpeak/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService 认证服务，处理登录、注册和 Token 刷新。
type AuthService struct {
	userRepo                 *repository.UserRepository
	emailConfigService       *EmailConfigService
	emailVerificationService *EmailVerificationService
}

func NewAuthService(userRepo *repository.UserRepository, emailConfigService *EmailConfigService, emailVerificationService *EmailVerificationService) *AuthService {
	return &AuthService{userRepo: userRepo, emailConfigService: emailConfigService, emailVerificationService: emailVerificationService}
}

// LoginRequest 用户名密码登录请求体。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest 新用户注册请求体。
type RegisterRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	Email     string `json:"email"`
	EmailCode string `json:"email_code"`
}

// AuthResponse 认证成功后的统一返回结构，包含双 Token 和用户信息。
type AuthResponse struct {
	Token              string     `json:"access_token"`
	RefreshToken       string     `json:"refresh_token"`
	User               model.User `json:"user"`
	NeedChangePassword bool       `json:"need_change_password"`
}

// Login 用户名密码登录：查用户 → bcrypt 比对密码 → 生成 Token 对。
func (s *AuthService) Login(req *LoginRequest) (*AuthResponse, error) {
	user, err := s.userRepo.GetByName(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND, "user not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PASSWORD)
	}

	if model.HasBanRole(user.Role) {
		return nil, pkg.NewAppError(pkg.USER_BANNED)
	}

	token, refreshToken, err := generateTokenPair(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion)
	if err != nil {
		return nil, err
	}

	needChange := user.Role == "admin" && bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("admin123")) == nil

	return &AuthResponse{
		Token:              token,
		RefreshToken:       refreshToken,
		User:               *user,
		NeedChangePassword: needChange,
	}, nil
}

// Register 新用户注册：查重名 → bcrypt 哈希密码 → 入库 → 生成 Token 对。
func (s *AuthService) Register(req *RegisterRequest) (*AuthResponse, error) {
	existing, _ := s.userRepo.GetByName(req.Username)
	if existing != nil {
		return nil, pkg.NewAppError(pkg.USERNAME_EXISTS)
	}

	if s.emailConfigService != nil && s.emailConfigService.IsVerificationAvailable() {
		if req.Email == "" || req.EmailCode == "" {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "email and email_code are required")
		}
		existingByEmail, err := s.userRepo.GetByEmail(req.Email)
		if err == nil && existingByEmail != nil {
			return nil, pkg.NewAppError(pkg.EMAIL_ALREADY_EXISTS)
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if s.emailVerificationService == nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "email verification service is not initialized")
		}
		if err := s.emailVerificationService.VerifyRegisterEmail(req.Email, req.EmailCode); err != nil {
			return nil, err
		}
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	user := &model.User{
		Name:          req.Username,
		Password:      string(hashedPwd),
		Role:          "user",
		Email:         req.Email,
		EmailVerified: req.Email != "" && s.emailConfigService != nil && s.emailConfigService.IsVerificationAvailable(),
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	token, refreshToken, err := generateTokenPair(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         *user,
	}, nil
}

// RefreshToken 用已有身份信息生成新 access_token（不验证旧 token 有效性，由 handler 层控制）。
func (s *AuthService) RefreshToken(username, displayName, userUUID, role string, tokenVersion uint) (string, error) {
	token, err := pkg.GenerateToken(username, displayName, userUUID, role, tokenVersion)
	if err != nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return token, nil
}

// ChangePassword 修改密码（需验证旧密码）。改密后递增 TokenVersion 使旧 token 失效。
func (s *AuthService) ChangePassword(username, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByName(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return pkg.NewAppError(pkg.INVALID_PASSWORD)
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	user.Password = string(hashedPwd)
	if err := s.userRepo.UpdatePasswordAndInvalidate(user); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// FirstChangePassword 首次登录修改密码（仅验证用户仍为默认密码，无需旧密码）。
// name 可选，传入时同时修改用户名。改密后递增版本并换发新 token 对。
func (s *AuthService) FirstChangePassword(username, newPassword string, name *string) (*AuthResponse, error) {
	user, err := s.userRepo.GetByName(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if user.Role != "admin" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "仅管理员支持首次登录改密")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("admin123")); err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "当前密码非默认密码，请使用普通改密接口")
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	user.Password = string(hashedPwd)

	if name != nil && *name != "" && *name != user.Name {
		existing, _ := s.userRepo.GetByName(*name)
		if existing != nil {
			return nil, pkg.NewAppError(pkg.USERNAME_EXISTS)
		}
		user.Name = *name
	}

	if err := s.userRepo.UpdatePasswordAndInvalidate(user); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// 重新读取用户获取递增后的 TokenVersion，换发新 token 对
	updatedUser, err := s.userRepo.GetByID(user.ID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	token, refreshToken, err := generateTokenPair(updatedUser.Name, updatedUser.DisplayName, updatedUser.UUID, updatedUser.Role, updatedUser.TokenVersion)
	if err != nil {
		return nil, err
	}
	return &AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         *updatedUser,
	}, nil
}

// ResetPassword 重置密码（通过邮箱验证码）。改密后递增 TokenVersion 使旧 token 失效。
func (s *AuthService) ResetPassword(email, code, newPassword string) error {
	if s.emailConfigService == nil || !s.emailConfigService.IsVerificationAvailable() {
		return pkg.NewAppError(pkg.PASSWORD_RESET_DISABLED)
	}
	if s.emailVerificationService == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "email verification service is not initialized")
	}
	if err := s.emailVerificationService.VerifyResetPasswordCode(email, code); err != nil {
		return err
	}

	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	user.Password = string(hashedPwd)
	if err := s.userRepo.UpdatePasswordAndInvalidate(user); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// BlacklistToken 将 JWT token 加入黑名单，使该 token 在有效期内不可再使用。
func (s *AuthService) BlacklistToken(claims *pkg.Claims) error {
	if claims == nil || claims.ID == "" {
		return nil
	}
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining <= 0 {
		return nil
	}
	return redis.BlacklistToken(claims.ID, remaining)
}

// GetTokenVersionByUUID 查询用户的当前 TokenVersion，供中间件校验 token 版本。
func (s *AuthService) GetTokenVersionByUUID(userUUID string) (uint, error) {
	user, err := s.userRepo.GetByUUID(userUUID)
	if err != nil {
		return 0, err
	}
	return user.TokenVersion, nil
}

// generateTokenPair 顺序生成 access_token 和 refresh_token。
// 注意：不能并发调用 GetSigningKey()，否则两个 goroutine 可能拿到不同密钥。
func generateTokenPair(username, displayName, userUUID, role string, tokenVersion uint) (string, string, error) {
	token, err := pkg.GenerateToken(username, displayName, userUUID, role, tokenVersion)
	if err != nil {
		return "", "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	refreshToken, err := pkg.GenerateRefreshToken(username, displayName, userUUID, role, tokenVersion)
	if err != nil {
		return "", "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return token, refreshToken, nil
}
// GenerateTokenPair 从 model.User 生成 token 对，供 OAuth 等模块复用。
func GenerateTokenPair(user *model.User) (string, string, error) {
	return generateTokenPair(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion)
}
