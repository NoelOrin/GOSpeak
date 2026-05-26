// Package service — OAuth 第三方登录业务逻辑。
// 处理 OAuth 登录流程的完整闭环：获取授权 URL → 回调 → 创建/绑定用户 + Token 发放。
package service

import (
	"errors"
	"fmt"

	"go_rtc/internal/model"
	"go_rtc/internal/pkg"
	"go_rtc/internal/pkg/oauth"
	"go_rtc/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OAuthService 第三方登录服务，协调三个 repository 完成 OAuth 流程。
type OAuthService struct {
	providerRepo *repository.OAuthProviderRepository
	accountRepo  *repository.OAuthAccountRepository
	userRepo     *repository.UserRepository
}

func NewOAuthService(
	providerRepo *repository.OAuthProviderRepository,
	accountRepo *repository.OAuthAccountRepository,
	userRepo *repository.UserRepository,
) *OAuthService {
	return &OAuthService{
		providerRepo: providerRepo,
		accountRepo:  accountRepo,
		userRepo:     userRepo,
	}
}

// GetAuthURL 构造 OAuth 授权页面 URL，供前端跳转。会检查提供商是否启用。
func (s *OAuthService) GetAuthURL(providerName, state string) (string, error) {
	provider, err := s.providerRepo.GetByName(providerName)
	if err != nil {
		return "", pkg.NewAppError(pkg.OAUTH_PROVIDER_NOT_FOUND)
	}
	if !provider.Enabled {
		return "", pkg.NewAppError(pkg.OAUTH_PROVIDER_DISABLED)
	}

	p := oauth.NewProvider(providerName, &oauth.ProviderConfig{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		AuthURL:      provider.AuthURL,
		TokenURL:     provider.TokenURL,
		UserInfoURL:  provider.UserInfoURL,
		RedirectURL:  provider.RedirectURL,
		Scopes:       provider.Scopes,
	})
	if p == nil {
		return "", pkg.NewAppError(pkg.OAUTH_PROVIDER_NOT_FOUND)
	}

	return p.GetAuthURL(state), nil
}

// HandleCallback 处理 OAuth 授权回调：兑换 access_token → 获取用户信息 →
// 检查是否已有绑定账号（有则直接登录）→ 无则创建新用户并绑定 OAuth 账号 → 发放 Token 对。
func (s *OAuthService) HandleCallback(providerName, code string) (*AuthResponse, error) {
	provider, err := s.providerRepo.GetByName(providerName)
	if err != nil {
		return nil, pkg.NewAppError(pkg.OAUTH_PROVIDER_NOT_FOUND)
	}
	if !provider.Enabled {
		return nil, pkg.NewAppError(pkg.OAUTH_PROVIDER_DISABLED)
	}

	p := oauth.NewProvider(providerName, &oauth.ProviderConfig{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		AuthURL:      provider.AuthURL,
		TokenURL:     provider.TokenURL,
		UserInfoURL:  provider.UserInfoURL,
		RedirectURL:  provider.RedirectURL,
		Scopes:       provider.Scopes,
	})
	if p == nil {
		return nil, pkg.NewAppError(pkg.OAUTH_PROVIDER_NOT_FOUND)
	}

	accessToken, err := p.ExchangeToken(code)
	if err != nil {
		return nil, pkg.NewAppError(pkg.OAUTH_TOKEN_EXCHANGE_FAILED, err.Error())
	}

	userInfo, err := p.GetUserInfo(accessToken)
	if err != nil {
		return nil, pkg.NewAppError(pkg.OAUTH_GET_USER_FAILED, err.Error())
	}

	oauthAccount, err := s.accountRepo.GetByProviderAndUID(providerName, userInfo.ProviderUID)
	if err == nil {
		user, err := s.userRepo.GetByID(oauthAccount.UserID)
		if err != nil {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		oauthAccount.AccessToken = accessToken
		s.accountRepo.Update(oauthAccount)
		return s.buildAuthResponse(user)
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	username := oauth.FormatUsername(userInfo.Username)
	existingUser, _ := s.userRepo.GetByName(username)
	if existingUser != nil {
		username = fmt.Sprintf("%s_%s", username, uuid.New().String()[:8])
	}

	user := &model.User{
		Name:     username,
		Password: "",
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	oauthAccount = &model.OAuthAccount{
		UserID:      user.ID,
		Provider:    providerName,
		ProviderUID: userInfo.ProviderUID,
		AccessToken: accessToken,
	}
	if err := s.accountRepo.Create(oauthAccount); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	return s.buildAuthResponse(user)
}

// buildAuthResponse 统一构造 OAuth 登录成功后的 AuthResponse（与密码登录复用同一结构）。
func (s *OAuthService) buildAuthResponse(user *model.User) (*AuthResponse, error) {
	token, err := pkg.GenerateToken(user.Name, user.UUID, user.Role)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	refreshToken, err := pkg.GenerateRefreshToken(user.Name, user.UUID, user.Role)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return &AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         *user,
	}, nil
}

// ListProviders 获取所有 OAuth 提供商配置（用于管理后台展示）。
func (s *OAuthService) ListProviders() ([]model.OAuthProvider, error) {
	return s.providerRepo.List()
}

// CreateProvider 创建 OAuth 提供商配置，未填写的 URL 和 Scope 自动用默认值补齐。
func (s *OAuthService) CreateProvider(provider *model.OAuthProvider) error {
	defaultCfg := oauth.GetDefaultConfig(provider.Name)
	if defaultCfg != nil {
		if provider.AuthURL == "" {
			provider.AuthURL = defaultCfg.AuthURL
		}
		if provider.TokenURL == "" {
			provider.TokenURL = defaultCfg.TokenURL
		}
		if provider.UserInfoURL == "" {
			provider.UserInfoURL = defaultCfg.UserInfoURL
		}
		if provider.Scopes == "" {
			provider.Scopes = defaultCfg.Scopes
		}
	}
	return s.providerRepo.Create(provider)
}

// UpdateProvider 更新 OAuth 提供商配置。
func (s *OAuthService) UpdateProvider(provider *model.OAuthProvider) error {
	return s.providerRepo.Update(provider)
}

// DeleteProvider 删除 OAuth 提供商配置。
func (s *OAuthService) DeleteProvider(id uint) error {
	return s.providerRepo.Delete(id)
}
