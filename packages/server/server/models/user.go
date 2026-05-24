package models

import (
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID        uint   `gorm:"primaryKey" json:"id"` // 仅用于维护表
	UUID      string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name      string `gorm:"uniqueIndex" json:"name"`
	Password  string `json:"password"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.UUID == "" {
		u.UUID = ulid.MustNew(ulid.Now(), nil).String()
	}
	return nil
}

func (u *User) TableName() string {
	return "users"
}
