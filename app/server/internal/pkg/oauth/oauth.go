// Package oauth 提供 OAuth2 第三方登录协议实现（GitHub / Google / QQ）。
// 通过 Provider 接口抽象统一的认证流程：构造授权 URL → 兑换 access_token → 获取用户信息。
package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ProviderConfig 第三方 OAuth 应用的配置项，根据平台填写对应值。
type ProviderConfig struct {
	ClientID     string       // OAuth App 的 Client ID
	ClientSecret string       // OAuth App 的 Client Secret
	AuthURL      string       // 用户授权页面 URL
	TokenURL     string       // 用 code 兑换 access_token 的端点
	UserInfoURL  string       // 获取用户信息的 API 端点
	RedirectURL  string       // 授权后回调地址
	Scopes       string       // 请求的权限范围，多个用空格分隔
	FieldMapping FieldMapping // 自建 OAuth 的用户信息字段映射（仅 GenericProvider 使用）
}

// UserInfo 统一用户信息模型，屏蔽各平台字段差异。
type UserInfo struct {
	Provider    string // 平台名（github / google / qq）
	ProviderUID string // 平台侧用户唯一标识（github=id, google=sub, qq=openid）
	Username    string // 昵称
	Avatar      string // 头像 URL
	Email       string // 邮箱（部分平台可能为空）
}

// Provider 定义 OAuth2 认证流程的三步接口。
type Provider interface {
	// GetAuthURL 构造授权页面 URL，用户浏览器跳转到此 URL 完成授权。
	GetAuthURL(state string) string
	// ExchangeToken 用授权回调中的 code 兑换 access_token。
	ExchangeToken(code string) (string, error)
	// GetUserInfo 用 access_token 获取用户基本信息。
	GetUserInfo(accessToken string) (*UserInfo, error)
}

// NewProvider 根据平台名创建对应的 Provider 实例。不支持的平台返回 nil。
func NewProvider(name string, cfg *ProviderConfig) Provider {
	switch name {
	case "github":
		return &GitHubProvider{cfg: cfg}
	case "google":
		return &GoogleProvider{cfg: cfg}
	case "qq":
		return &QQProvider{cfg: cfg}
	default:
		// 自建/自定义 OAuth 提供商，使用通用实现
		if cfg.AuthURL != "" && cfg.TokenURL != "" && cfg.UserInfoURL != "" {
			return &GenericProvider{cfg: cfg, name: name, fields: cfg.FieldMapping}
		}
		return nil
	}
}

const (
	oauthHTTPTimeout = 10 * time.Second
	maxOAuthBody     = 2 << 20 // 2 MiB
)

var oauthHTTPClient = &http.Client{Timeout: oauthHTTPTimeout}

// safeOAuthURL 仅允许 http/https 且必须携带 host，防止 SSRF 类出站请求。
func safeOAuthURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("oauth: invalid url: %w", err)
	}
	switch u.Scheme {
	case "https", "http":
	default:
		return "", fmt.Errorf("oauth: unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("oauth: empty host")
	}
	return u.String(), nil
}

func httpDo(req *http.Request) ([]byte, error) {
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, maxOAuthBody+1))
}

// httpPostForm 通用 POST application/x-www-form-urlencoded 请求，限制响应体大小。
func httpPostForm(rawURL string, data url.Values) ([]byte, error) {
	safe, err := safeOAuthURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, safe, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := httpDo(req)
	if err != nil {
		return nil, err
	}
	if len(body) > maxOAuthBody {
		return nil, fmt.Errorf("oauth: response body too large")
	}
	return body, nil
}

// httpGet 通用 GET 请求，支持 Bearer Token 鉴权，限制响应体大小。
func httpGet(rawURL, token string) ([]byte, error) {
	safe, err := safeOAuthURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, safe, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	body, err := httpDo(req)
	if err != nil {
		return nil, err
	}
	if len(body) > maxOAuthBody {
		return nil, fmt.Errorf("oauth: response body too large")
	}
	return body, nil
}

// GitHubProvider GitHub OAuth2 登录实现。
// 响应格式：URL 编码键值对（token 端点），JSON（userinfo 端点）。
type GitHubProvider struct {
	cfg *ProviderConfig
}

func (g *GitHubProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", g.cfg.ClientID)
	params.Set("redirect_uri", g.cfg.RedirectURL)
	params.Set("scope", g.cfg.Scopes)
	params.Set("state", state)
	return g.cfg.AuthURL + "?" + params.Encode()
}

func (g *GitHubProvider) ExchangeToken(code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", g.cfg.ClientID)
	data.Set("client_secret", g.cfg.ClientSecret)
	data.Set("code", code)

	body, err := httpPostForm(g.cfg.TokenURL, data)
	if err != nil {
		return "", err
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return "", err
	}
	token := values.Get("access_token")
	if token == "" {
		return "", fmt.Errorf("github: no access_token in response")
	}
	return token, nil
}

