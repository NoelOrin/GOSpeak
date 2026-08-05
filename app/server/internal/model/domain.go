package model

import (
	"time"

	"github.com/nrednav/cuid2"
	"gorm.io/gorm"
)

// Domain 代表一个语音域（原 Domain/Server 语义）。
type Domain struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UUID        string    `gorm:"size:32;uniqueIndex" json:"uuid"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	IconURL     string    `gorm:"size:512" json:"icon_url"`
	Description string    `gorm:"size:500" json:"description"`
	OwnerUUID   string    `gorm:"type:uuid;index;not null" json:"owner_uuid"`
	InviteCode  string    `gorm:"size:32;uniqueIndex" json:"invite_code"`
	IsPublic    bool      `gorm:"default:false" json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (d *Domain) TableName() string {
	return "domains"
}

func (d *Domain) BeforeCreate(_ *gorm.DB) error {
	if d.UUID == "" {
		d.UUID = cuid2.Generate()
	}
	if d.InviteCode == "" {
		d.InviteCode = generateInviteCode()
	}
	return nil
}

// DomainMember 用户-Domain 多对多关系。
type DomainMember struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DomainUUID string    `gorm:"size:32;index:idx_domain_member,priority:1;not null" json:"domain_uuid"`
	UserUUID   string    `gorm:"type:uuid;index:idx_domain_member,priority:2;not null" json:"user_uuid"`
	Nickname   string    `gorm:"size:64" json:"nickname"`
	RoleName   string    `gorm:"size:32;default:member" json:"role_name"`
	JoinedAt   time.Time `json:"joined_at"`
}

func (DomainMember) TableName() string {
	return "domain_members"
}

// DomainMemberInfo 成员列表视图，附带用户全局名称。
type DomainMemberInfo struct {
	ID          uint      `json:"id"`
	DomainUUID  string    `json:"domain_uuid"`
	UserUUID    string    `json:"user_uuid"`
	Nickname    string    `json:"nickname"`
	RoleName    string    `json:"role_name"`
	JoinedAt    time.Time `json:"joined_at"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
}

// DomainDetail 我的语音域批量详情视图，附带成员数与房间数。
type DomainDetail struct {
	ID          uint      `json:"id"`
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	IconURL     string    `json:"icon_url"`
	Description string    `json:"description"`
	OwnerUUID   string    `json:"owner_uuid"`
	InviteCode  string    `json:"invite_code"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MemberCount int64     `json:"member_count"`
	RoomCount   int64     `json:"room_count"`
}

// generateInviteCode 生成 8 字符随机邀请码。
func generateInviteCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	u := cuid2.Generate()
	var b [8]byte
	for i := range b {
		c := u[i]
		var v byte
		switch {
		case c >= '0' && c <= '9':
			v = c - '0'
		case c >= 'a' && c <= 'z':
			v = c - 'a' + 10
		default:
			v = 0
		}
		b[i] = charset[int(v)%len(charset)]
	}
	return string(b[:])
}
