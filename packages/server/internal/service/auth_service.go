// Package service 业务逻辑层，协调 repository 和外部服务完成核心业务。
package service

import (
	"errors"
	"sync"

	"go_rtc/internal/model"
	"go_rtc/internal/pkg"
	"go_rtc/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService 认证服务，处理登录、注册和 Token 刷新。
type AuthService struct {
	userRepo *repository.UserRepository
}

func NewAuthService(userRepo *repository.UserRepository) *AuthService {
	return &AuthService{userRepo: userRepo}
}

// LoginRequest 用户名密码登录请求体。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest 新用户注册请求体。
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse 认证成功后的统一返回结构，包含双 Token 和用户信息。
type AuthResponse struct {
	Token             string     `json:"access_token"`
	RefreshToken      string     `json:"refresh_token"`
	User              model.User `json:"user"`
	NeedChangePassword bool      `json:"need_change_password"`
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

	token, refreshToken, err := generateTokenPair(user.Name, user.DisplayName, user.UUID, user.Role)
	if err != nil {
		return nil, err
	}

	needChange := user.Role == "admin" && bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("admin123")) == nil

	return &AuthResponse{
		Token:             token,
		RefreshToken:      refreshToken,
		User:              *user,
		NeedChangePassword: needChange,
	}, nil
}

// Register 新用户注册：查重名 → bcrypt 哈希密码 → 入库 → 生成 Token 对。
func (s *AuthService) Register(req *RegisterRequest) (*AuthResponse, error) {
	existing, _ := s.userRepo.GetByName(req.Username)
	if existing != nil {
		return nil, pkg.NewAppError(pkg.USERNAME_EXISTS)
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	user := &model.User{
		Name:     req.Username,
		Password: string(hashedPwd),
		Role:     "user",
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	token, refreshToken, err := generateTokenPair(user.Name, user.DisplayName, user.UUID, user.Role)
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
func (s *AuthService) RefreshToken(username, displayName, userUUID, role string) (string, error) {
	token, err := pkg.GenerateToken(username, displayName, userUUID, role)
	if err != nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return token, nil
}

// ChangePassword 修改密码（需验证旧密码）。
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
	if err := s.userRepo.Update(user); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// FirstChangePassword 首次登录修改密码（仅验证用户仍为默认密码，无需旧密码）。
// name 可选，传入时同时修改用户名。
func (s *AuthService) FirstChangePassword(username, newPassword string, name *string) (*model.User, error) {
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

	if err := s.userRepo.Update(user); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return user, nil
}

// ResetPassword 重置密码（仅需用户名，用于忘记密码场景）。
func (s *AuthService) ResetPassword(username, newPassword string) error {
	user, err := s.userRepo.GetByName(username)
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
	if err := s.userRepo.Update(user); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// generateTokenPair 并行生成 access_token 和 refresh_token。
func generateTokenPair(username, displayName, userUUID, role string) (string, string, error) {
	var token, refreshToken string
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		token, err1 = pkg.GenerateToken(username, displayName, userUUID, role)
	}()
	go func() {
		defer wg.Done()
		refreshToken, err2 = pkg.GenerateRefreshToken(username, displayName, userUUID, role)
	}()
	wg.Wait()
	if err1 != nil {
		return "", "", pkg.NewAppError(pkg.INTERNAL_ERROR, err1.Error())
	}
	if err2 != nil {
		return "", "", pkg.NewAppError(pkg.INTERNAL_ERROR, err2.Error())
	}
	return token, refreshToken, nil
}
