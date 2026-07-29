package model

import (
	"GOSpeak/internal/permcode"
	"time"
)

type Permission struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"uniqueIndex;size:64" json:"code"`
	Name        string    `gorm:"size:64" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

func (Permission) TableName() string {
	return "permissions"
}

// 所有可用权限定义
const (
	PermRoomCreate = permcode.PermRoomCreate
	PermRoomRead   = permcode.PermRoomRead
	PermRoomUpdate = permcode.PermRoomUpdate
	PermRoomDelete = permcode.PermRoomDelete

	PermUserRead   = permcode.PermUserRead
	PermUserUpdate = permcode.PermUserUpdate
	PermUserDelete = permcode.PermUserDelete

	PermRoleRead   = permcode.PermRoleRead
	PermRoleManage = permcode.PermRoleManage

	PermSignalKick = permcode.PermSignalKick
	PermMuteManage = permcode.PermMuteManage
	PermSFUManage  = permcode.PermSFUManage

	PermBotManage = permcode.PermBotManage

	PermEmailConfigRead   = permcode.PermEmailConfigRead
	PermEmailConfigManage = permcode.PermEmailConfigManage

	PermStorageRead   = permcode.PermStorageRead
	PermStorageManage = permcode.PermStorageManage
	PermStorageDelete = permcode.PermStorageDelete

	PermOAuthRead   = permcode.PermOAuthRead
	PermOAuthManage = permcode.PermOAuthManage

	PermPluginRead   = permcode.PermPluginRead
	PermPluginManage = permcode.PermPluginManage
	PermGuildCreate     = permcode.PermGuildCreate
	PermGuildRead       = permcode.PermGuildRead
	PermGuildManage     = permcode.PermGuildManage
	PermGuildDelete     = permcode.PermGuildDelete
	PermGuildInvite     = permcode.PermGuildInvite
	PermGuildKick       = permcode.PermGuildKick
	PermGuildRoleManage = permcode.PermGuildRoleManage
)


// DefaultPermissions 种子权限列表
var DefaultPermissions = []Permission{
	{Code: PermRoomCreate, Name: "创建房间", Description: "创建新的语音房间"},
	{Code: PermRoomRead, Name: "查看房间", Description: "查看房间列表和详情"},
	{Code: PermRoomUpdate, Name: "编辑房间", Description: "修改房间名称、人数上限等"},
	{Code: PermRoomDelete, Name: "删除房间", Description: "删除房间"},

	{Code: PermUserRead, Name: "查看用户", Description: "查看用户列表和详情"},
	{Code: PermUserUpdate, Name: "编辑用户", Description: "修改用户信息"},
	{Code: PermUserDelete, Name: "删除用户", Description: "删除用户账号"},

	{Code: PermRoleRead, Name: "查看角色", Description: "查看角色列表"},
	{Code: PermRoleManage, Name: "管理角色", Description: "创建、删除角色和分配权限"},

	{Code: PermSignalKick, Name: "踢出房间", Description: "将用户从语音房间中踢出"},
	{Code: PermMuteManage, Name: "管理禁言", Description: "对用户进行全局禁言/取消禁言/查看禁言列表"},
	{Code: PermSFUManage, Name: "管理 SFU", Description: "查看和修改 SFU 提供商配置"},
	{Code: PermBotManage, Name: "管理 BOT 密钥", Description: "创建、查看、吊销 BOT 专用 API Key"},

	{Code: PermEmailConfigRead, Name: "查看邮件配置", Description: "查看 SMTP 邮件服务器配置"},
	{Code: PermEmailConfigManage, Name: "管理邮件配置", Description: "修改 SMTP 邮件服务器配置"},

	{Code: PermStorageRead, Name: "查看存储配置", Description: "查看对象存储配置"},
	{Code: PermStorageManage, Name: "管理存储配置", Description: "修改对象存储配置"},
	{Code: PermStorageDelete, Name: "删除存储对象", Description: "删除对象存储中的文件"},

	{Code: PermOAuthRead, Name: "查看 OAuth 配置", Description: "查看第三方/自建 OAuth 登录提供商配置"},
	{Code: PermOAuthManage, Name: "管理 OAuth 配置", Description: "创建、修改、删除 OAuth 登录提供商"},

	{Code: PermPluginRead, Name: "查看插件", Description: "查看后端插件列表与配置"},
	{Code: PermPluginManage, Name: "管理插件", Description: "启用/停用插件并修改插件配置"},
	{Code: PermGuildCreate, Name: "创建语音服务器", Description: "创建新的语音服务器"},
	{Code: PermGuildRead, Name: "查看语音服务器", Description: "查看语音服务器列表"},
	{Code: PermGuildManage, Name: "管理语音服务器", Description: "修改语音服务器设置"},
	{Code: PermGuildDelete, Name: "删除语音服务器", Description: "删除语音服务器"},
	{Code: PermGuildInvite, Name: "邀请成员", Description: "生成和管理邀请码"},
	{Code: PermGuildKick, Name: "踢出成员", Description: "将成员移出语音服务器"},
	{Code: PermGuildRoleManage, Name: "管理 Guild 角色", Description: "管理语音服务器内的角色和权限"},
}


// RolePermission 角色-权限关联表
type RolePermission struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	RoleName     string `gorm:"index;size:32" json:"role_name"`
	PermissionID uint   `gorm:"index" json:"permission_id"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}

// BotScopedPermissions Bot 可被授予的权限码白名单。
// 仅暴露业务面权限，避免 Bot 接触角色/用户管理、凭据配置、SFU 等平台级管理面。
var BotScopedPermissions = []string{
	PermRoomRead,
	PermUserRead,
	PermSignalKick,
	PermRoomCreate,
	PermMuteManage,
}

// BotScopedPermissionsSet 返回 Bot 允许权限码集合。
func BotScopedPermissionsSet() map[string]struct{} {
	set := make(map[string]struct{}, len(BotScopedPermissions))
	for _, c := range BotScopedPermissions {
		set[c] = struct{}{}
	}
	return set
}

// DefaultRolePermissions 默认角色权限映射
var DefaultRolePermissions = map[string][]string{
	"admin": {
		PermRoomCreate, PermRoomRead, PermRoomUpdate, PermRoomDelete,
		PermUserRead, PermUserUpdate, PermUserDelete,
		PermRoleRead, PermRoleManage,
		PermSignalKick,
	PermRoomCreate, PermMuteManage, PermSFUManage, PermBotManage,
		PermEmailConfigRead, PermEmailConfigManage,
		PermStorageRead, PermStorageManage, PermStorageDelete,
		PermOAuthRead, PermOAuthManage,
		PermPluginRead, PermPluginManage,
		PermGuildCreate, PermGuildRead, PermGuildManage, PermGuildDelete,
		PermGuildInvite, PermGuildKick, PermGuildRoleManage,
	},
	"user": {
		PermRoomCreate, PermRoomRead,
		PermUserRead,
		PermRoleRead,
		PermGuildCreate, PermGuildRead,
	},
	"ban": {},
}

// ValidPermissionSet 返回所有已知权限码集合，用于校验外部传入的权限码。
func ValidPermissionSet() map[string]struct{} {
	set := make(map[string]struct{}, len(DefaultPermissions))
	for _, p := range DefaultPermissions {
		set[p.Code] = struct{}{}
	}
	return set
}