func (g *GitHubProvider) GetUserInfo(accessToken string) (*UserInfo, error) {
	body, err := httpGet(g.cfg.UserInfoURL, accessToken)
	if err != nil {
		return nil, err
	}
	var data struct {
		ID     int    `json:"id"`
		Login  string `json:"login"`
		Email  string `json:"email"`
		Avatar string `json:"avatar_url"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &UserInfo{
		Provider:    "github",
		ProviderUID: fmt.Sprintf("%d", data.ID),
		Username:    data.Login,
		Avatar:      data.Avatar,
		Email:       data.Email,
	}, nil
}

// GoogleProvider Google OAuth2 登录实现。
// 响应格式：JSON（token 端点和 userinfo 端点）。
// 注意请求中额外携带 response_type=code 和 access_type=offline 参数。
type GoogleProvider struct {
	cfg *ProviderConfig
}

func (gp *GoogleProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", gp.cfg.ClientID)
	params.Set("redirect_uri", gp.cfg.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", gp.cfg.Scopes)
	params.Set("state", state)
	params.Set("access_type", "offline")
	return gp.cfg.AuthURL + "?" + params.Encode()
}

func (gp *GoogleProvider) ExchangeToken(code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", gp.cfg.ClientID)
	data.Set("client_secret", gp.cfg.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", gp.cfg.RedirectURL)

	body, err := httpPostForm(gp.cfg.TokenURL, data)
	if err != nil {
		return "", err
	}
	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("google: %s", result.Error)
	}
	return result.AccessToken, nil
}

func (gp *GoogleProvider) GetUserInfo(accessToken string) (*UserInfo, error) {
	body, err := httpGet(gp.cfg.UserInfoURL, accessToken)
	if err != nil {
		return nil, err
	}
	var data struct {
		Sub     string `json:"sub"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &UserInfo{
		Provider:    "google",
		ProviderUID: data.Sub,
		Username:    data.Name,
		Avatar:      data.Picture,
		Email:       data.Email,
	}, nil
}

// QQProvider QQ OAuth2 登录实现。
// QQ 的 token 端点同时支持 JSON 和 URL 编码响应（通过 fmt=json 参数控制）。
// userinfo 端点需将 access_token 以 query 参数形式传递。
type QQProvider struct {
	cfg *ProviderConfig
}

func (q *QQProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", q.cfg.ClientID)
	params.Set("redirect_uri", q.cfg.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", q.cfg.Scopes)
	params.Set("state", state)
	return q.cfg.AuthURL + "?" + params.Encode()
}

func (q *QQProvider) ExchangeToken(code string) (string, error) {
	params := url.Values{}
	params.Set("client_id", q.cfg.ClientID)
	params.Set("client_secret", q.cfg.ClientSecret)
	params.Set("code", code)
	params.Set("grant_type", "authorization_code")
	params.Set("redirect_uri", q.cfg.RedirectURL)
	params.Set("fmt", "json")

	body, err := httpPostForm(q.cfg.TokenURL, params)
	if err != nil {
		return "", err
	}
	var result struct {
		AccessToken string `json:"access_token"`
		Error       int    `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.Error != 0 {
		return "", fmt.Errorf("qq: %s", result.ErrorDesc)
	}
	return result.AccessToken, nil
}

func (q *QQProvider) GetUserInfo(accessToken string) (*UserInfo, error) {
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("fmt", "json")

	body, err := httpGet(q.cfg.UserInfoURL+"?"+params.Encode(), "")
	if err != nil {
		return nil, err
	}
	var data struct {
		Ret      int    `json:"ret"`
		OpenID   string `json:"openid"`
		Nickname string `json:"nickname"`
		Avatar   string `json:"figureurl_qq_2"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	if data.Ret != 0 {
		return nil, fmt.Errorf("qq: get user info failed, ret=%d", data.Ret)
	}
	return &UserInfo{
		Provider:    "qq",
		ProviderUID: data.OpenID,
		Username:    data.Nickname,
		Avatar:      data.Avatar,
	}, nil
}

// GetDefaultConfig 返回各平台的预设端点配置和默认 scope。
// ClientID / ClientSecret / RedirectURL 需业务方自行注入。
func GetDefaultConfig(name string) *ProviderConfig {
	switch name {
	case "github":
		return &ProviderConfig{
			AuthURL:     "https://github.com/login/oauth/authorize",
			TokenURL:    "https://github.com/login/oauth/access_token",
			UserInfoURL: "https://api.github.com/user",
			Scopes:      "read:user user:email",
		}
	case "google":
		return &ProviderConfig{
			AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:    "https://oauth2.googleapis.com/token",
			UserInfoURL: "https://www.googleapis.com/oauth2/v3/userinfo",
			Scopes:      "openid email profile",
		}
	case "qq":
		return &ProviderConfig{
			AuthURL:     "https://graph.qq.com/oauth2.0/authorize",
			TokenURL:    "https://graph.qq.com/oauth2.0/token",
			UserInfoURL: "https://graph.qq.com/user/get_user_info",
			Scopes:      "get_user_info",
		}
	default:
		return nil
	}
}

// FormatUsername 将第三方昵称转为统一小写+下划线格式的用户名，用于自动生成系统用户名。
func FormatUsername(base string) string {
	base = strings.ToLower(base)
	base = strings.ReplaceAll(base, " ", "_")
	return base
}
