package model

import "time"

type MessageReaction struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	MessageUUID string    `gorm:"size:36;uniqueIndex:idx_react_unique,priority:1;not null" json:"message_uuid"`
	UserID      string    `gorm:"size:64;uniqueIndex:idx_react_unique,priority:2;not null" json:"user_id"`
	Emoji       string    `gorm:"size:32;uniqueIndex:idx_react_unique,priority:3;not null" json:"emoji"`
	CreatedAt   time.Time `json:"created_at"`
}

func (MessageReaction) TableName() string { return "message_reactions" }
