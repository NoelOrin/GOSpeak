package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BotToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UUID      string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name      string    `gorm:"size:64;uniqueIndex" json:"name"`
	UserUUID  string    `gorm:"type:uuid;index" json:"user_uuid"`
	Role      string    `gorm:"size:32;default:user" json:"role"`
	Revoked   bool      `gorm:"default:false" json:"revoked"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BotToken) TableName() string {
	return "bot_tokens"
}

func (bt *BotToken) BeforeCreate(_ *gorm.DB) error {
	if bt.UUID == "" {
		bt.UUID = uuid.New().String()
	}
	return nil
}
