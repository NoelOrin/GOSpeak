package model

import "time"

type UserGroup struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_group_name,priority:1;not null" json:"user_id"`
	GroupName string    `gorm:"size:64;uniqueIndex:idx_user_group_name,priority:2;not null" json:"group_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserGroup) TableName() string {
	return "user_groups"
}
