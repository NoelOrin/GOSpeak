package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLog 记录管理类敏感操作（踢人/禁言/删房/删用户等），用于安全审计与追溯。
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UUID       string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	ActorID    uint      `gorm:"index" json:"actor_id"`           // 操作者用户 ID
	ActorUUID  string    `gorm:"index" json:"actor_uuid"`         // 操作者用户 UUID
	ActorName  string    `gorm:"size:64" json:"actor_name"`       // 操作者用户名（冗余，便于追溯）
	Action     string    `gorm:"size:48;index" json:"action"`     // 动作常量，见 audit 包
	TargetType string    `gorm:"size:48" json:"target_type"`      // 目标类型：member/room/user/mute
	TargetID   string    `gorm:"size:128;index" json:"target_id"` // 目标标识（UUID / ID / name）
	Detail     string    `gorm:"type:text" json:"detail"`         // 操作细节（时长/原因等）
	IP         string    `gorm:"size:64" json:"ip"`
	Success    bool      `gorm:"index" json:"success"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }

func (a *AuditLog) BeforeCreate(_ *gorm.DB) error {
	if a.UUID == "" {
		a.UUID = uuid.New().String()
	}
	return nil
}
