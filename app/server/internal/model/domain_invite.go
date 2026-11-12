package model

import "time"

// DomainInvite 是域邀请码。Code 是自包含签名串，ExpiresAt 与 MaxUses
// 在创建时写入码内，之后不可修改；UsedCount 是库侧权威计数。
type DomainInvite struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	DomainUUID string     `gorm:"size:32;index:idx_domain_invite_domain,priority:1;not null" json:"domain_uuid"`
	Label      string     `gorm:"size:100" json:"label"`
	Code       string     `gorm:"size:64;uniqueIndex;not null" json:"code"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	MaxUses    int        `gorm:"default:0" json:"max_uses"`
	UsedCount  int        `gorm:"default:0" json:"used_count"`
	DisabledAt *time.Time `gorm:"index:idx_domain_invite_domain,priority:2" json:"disabled_at,omitempty"`
	CreatedBy  string     `gorm:"size:36;index" json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (DomainInvite) TableName() string { return "domain_invites" }

// IsDisabled 报告邀请码是否已被显式禁用。
func (i *DomainInvite) IsDisabled() bool { return i != nil && i.DisabledAt != nil }

// IsExhausted 报告已用次数是否达到上限（MaxUses 为 0 表示不限）。
func (i *DomainInvite) IsExhausted() bool {
	return i != nil && i.MaxUses > 0 && i.UsedCount >= i.MaxUses
}

// DomainInviteRedemption 是邀请码使用记录。失败尝试也记录，
// 不存 IP，不存码的修改历史。
type DomainInviteRedemption struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	InviteID   uint      `gorm:"index:idx_invite_redemption_invite;not null" json:"invite_id"`
	DomainUUID string    `gorm:"size:32;index" json:"domain_uuid"`
	UserUUID   string    `gorm:"size:36;index" json:"user_uuid"`
	Code       string    `gorm:"size:64" json:"code"`
	Success    bool      `gorm:"default:false" json:"success"`
	FailReason string    `gorm:"size:32" json:"fail_reason"`
	CreatedAt  time.Time `json:"created_at"`
}

func (DomainInviteRedemption) TableName() string { return "domain_invite_redemptions" }

// 邀请码使用失败原因，写入 DomainInviteRedemption.FailReason。
const (
	InviteFailExpired       = "expired"
	InviteFailExhausted     = "exhausted"
	InviteFailInvalid       = "invalid"
	InviteFailDisabled      = "disabled"
	InviteFailRejected      = "rejected"
	InviteFailAlreadyMember = "already_member"
)
