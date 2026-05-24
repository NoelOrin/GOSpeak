package models

import (
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"time"
)

type Room struct {
	ID        uint   `gorm:"primaryKey" json:"id"` // 仅用于维护表
	UUID      string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name      string
	Limit     uint
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (r *Room) BeforeCreate(_ *gorm.DB) error {
	if r.UUID == "" {
		r.UUID = ulid.MustNew(ulid.Now(), nil).String()
	}
	return nil
}

func (r *Room) TableName() string {
	return "room"
}
