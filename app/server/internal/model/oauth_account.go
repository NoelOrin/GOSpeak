package model

import "time"

type OAuthAccount struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index" json:"user_id"`
	Provider     string    `gorm:"index;size:32" json:"provider"`
	ProviderUID  string    `gorm:"index;size:255" json:"provider_uid"`
	AccessToken  string    `gorm:"size:512" json:"-"`
	RefreshToken string    `gorm:"size:512" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (OAuthAccount) TableName() string {
	return "oauth_accounts"
}
