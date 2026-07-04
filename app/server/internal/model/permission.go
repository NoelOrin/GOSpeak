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

// DefaultRolePermissions 默认角色权限映射
var DefaultRolePermissions = map[string][]string{
	"admin": {
		PermRoomCreate, PermRoomRead, PermRoomUpdate, PermRoomDelete,
		PermUserRead, PermUserUpdate, PermUserDelete,
		PermRoleRead, PermRoleManage,
		PermSignalKick, PermMuteManage, PermSFUManage,
	},
	"user": {
		PermRoomCreate, PermRoomRead,
		PermUserRead,
		PermRoleRead,
	},
	"ban": {},
}
