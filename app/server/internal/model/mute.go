package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Mute 持久化禁言记录，全局生效
type Mute struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UUID      string     `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	UserID    uint       `gorm:"uniqueIndex" json:"user_id"`
	MuterID   uint       `json:"muter_id"`
	Duration  int64      `json:"duration"` // 禁言秒数，0 = 永久
	Permanent bool       `gorm:"default:false" json:"permanent"`
	ExpiresAt *time.Time `json:"expires_at"` // nil = 永久禁言
	Reason    string     `gorm:"size:255" json:"reason"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (Mute) TableName() string {
	return "mutes"
}

func (m *Mute) BeforeCreate(_ *gorm.DB) error {
	if m.UUID == "" {
		m.UUID = uuid.New().String()
	}
	return nil
}
