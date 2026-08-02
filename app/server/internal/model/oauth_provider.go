package model

import "time"

type OAuthProvider struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Name         string `gorm:"uniqueIndex;size:32" json:"name"`
	DisplayName  string `gorm:"size:64" json:"display_name"`
	IconURL      string `gorm:"size:512" json:"icon_url"`
	ClientID     string `gorm:"size:255" json:"client_id"`
	ClientSecret string `gorm:"size:1024" json:"-"`
	AuthURL      string `gorm:"size:512" json:"auth_url"`
	TokenURL     string `gorm:"size:512" json:"token_url"`
	UserInfoURL  string `gorm:"size:512" json:"userinfo_url"`
	RedirectURL  string `gorm:"size:512" json:"redirect_url"`
	Scopes       string `gorm:"size:512" json:"scopes"`
	// 自建 OAuth 的用户信息字段映射（JSON key），为空时使用默认值
	UIDField      string    `gorm:"size:128" json:"uid_field"`
	UsernameField string    `gorm:"size:128" json:"username_field"`
	AvatarField   string    `gorm:"size:128" json:"avatar_field"`
	EmailField    string    `gorm:"size:128" json:"email_field"`
	Enabled       bool      `gorm:"default:true" json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (OAuthProvider) TableName() string {
	return "oauth_providers"
}
