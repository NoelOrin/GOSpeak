package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Message struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UUID      string         `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	RoomUUID  string         `gorm:"size:36;index:idx_msg_room_created,priority:1;not null" json:"room_uuid"`
	AuthorID  string         `gorm:"size:64;index;not null" json:"author_id"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	ReplyTo   string         `gorm:"size:36;index" json:"reply_to,omitempty"`
	EditedAt  *time.Time     `json:"edited_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedAt time.Time      `gorm:"index:idx_msg_room_created,priority:2" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (m *Message) BeforeCreate(_ *gorm.DB) error {
	if m.UUID == "" {
		m.UUID = uuid.New().String()
	}
	return nil
}

func (m *Message) TableName() string { return "messages" }
