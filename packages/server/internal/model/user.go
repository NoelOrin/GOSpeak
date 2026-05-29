package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UUID      string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name      string `gorm:"uniqueIndex" json:"name"`
	Avatar    string `gorm:"default:''" json:"avatar"`
	Password  string `json:"-"`
	Role      string `gorm:"default:user" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.UUID == "" {
		u.UUID = uuid.New().String()
	}
	return nil
}

func (u *User) TableName() string {
	return "users"
}