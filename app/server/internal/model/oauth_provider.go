package model

import "time"

type OAuthProvider struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `gorm:"uniqueIndex;size:32" json:"name"`
	ClientID     string    `gorm:"size:255" json:"client_id"`
	ClientSecret string    `gorm:"size:255" json:"client_secret"`
	AuthURL      string    `gorm:"size:512" json:"auth_url"`
	TokenURL     string    `gorm:"size:512" json:"token_url"`
	UserInfoURL  string    `gorm:"size:512" json:"userinfo_url"`
	RedirectURL  string    `gorm:"size:512" json:"redirect_url"`
	Scopes       string    `gorm:"size:512" json:"scopes"`
	Enabled      bool      `gorm:"default:true" json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (OAuthProvider) TableName() string {
	return "oauth_providers"
}
