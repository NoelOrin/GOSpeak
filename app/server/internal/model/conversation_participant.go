package model

import "time"

// ConversationParticipant tracks a 2-person direct conversation session.
// IdentityA and IdentityB are always sorted lexicographically so lookups
// work regardless of who initiates the conversation.
type ConversationParticipant struct {
	ConversationID     string     `gorm:"primaryKey;size:32" json:"conversation_id"`
	IdentityA          string     `gorm:"size:64;not null;index:idx_conv_part_a" json:"identity_a"`
	IdentityB          string     `gorm:"size:64;not null;index:idx_conv_part_b" json:"identity_b"`
	LastMessageID      *string    `gorm:"size:26" json:"last_message_id"`
	LastContent        string     `gorm:"size:200" json:"last_content"`
	LastSenderIdentity string     `gorm:"size:64" json:"last_sender_identity"`
	LastMessageAt      *time.Time `json:"last_message_at"`
	UnreadCountA       int        `gorm:"default:0" json:"unread_count_a"`
	UnreadCountB       int        `gorm:"default:0" json:"unread_count_b"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (ConversationParticipant) TableName() string { return "conversation_participants" }
