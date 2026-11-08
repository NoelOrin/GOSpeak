package model

import "time"

type EmailConfig struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Enabled           bool      `gorm:"default:false" json:"enabled"`
	SMTPHost          string    `gorm:"size:255" json:"smtp_host"`
	SMTPPort          string    `gorm:"size:32;default:587" json:"smtp_port"`
	SMTPUsername      string    `gorm:"size:255" json:"smtp_username"`
	SMTPPassword      string    `gorm:"size:255" json:"-"`
	SMTPFrom          string    `gorm:"size:255" json:"smtp_from"`
	SMTPFromName      string    `gorm:"size:255" json:"smtp_from_name"`
	EmailCodeTTL      string    `gorm:"size:32;default:10m" json:"email_code_ttl"`
	EmailSendCooldown string    `gorm:"size:32;default:60s" json:"email_send_cooldown"`
	EmailCodeSecret   string    `gorm:"size:255" json:"-"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (EmailConfig) TableName() string {
	return "email_configs"
}
