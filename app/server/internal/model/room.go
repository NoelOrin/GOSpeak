package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Room struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	UUID          string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name          string `gorm:"uniqueIndex:idx_room_domain_name,priority:1" json:"name"`
	Password      string `json:"-"`
	Description   string `gorm:"size:255" json:"description"`
	Limit         uint   `json:"limit"`
	AudioOnly     bool   `gorm:"not null;default:true" json:"audio_only"`
	AllowAudience bool   `gorm:"not null;default:true" json:"allow_audience"`
	Type          string `gorm:"size:16;not null;default:voice;index" json:"type"`
	CreatedBy     string `gorm:"index;size:64" json:"created_by"`
	// DomainUUID 归属的语音域 CUID2，不允许为空。
	DomainUUID string    `gorm:"size:32;uniqueIndex:idx_room_domain_name,priority:2;not null" json:"domain_uuid"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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

const (
	RoomTypeText  = "text"
	RoomTypeVoice = "voice"
)

func NormalizeRoomType(t string) string {
	switch t {
	case RoomTypeText:
		return RoomTypeText
	default:
		return RoomTypeVoice
	}
}
