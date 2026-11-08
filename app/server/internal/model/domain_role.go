package model

import "time"

const (
	DomainRoleOwner  = "owner"
	DomainRoleAdmin  = "admin"
	DomainRoleMember = "member"
	DomainRoleGuest  = "guest"
)

// DomainRole 每个域独立角色。
type DomainRole struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DomainUUID string    `gorm:"size:32;uniqueIndex:idx_domain_roles_uuid_name,priority:1" json:"domain_uuid"`
	Name       string    `gorm:"size:32;uniqueIndex:idx_domain_roles_uuid_name,priority:2" json:"name"`
	IsSystem   bool      `gorm:"default:false" json:"is_system"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (DomainRole) TableName() string {
	return "domain_roles"
}

// DomainRolePermission 域角色-权限码关联表。权限码必须属于 AssignableDomainPermissions。
type DomainRolePermission struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	DomainUUID     string `gorm:"size:32;uniqueIndex:idx_domain_role_perms,priority:1" json:"domain_uuid"`
	RoleName       string `gorm:"size:32;uniqueIndex:idx_domain_role_perms,priority:2" json:"role_name"`
	PermissionCode string `gorm:"size:64;uniqueIndex:idx_domain_role_perms,priority:3" json:"permission_code"`
}

func (DomainRolePermission) TableName() string {
	return "domain_role_permissions"
}

// AssignableDomainPermissions 域角色可分配的权限码白名单。
var AssignableDomainPermissions = []string{
	PermDomainManage, PermDomainInvite, PermDomainKick, PermDomainRoleManage,
	PermRoomCreate, PermRoomRead, PermRoomUpdate, PermRoomDelete,
	PermMessageSend, PermMessageRead, PermMessageDeleteOthers,
}

func AssignableDomainPermissionsSet() map[string]struct{} {
	set := make(map[string]struct{}, len(AssignableDomainPermissions))
	for _, code := range AssignableDomainPermissions {
		set[code] = struct{}{}
	}
	return set
}

// DefaultDomainRolePermissions 每个域创建时 seed 的系统角色权限。owner 不存权限行。
var DefaultDomainRolePermissions = map[string][]string{
	DomainRoleAdmin: {
		PermDomainManage, PermDomainInvite, PermDomainKick, PermDomainRoleManage,
		PermRoomCreate, PermRoomRead, PermRoomUpdate, PermRoomDelete,
		PermMessageSend, PermMessageRead, PermMessageDeleteOthers,
	},
	DomainRoleMember: {
		PermRoomCreate, PermRoomRead, PermMessageSend, PermMessageRead,
	},
	DomainRoleGuest: {
		PermRoomRead, PermMessageRead,
	},
}

func IsSystemDomainRole(name string) bool {
	switch name {
	case DomainRoleOwner, DomainRoleAdmin, DomainRoleMember, DomainRoleGuest:
		return true
	default:
		return false
	}
}
