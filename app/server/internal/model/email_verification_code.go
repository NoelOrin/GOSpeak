package model

import "time"

type EmailVerificationCode struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Email        string     `gorm:"size:128;index:idx_email_scene_created" json:"email"`
	Scene        string     `gorm:"size:64;index:idx_email_scene_created" json:"scene"`
	CodeHash     string     `gorm:"size:255;not null" json:"-"`
	UserID       *uint      `json:"user_id"`
	IPAddress    string     `gorm:"size:64;index" json:"ip_address"`
	ExpiresAt    time.Time  `json:"expires_at"`
	UsedAt       *time.Time `json:"used_at"`
	AttemptCount int        `gorm:"default:0" json:"attempt_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (EmailVerificationCode) TableName() string {
	return "email_verification_codes"
}
