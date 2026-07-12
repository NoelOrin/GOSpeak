// Package service — OAuth 第三方登录业务逻辑。
// 处理 OAuth 登录流程的完整闭环：获取授权 URL → 回调 → 创建/绑定用户 + Token 发放。
package service

import (
	"errors"
	"fmt"
	"sync"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/pkg/oauth"
	"GOSpeak/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateOAuthProviderRequest is the handler-facing DTO for creating an OAuth provider.
type CreateOAuthProviderRequest struct {
	Name          string `json:"name" binding:"required"`
	DisplayName   string `json:"display_name"`
	IconURL       string `json:"icon_url"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	AuthURL       string `json:"auth_url"`
	TokenURL      string `json:"token_url"`
	UserInfoURL   string `json:"user_info_url"`
	RedirectURL   string `json:"redirect_url"`
	Scopes        string `json:"scopes"`
	UIDField      string `json:"uid_field"`
	UsernameField string `json:"username_field"`
	AvatarField   string `json:"avatar_field"`
	EmailField    string `json:"email_field"`
	Enabled       *bool  `json:"enabled"`
}

// UpdateOAuthProviderRequest is the handler-facing DTO for updating an OAuth provider.
type UpdateOAuthProviderRequest struct {
	ID            uint   `json:"id" binding:"required"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	IconURL       string `json:"icon_url"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	AuthURL       string `json:"auth_url"`
	TokenURL      string `json:"token_url"`
	UserInfoURL   string `json:"user_info_url"`
	RedirectURL   string `json:"redirect_url"`
	Scopes        string `json:"scopes"`
	UIDField      string `json:"uid_field"`
	UsernameField string `json:"username_field"`
	AvatarField   string `json:"avatar_field"`
	EmailField    string `json:"email_field"`
	Enabled       *bool  `json:"enabled"`
}

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

// providerConfigFromModel 将 DB 模型转为 oauth.ProviderConfig（含 FieldMapping）。
func providerConfigFromModel(p *model.OAuthProvider) *oauth.ProviderConfig {
	return &oauth.ProviderConfig{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		AuthURL:      p.AuthURL,
		TokenURL:     p.TokenURL,
		UserInfoURL:  p.UserInfoURL,
		RedirectURL:  p.RedirectURL,
		Scopes:       p.Scopes,
		FieldMapping: oauth.FieldMapping{
			UIDField:      p.UIDField,
			UsernameField: p.UsernameField,
			AvatarField:   p.AvatarField,
			EmailField:    p.EmailField,
		},
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

	p := oauth.NewProvider(providerName, providerConfigFromModel(provider))
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

	p := oauth.NewProvider(providerName, providerConfigFromModel(provider))
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

	// 并行查询：OAuth 绑定账户 + 用户名可用性
	username := oauth.FormatUsername(userInfo.Username)
	var oauthAccount *model.OAuthAccount
	var existingUser *model.User
	var accountErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		oauthAccount, accountErr = s.accountRepo.GetByProviderAndUID(providerName, userInfo.ProviderUID)
	}()
	go func() {
		defer wg.Done()
		existingUser, _ = s.userRepo.GetByName(username)
	}()
	wg.Wait()

	if accountErr == nil {
		user, err := s.userRepo.GetByID(oauthAccount.UserID)
		if err != nil {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		oauthAccount.AccessToken = accessToken
		s.accountRepo.Update(oauthAccount)
		return s.buildAuthResponse(user)
	}

	if !errors.Is(accountErr, gorm.ErrRecordNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, accountErr.Error())
	}

	if existingUser != nil {
		username = fmt.Sprintf("%s_%s", username, uuid.New().String()[:8])
	}

	user := &model.User{
		Name:     username,
		Password: "",
	}
	oauthAccount = &model.OAuthAccount{
		Provider:    providerName,
		ProviderUID: userInfo.ProviderUID,
		AccessToken: accessToken,
	}
	if err := s.accountRepo.CreateWithUser(user, oauthAccount); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	return s.buildAuthResponse(user)
}

// buildAuthResponse 统一构造 OAuth 登录成功后的 AuthResponse（与密码登录复用同一结构）。
func (s *OAuthService) buildAuthResponse(user *model.User) (*AuthResponse, error) {
token, refreshToken, err := GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}
	return &AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         *user,
	}, nil
}

// EnabledProviderInfo 是不包含敏感信息的公开 OAuth 提供商信息，供登录页展示。
type EnabledProviderInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	IconURL     string `json:"icon_url"`
}

// ListEnabledProviders 返回已启用的 OAuth 提供商公开信息（不含 secret）。
func (s *OAuthService) ListEnabledProviders() ([]EnabledProviderInfo, error) {
	providers, err := s.providerRepo.List()
	if err != nil {
		return nil, err
	}
	var result []EnabledProviderInfo
	for _, p := range providers {
		if p.Enabled {
			displayName := p.DisplayName
			if displayName == "" {
				displayName = p.Name
			}
			result = append(result, EnabledProviderInfo{
				Name:        p.Name,
				DisplayName: displayName,
				IconURL:     p.IconURL,
			})
		}
	}
	return result, nil
}
// ListProviders 获取所有 OAuth 提供商配置（用于管理后台展示）。
func (s *OAuthService) ListProviders() ([]model.OAuthProvider, error) {
	return s.providerRepo.List()
}

// CreateProviderFromDTO creates an OAuth provider from a handler-level DTO.
func (s *OAuthService) CreateProviderFromDTO(req *CreateOAuthProviderRequest) (*model.OAuthProvider, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	provider := &model.OAuthProvider{
		Name:          req.Name,
		DisplayName:   req.DisplayName,
		IconURL:       req.IconURL,
		ClientID:      req.ClientID,
		ClientSecret:  req.ClientSecret,
		AuthURL:       req.AuthURL,
		TokenURL:      req.TokenURL,
		UserInfoURL:   req.UserInfoURL,
		RedirectURL:   req.RedirectURL,
		Scopes:        req.Scopes,
		UIDField:      req.UIDField,
		UsernameField: req.UsernameField,
		AvatarField:   req.AvatarField,
		EmailField:    req.EmailField,
		Enabled:       enabled,
	}

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

	if err := s.providerRepo.Create(provider); err != nil {
		return nil, err
	}
	return provider, nil
}

// UpdateProviderFromDTO updates an OAuth provider from a handler-level DTO.
func (s *OAuthService) UpdateProviderFromDTO(req *UpdateOAuthProviderRequest) (*model.OAuthProvider, error) {
	provider := &model.OAuthProvider{
		ID:            req.ID,
		Name:          req.Name,
		DisplayName:   req.DisplayName,
		IconURL:       req.IconURL,
		ClientID:      req.ClientID,
		ClientSecret:  req.ClientSecret,
		AuthURL:       req.AuthURL,
		TokenURL:      req.TokenURL,
		UserInfoURL:   req.UserInfoURL,
		RedirectURL:   req.RedirectURL,
		Scopes:        req.Scopes,
		UIDField:      req.UIDField,
		UsernameField: req.UsernameField,
		AvatarField:   req.AvatarField,
		EmailField:    req.EmailField,
	}
	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}

	if err := s.providerRepo.Update(provider); err != nil {
		return nil, err
	}
	return provider, nil
}

// DeleteProvider 删除 OAuth 提供商配置。
func (s *OAuthService) DeleteProvider(id uint) error {
	return s.providerRepo.Delete(id)
}
