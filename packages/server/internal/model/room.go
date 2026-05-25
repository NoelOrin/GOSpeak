package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Room struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UUID      string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name      string `json:"name"`
	Limit     uint   `json:"limit"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *Room) BeforeCreate(_ *gorm.DB) error {
	if r.UUID == "" {
		r.UUID = uuid.New().String()
	}
	return nil
}

func (r *Room) TableName() string {
	return "room"
}