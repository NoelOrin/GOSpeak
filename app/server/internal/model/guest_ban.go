package model

import "time"

// DomainGuestBan 域名下的访客封禁记录；ExpiresAt 为 nil 表示永久封禁。
type DomainGuestBan struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	DomainUUID string     `gorm:"size:32;uniqueIndex:uniq_guest_ban_domain_user,priority:1;not null" json:"domain_uuid"`
	UserUUID   string     `gorm:"type:uuid;uniqueIndex:uniq_guest_ban_domain_user,priority:2;not null" json:"user_uuid"`
	Reason     string     `gorm:"size:255" json:"reason"`
	BannedBy   string     `gorm:"type:uuid" json:"banned_by"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

func (DomainGuestBan) TableName() string { return "domain_guest_bans" }

func (b *DomainGuestBan) IsActive() bool {
	return b != nil && (b.ExpiresAt == nil || b.ExpiresAt.After(time.Now()))
}
