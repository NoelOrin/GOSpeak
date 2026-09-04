package model

type MessageMention struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	MessageUUID string `gorm:"size:36;uniqueIndex:idx_message_user;not null" json:"message_uuid"`
	UserID      string `gorm:"size:64;uniqueIndex:idx_message_user;not null" json:"user_id"`
}

func (MessageMention) TableName() string { return "message_mentions" }
