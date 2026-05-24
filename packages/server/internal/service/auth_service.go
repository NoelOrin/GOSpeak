package service

import (
	"errors"
	"go_rtc/internal/model"
	"go_rtc/internal/pkg"
	"go_rtc/internal/repository"
	"gorm.io/gorm"
)

type AuthService struct {
	userRepo *repository.UserRepository
}

func NewAuthService(userRepo *repository.UserRepository) *AuthService {
	return &AuthService{userRepo: userRepo}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	User         model.User `json:"user"`
}

func (s *AuthService) Login(req *LoginRequest) (*AuthResponse, error) {
	user, err := s.userRepo.GetByName(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	if user.Password != req.Password {
		return nil, errors.New("invalid password")
	}

	token, err := pkg.GenerateToken(user.Name)
	if err != nil {
		return nil, err
	}

	refreshToken, err := pkg.GenerateRefreshToken(user.Name)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         *user,
	}, nil
}

func (s *AuthService) Register(req *RegisterRequest) (*AuthResponse, error) {
	existing, _ := s.userRepo.GetByName(req.Username)
	if existing != nil {
		return nil, errors.New("username already exists")
	}

	user := &model.User{
		Name:     req.Username,
		Password: req.Password,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	token, err := pkg.GenerateToken(user.Name)
	if err != nil {
		return nil, err
	}

	refreshToken, err := pkg.GenerateRefreshToken(user.Name)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         *user,
	}, nil
}

func (s *AuthService) RefreshToken(username string) (string, error) {
	return pkg.GenerateToken(username)
}