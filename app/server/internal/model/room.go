package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Room struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UUID          string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name          string    `gorm:"index" json:"name"`
	Password      string    `json:"-"`
	Description   string    `gorm:"size:255" json:"description"`
	Limit         uint      `json:"limit"`
	AudioOnly     bool      `gorm:"not null;default:true" json:"audio_only"`
	AllowAudience bool      `gorm:"not null;default:true" json:"allow_audience"`
	CreatedBy     string    `gorm:"index;size:64" json:"created_by"`
	// GuildUUID 归属的语音服务器 UUID。空值表示平台级房间（向后兼容存量数据）。
	GuildUUID     string    `gorm:"type:uuid;index" json:"guild_uuid"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (r *Room) BeforeCreate(_ *gorm.DB) error {
	if r.UUID == "" {
		r.UUID = uuid.New().String()
	}
	return nil
}

func (r *Room) TableName() string {
	return "room"
}
