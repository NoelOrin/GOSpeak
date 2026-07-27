package model

import "time"

const (
	MessageStatusActive  = "active"
	MessageStatusDeleted = "deleted"
)

type Message struct {
	ID             string    `gorm:"primaryKey;size:26" json:"id"`
	RoomUUID       string    `gorm:"size:36;index:idx_msg_room_id,priority:1;not null" json:"room_uuid"`
	SenderIdentity string    `gorm:"size:64;not null" json:"sender_identity"`
	SenderDisplay  string    `gorm:"size:128" json:"sender_display"`
	SenderRole     string    `gorm:"size:32" json:"sender_role"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	ReplyToID      string    `gorm:"size:26" json:"reply_to_id,omitempty"`
	Status         string    `gorm:"size:16;index;default:active" json:"status"`
	CreatedAt      time.Time `gorm:"index:idx_msg_room_id,priority:2" json:"created_at"`
}

func (Message) TableName() string { return "messages" }
