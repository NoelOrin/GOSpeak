package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	AnnouncementScopePlatform = "platform"
	AnnouncementScopeDomain   = "domain"

	AnnouncementLevelInfo     = "info"
	AnnouncementLevelWarning  = "warning"
	AnnouncementLevelCritical = "critical"
)

// Announcement 是站内系统公告。PublishedAt 为 nil 表示草稿，不进入任何用户可见集合。
type Announcement struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UUID          string         `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	ScopeType     string         `gorm:"size:16;not null;index:idx_announcement_visible,priority:1" json:"scope_type"`
	ScopeID       string         `gorm:"size:32;index:idx_announcement_visible,priority:2" json:"scope_id"`
	Title         string         `gorm:"size:200;not null" json:"title"`
	Content       string         `gorm:"type:text;not null" json:"content"`
	Level         string         `gorm:"size:16;default:info;index" json:"level"`
	Pinned        bool           `gorm:"default:false" json:"pinned"`
	PublishedAt   *time.Time     `gorm:"index:idx_announcement_visible,priority:3" json:"published_at,omitempty"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
	CreatedByUUID string         `gorm:"size:36;index" json:"created_by_uuid"`
	CreatedByName string         `gorm:"size:64" json:"created_by_name"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func (a *Announcement) BeforeCreate(_ *gorm.DB) error {
	if a.UUID == "" {
		a.UUID = uuid.New().String()
	}
	if a.Level == "" {
		a.Level = AnnouncementLevelInfo
	}
	if a.ScopeType == "" {
		a.ScopeType = AnnouncementScopePlatform
	}
	return nil
}

func (a *Announcement) TableName() string { return "announcements" }

// AnnouncementRead 记录「某用户已读某公告」，联合唯一保证幂等。
type AnnouncementRead struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	AnnouncementUUID string    `gorm:"size:36;uniqueIndex:idx_announcement_user;not null" json:"announcement_uuid"`
	UserUUID         string    `gorm:"size:36;uniqueIndex:idx_announcement_user;not null" json:"user_uuid"`
	ReadAt           time.Time `json:"read_at"`
}

func (AnnouncementRead) TableName() string { return "announcement_reads" }
