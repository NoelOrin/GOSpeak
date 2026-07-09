package model

import (
	"time"

	"GOSpeak/internal/permcode"
)

// AdminOnlyPermissions 是仅限管理员角色使用的权限码集合，禁止授予 Bot API Key，
// 避免 Bot 获得系统级管理能力：角色管理、SFU 配置、禁言管理、用户删除/修改、Bot 密钥管理本身。
var AdminOnlyPermissions = map[string]struct{}{
	permcode.PermRoleManage: {},
	permcode.PermSFUManage:  {},
	permcode.PermMuteManage: {},
	permcode.PermUserDelete: {},
	permcode.PermUserUpdate: {},
	permcode.PermBotManage:  {},
	// Bot 不应具备房间管理能力：禁止创建与删除房间。
	permcode.PermRoomCreate: {},
	permcode.PermRoomDelete: {},
}

// IsAdminOnlyPermission 判断权限码是否属于管理员专属（不可授予 Bot）。
func IsAdminOnlyPermission(code string) bool {
	_, ok := AdminOnlyPermissions[code]
	return ok
}

// BotAPIKey Bot 专用 API Key，携带受限权限集合与过期时间。
type BotAPIKey struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UUID        string     `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name        string     `gorm:"size:64" json:"name"`
	KeyHash     string     `gorm:"size:128;uniqueIndex" json:"-"`
	Permissions string     `gorm:"type:text" json:"permissions"`
	CreatedBy   string     `gorm:"size:64" json:"created_by"`
	ExpiresAt   time.Time  `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	Revoked     bool       `gorm:"default:false" json:"revoked"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (BotAPIKey) TableName() string {
	return "bot_api_keys"
}

// BotAPIKeyResponse 对外返回结构，隐藏 KeyHash。
type BotAPIKeyResponse struct {
	ID          uint       `json:"id"`
	UUID        string     `json:"uuid"`
	Name        string     `json:"name"`
	Permissions []string   `json:"permissions"`
	CreatedBy   string     `json:"created_by"`
	ExpiresAt   time.Time  `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	Revoked     bool       `json:"revoked"`
	CreatedAt   time.Time  `json:"created_at"`
}
