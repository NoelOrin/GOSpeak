package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

const (
	UserStatusActive   = "active"
	UserStatusBanned   = "banned"
	UserStatusDisabled = "disabled"
)

type User struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	UUID          string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name          string `gorm:"uniqueIndex" json:"name"`
	DisplayName   string `gorm:"default:''" json:"display_name"`
	Avatar        string `gorm:"default:''" json:"avatar"`
	Email         string `gorm:"size:128;index" json:"email"`
	EmailVerified bool   `gorm:"default:false" json:"email_verified"`
	IsBot         bool   `gorm:"default:false" json:"is_bot"`
	IsGuest       bool   `gorm:"default:false;index" json:"is_guest"`
	Password      string `json:"-"`
	Role          string `gorm:"default:user" json:"role"`
	Status        string `gorm:"size:16;default:active;index" json:"status"`
	// Permissions 由 profile 等接口按角色动态下发，不落库。
	Permissions  []string  `gorm:"-" json:"permissions,omitempty"`
	TokenVersion uint      `gorm:"default:0" json:"token_version"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.UUID == "" {
		u.UUID = uuid.New().String()
	}
	if u.Status == "" {
		u.Status = UserStatusActive
	}
	return nil
}

// IsBanned 显式状态优先，兼容历史 Role=="ban" 的存量数据。
func (u *User) IsBanned() bool {
	return u != nil && (u.Status == UserStatusBanned || HasBanRole(u.Role))
}

func (u *User) TableName() string {
	return "users"
}
