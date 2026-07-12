package oauth

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// FieldMapping 自建 OAuth 提供商的用户信息字段映射。
// 指定 UserInfo JSON 响应中各字段的 key 名，为空时使用默认值。
type FieldMapping struct {
	UIDField      string // 用户唯一标识字段，默认 "id"
	UsernameField string // 用户名字段，默认 "username"
	AvatarField   string // 头像字段，默认 "avatar_url"
	EmailField    string // 邮箱字段，默认 "email"
}

// GenericProvider 通用 OAuth2 提供商实现，支持任意标准 OAuth2 服务端。
// 通过 FieldMapping 适配不同平台的用户信息 JSON 结构。
type GenericProvider struct {
	cfg    *ProviderConfig
	name   string
	fields FieldMapping
}

// NewGenericProvider 创建通用 OAuth 提供商实例，支持自定义字段映射。
func NewGenericProvider(name string, cfg *ProviderConfig, fields FieldMapping) *GenericProvider {
	return &GenericProvider{cfg: cfg, name: name, fields: fields}
}

func (g *GenericProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", g.cfg.ClientID)
	params.Set("redirect_uri", g.cfg.RedirectURL)
	params.Set("response_type", "code")
	if g.cfg.Scopes != "" {
		params.Set("scope", g.cfg.Scopes)
	}
	params.Set("state", state)
	return g.cfg.AuthURL + "?" + params.Encode()
}

func (g *GenericProvider) ExchangeToken(code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", g.cfg.ClientID)
	data.Set("client_secret", g.cfg.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", g.cfg.RedirectURL)

	body, err := httpPostForm(g.cfg.TokenURL, data)
	if err != nil {
		return "", err
	}

	// 先尝试 JSON 解析，再回退到 URL 编码
	var jsonResult struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &jsonResult); err == nil {
		if jsonResult.AccessToken != "" {
			return jsonResult.AccessToken, nil
		}
		if jsonResult.Error != "" {
			return "", fmt.Errorf("%s: %s", g.name, jsonResult.Error)
		}
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return "", err
	}
	token := values.Get("access_token")
	if token == "" {
		return "", fmt.Errorf("%s: no access_token in response", g.name)
	}
	return token, nil
}

func (g *GenericProvider) GetUserInfo(accessToken string) (*UserInfo, error) {
	body, err := httpGet(g.cfg.UserInfoURL, accessToken)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	uidKey := g.fields.UIDField
	if uidKey == "" {
		uidKey = "id"
	}
	usernameKey := g.fields.UsernameField
	if usernameKey == "" {
		usernameKey = "username"
	}
	avatarKey := g.fields.AvatarField
	if avatarKey == "" {
		avatarKey = "avatar_url"
	}
	emailKey := g.fields.EmailField
	if emailKey == "" {
		emailKey = "email"
	}

	toString := func(v interface{}) string {
		switch val := v.(type) {
		case string:
			return val
		case float64:
			return fmt.Sprintf("%v", int64(val))
		case json.Number:
			return val.String()
		default:
			return fmt.Sprintf("%v", v)
		}
	}

	return &UserInfo{
		Provider:    g.name,
		ProviderUID: toString(raw[uidKey]),
		Username:    toString(raw[usernameKey]),
		Avatar:      toString(raw[avatarKey]),
		Email:       toString(raw[emailKey]),
	}, nil
}
