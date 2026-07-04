package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"sync"
)

// PermissionService 权限服务，管理权限定义和角色权限缓存。
type PermissionService struct {
	permRepo *repository.PermissionRepository

	// 缓存：roleName → permission codes
	cache   map[string]map[string]struct{}
	cacheMu sync.RWMutex
}

func NewPermissionService(permRepo *repository.PermissionRepository) *PermissionService {
	return &PermissionService{
		permRepo: permRepo,
		cache:    make(map[string]map[string]struct{}),
	}
}

// LoadCache 启动时从 DB 加载所有角色权限到内存。
func (s *PermissionService) LoadCache() error {
	rolePerms, err := s.permRepo.GetAllRolePermissions()
	if err != nil {
		return err
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache = make(map[string]map[string]struct{}, len(rolePerms))
	for role, codes := range rolePerms {
		set := make(map[string]struct{}, len(codes))
		for _, c := range codes {
			set[c] = struct{}{}
		}
		s.cache[role] = set
	}
	return nil
}

// HasPermission 检查角色是否拥有指定权限码。
func (s *PermissionService) HasPermission(roleName, permCode string) bool {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	perms, ok := s.cache[roleName]
	if !ok {
		return false
	}
	_, found := perms[permCode]
	return found
}

// GetRolePermissions 获取角色的权限码列表。
func (s *PermissionService) GetRolePermissions(roleName string) []string {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	perms, ok := s.cache[roleName]
	if !ok {
		return nil
	}
	codes := make([]string, 0, len(perms))
	for c := range perms {
		codes = append(codes, c)
	}
	return codes
}

// ListPermissions 获取所有权限定义。
func (s *PermissionService) ListPermissions() ([]model.Permission, error) {
	return s.permRepo.List()
}

// SyncRolePermissions 同步角色权限并刷新缓存。
func (s *PermissionService) SyncRolePermissions(roleName string, permCodes []string) error {
	if err := s.permRepo.SyncRolePermissions(roleName, permCodes); err != nil {
		return err
	}
	// 刷新该角色的缓存
	s.cacheMu.Lock()
	set := make(map[string]struct{}, len(permCodes))
	for _, c := range permCodes {
		set[c] = struct{}{}
	}
	s.cache[roleName] = set
	s.cacheMu.Unlock()
	return nil
}
