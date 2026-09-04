// Package service — 角色业务逻辑：角色 CRUD 及缓存同步。
package service

import (
	"strings"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// RoleService 角色服务，封装角色 CRUD 及缓存管理。
type RoleService struct {
	roleRepo *repository.RoleRepository
}

// onRoleChanged 角色增删改后触发的回调（权限缓存/Casbin 策略重载），由组合根注入。
var onRoleChanged func()

// onRoleRenamed 角色改名回调（迁移 role_permissions 关联行 + Casbin 策略重载）。
var onRoleRenamed func(oldName, newName string) error

// SetOnRoleRenamed 注册角色改名回调。
func (s *RoleService) SetOnRoleRenamed(fn func(oldName, newName string) error) {
	onRoleRenamed = fn
}

// SetOnRoleChanged 注册角色变更回调，避免 RoleService 直接依赖 PermissionService。
func (s *RoleService) SetOnRoleChanged(fn func()) {
	onRoleChanged = fn
}

func (s *RoleService) notifyChanged() {
	if onRoleChanged != nil {
		onRoleChanged()
	}
}

// validateRoleName 规范化并校验角色名：去空白、1-64 字符。
func validateRoleName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "role name must be 1-64 characters")
	}
	return name, nil
}

func NewRoleService(roleRepo *repository.RoleRepository) *RoleService {
	return &RoleService{roleRepo: roleRepo}
}

// List 获取所有角色列表。
func (s *RoleService) List() ([]model.Role, error) {
	roles, err := s.roleRepo.List()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return roles, nil
}

// Create 创建新角色并刷新缓存。
func (s *RoleService) Create(name string) (*model.Role, error) {
	name, err := validateRoleName(name)
	if err != nil {
		return nil, err
	}
	role := &model.Role{Name: name}
	if err := s.roleRepo.Create(role); err != nil {
		return nil, pkg.NewAppError(pkg.ALREADY_EXISTS, "role already exists")
	}
	s.reloadCache()
	s.notifyChanged()
	return role, nil
}

// Update 更新角色名称并刷新缓存。
// 角色名是权限缓存/Casbin 策略的 key：改名后必须连带重载权限缓存。
func (s *RoleService) Update(id uint, name string) (*model.Role, error) {
	name, err := validateRoleName(name)
	if err != nil {
		return nil, err
	}
	oldRole, err := s.roleRepo.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "role not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	role, err := s.roleRepo.Update(id, name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "role not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	// 角色名是权限关联行与 Casbin 策略的 key，改名必须连带迁移，否则权限立即丢失。
	if onRoleRenamed != nil && oldRole.Name != name {
		if err := onRoleRenamed(oldRole.Name, name); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "role renamed but permissions migration failed: "+err.Error())
		}
	}
	s.reloadCache()
	s.notifyChanged()
	return role, nil
}

// Delete 删除角色并刷新缓存。
func (s *RoleService) Delete(id uint) error {
	if err := s.roleRepo.Delete(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.NewAppError(pkg.NOT_FOUND, "role not found")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.reloadCache()
	s.notifyChanged()
	return nil
}

// reloadCache 重新加载角色缓存，确保缓存与 DB 一致。
func (s *RoleService) reloadCache() {
	if roles, err := s.roleRepo.List(); err == nil {
		model.LoadRoleCache(roles)
	}
}
