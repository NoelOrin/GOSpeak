package model

import "time"

const (
	MessageStatusActive  = "active"
	MessageStatusDeleted = "deleted"
)

// ConversationType constants
const (
	ConversationTypeRoom   = "room"
	ConversationTypeDirect = "direct"
)

type Message struct {
	ID               string    `gorm:"primaryKey;size:26;index:idx_msg_room_status_id,priority:3" json:"id"`
	RoomUUID         *string   `gorm:"size:255;index:idx_msg_room_status_id,priority:1" json:"room_uuid"`
	GuildUUID        *string   `gorm:"type:uuid;index" json:"guild_uuid"`
	SenderIdentity   string    `gorm:"size:64;not null" json:"sender_identity"`
	SenderDisplay    string    `gorm:"size:128" json:"sender_display"`
	SenderRole       string    `gorm:"size:32" json:"sender_role"`
	Content          string    `gorm:"type:text;not null" json:"content"`
	ReplyToID        *string   `gorm:"size:26" json:"reply_to_id,omitempty"`
	Status           string    `gorm:"size:16;index:idx_msg_room_status_id,priority:2;default:active" json:"status"`
	ConversationType string    `gorm:"size:10;index:idx_msg_conversation,priority:1;default:room" json:"conversation_type"`
	ConversationID   *string   `gorm:"size:32;index:idx_msg_conversation,priority:2" json:"conversation_id"`
	TargetIdentity   *string   `gorm:"size:64;index" json:"target_identity"`
	CreatedAt        time.Time `json:"created_at"`
}

func (Message) TableName() string { return "messages" }
