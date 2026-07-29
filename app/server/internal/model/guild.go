package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Guild 代表一个语音服务器（类 Discord Server/Guild）。
type Guild struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UUID        string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	IconURL     string    `gorm:"size:512" json:"icon_url"`
	Description string    `gorm:"size:500" json:"description"`
	OwnerUUID   string    `gorm:"type:uuid;index;not null" json:"owner_uuid"`
	InviteCode  string    `gorm:"size:32;uniqueIndex" json:"invite_code"`
	MaxRooms    uint      `gorm:"default:0" json:"max_rooms"`
	IsPublic    bool      `gorm:"default:false" json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (g *Guild) TableName() string {
	return "guilds"
}

func (g *Guild) BeforeCreate(_ *gorm.DB) error {
	if g.UUID == "" {
		g.UUID = uuid.New().String()
	}
	if g.InviteCode == "" {
		g.InviteCode = generateInviteCode()
	}
	return nil
}

// GuildMember 用户-Guild 多对多关系。
type GuildMember struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GuildUUID string    `gorm:"type:uuid;index:idx_guild_member,priority:1;not null" json:"guild_uuid"`
	UserUUID  string    `gorm:"type:uuid;index:idx_guild_member,priority:2;not null" json:"user_uuid"`
	Nickname  string    `gorm:"size:64" json:"nickname"`
	RoleName  string    `gorm:"size:32;default:member" json:"role_name"`
	JoinedAt  time.Time `json:"joined_at"`
}

func (GuildMember) TableName() string {
	return "guild_members"
}

// generateInviteCode 生成 8 字符随机邀请码。
func generateInviteCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	u := uuid.New()
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[u.ID()%uint32(len(charset))]
	}
	return string(b)
}
