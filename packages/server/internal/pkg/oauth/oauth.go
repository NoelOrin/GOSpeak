package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type ProviderConfig struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	RedirectURL  string
	Scopes       string
}

type UserInfo struct {
	Provider    string
	ProviderUID string
	Username    string
	Avatar      string
	Email       string
}

type Provider interface {
	GetAuthURL(state string) string
	ExchangeToken(code string) (string, error)
	GetUserInfo(accessToken string) (*UserInfo, error)
}

func NewProvider(name string, cfg *ProviderConfig) Provider {
	switch name {
	case "github":
		return &GitHubProvider{cfg: cfg}
	case "google":
		return &GoogleProvider{cfg: cfg}
	case "qq":
		return &QQProvider{cfg: cfg}
	default:
		return nil
	}
}

func httpPostForm(url string, data url.Values) ([]byte, error) {
	resp, err := http.PostForm(url, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func httpGet(url, token string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

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
		ID    int    `json:"id"`
		Login string `json:"login"`
		Email string `json:"email"`
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
		AccessToken  string `json:"access_token"`
		Error        int    `json:"error"`
		ErrorDesc    string `json:"error_description"`
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
		Ret             int    `json:"ret"`
		OpenID          string `json:"openid"`
		Nickname        string `json:"nickname"`
		Avatar          string `json:"figureurl_qq_2"`
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

func FormatUsername(base string) string {
	base = strings.ToLower(base)
	base = strings.ReplaceAll(base, " ", "_")
	return base
}
