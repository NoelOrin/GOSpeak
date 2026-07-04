// Package service — 角色业务逻辑：角色 CRUD 及缓存同步。
package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// RoleService 角色服务，封装角色 CRUD 及缓存管理。
type RoleService struct {
	roleRepo *repository.RoleRepository
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
	role := &model.Role{Name: name}
	if err := s.roleRepo.Create(role); err != nil {
		return nil, pkg.NewAppError(pkg.ALREADY_EXISTS, "role already exists")
	}
	s.reloadCache()
	return role, nil
}

// Update 更新角色名称并刷新缓存。
func (s *RoleService) Update(id uint, name string) (*model.Role, error) {
	role, err := s.roleRepo.Update(id, name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "role not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.reloadCache()
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
	return nil
}

// reloadCache 重新加载角色缓存，确保缓存与 DB 一致。
func (s *RoleService) reloadCache() {
	if roles, err := s.roleRepo.List(); err == nil {
		model.LoadRoleCache(roles)
	}
}
