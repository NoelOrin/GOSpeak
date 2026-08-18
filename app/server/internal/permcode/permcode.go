// Package permcode 定义系统权限码常量。
// model 包和 route 包均依赖此包，避免路由层直接导入 model。
package permcode

const (
	PermRoomCreate = "room:create"
	PermRoomRead   = "room:read"
	PermRoomUpdate = "room:update"
	PermRoomDelete = "room:delete"

	PermUserRead   = "user:read"
	PermUserUpdate = "user:update"
	PermUserDelete = "user:delete"

	PermRoleRead   = "role:read"
	PermRoleManage = "role:manage"

	PermSignalKick = "signal:kick"
	PermMuteManage = "mute:manage"
	PermSFUManage  = "sfu:manage"

	PermBotManage = "bot:manage"

	PermEmailConfigRead   = "email_config:read"
	PermEmailConfigManage = "email_config:manage"

	PermMessageSend         = "message:send"
	PermMessageRead         = "message:read"
	PermMessageDeleteOthers = "message:delete_others"

	PermStorageRead   = "storage:read"
	PermStorageManage = "storage:manage"
	PermStorageDelete = "storage:delete"

	PermOAuthRead   = "oauth:read"
	PermOAuthManage = "oauth:manage"

	PermPluginRead   = "plugin:read"
	PermPluginManage = "plugin:manage"

	PermClusterRead   = "cluster:read"
	PermClusterManage = "cluster:manage"

	PermAuditRead = "audit:read"
)
