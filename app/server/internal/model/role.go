package model

import "time"

type Role struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Role) TableName() string {
	return "roles"
}

// DefaultRoles 启动时种子写入的角色列表。
var DefaultRoles = []Role{
	{Name: "admin"},
	{Name: "user"},
	{Name: "ban"},
}

// roleCache 进程级角色缓存，启动时从 roles 表加载。
var roleCache = map[string]struct{}{}

// LoadRoleCache 从数据库加载所有角色到内存缓存。
func LoadRoleCache(roles []Role) {
	for k := range roleCache {
		delete(roleCache, k)
	}
	for _, r := range roles {
		roleCache[r.Name] = struct{}{}
	}
}

// IsValidRole 检查角色名是否存在于 roles 表。
func IsValidRole(name string) bool {
	_, ok := roleCache[name]
	return ok
}

// HasBanRole 检查角色是否为 ban。
func HasBanRole(name string) bool {
	_, ok := roleCache[name]
	return ok && name == "ban"
}
