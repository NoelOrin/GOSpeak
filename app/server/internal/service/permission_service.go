package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	casbinpersist "github.com/casbin/casbin/v2/persist"
)

const EventPermissionsInvalidated = "cache:permissions-invalidated"

// cacheBus publishes internal cache invalidation events (no WebSocket).
type cacheBus interface {
	PublishInternal(ctx context.Context, event string, payload interface{}) error
}

// PermissionService 权限服务，管理权限定义和角色权限缓存。
type PermissionService struct {
	permRepo  *repository.PermissionRepository
	enforcer  *casbin.SyncedEnforcer
	useCasbin bool

	// 缓存：roleName → permission codes
	cache   map[string]map[string]struct{}
	cacheMu sync.RWMutex
	bus     cacheBus
}

func NewPermissionService(permRepo *repository.PermissionRepository) *PermissionService {
	return &PermissionService{
		permRepo: permRepo,
		cache:    make(map[string]map[string]struct{}),
		enforcer: &casbin.SyncedEnforcer{},
	}
}

// UseCasbin loads role permissions into a Casbin enforcer and makes it the
// authoritative HasPermission implementation. The legacy cache stays warm so
// existing callers and rollback paths keep working during migration.
func (s *PermissionService) UseCasbin(adapter casbinpersist.Adapter) error {
	casbinModel, err := casbinmodel.NewModelFromString(casbinModelText())
	if err != nil {
		return fmt.Errorf("load casbin model: %w", err)
	}
	enforcer, err := casbin.NewSyncedEnforcer(casbinModel, adapter)
	if err != nil {
		return fmt.Errorf("init casbin enforcer: %w", err)
	}
	if err := enforcer.LoadPolicy(); err != nil {
		return fmt.Errorf("load casbin policy: %w", err)
	}
	s.cacheMu.Lock()
	s.enforcer = enforcer
	s.useCasbin = true
	s.cacheMu.Unlock()
	return nil
}

func (s *PermissionService) SetEventBus(b cacheBus) {
	s.bus = b
}

// OnRemoteInvalidate reloads role permission cache from DB.
func (s *PermissionService) OnRemoteInvalidate(payload interface{}) {
	if err := s.LoadCache(); err != nil {
		log.Printf("[Permission] remote cache reload failed: %v", err)
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
	enforcer := s.enforcer
	hasCasbin := s.useCasbin
	s.cacheMu.RUnlock()
	if hasCasbin && enforcer != nil {
		allowed, err := enforcer.Enforce(roleName, permCode)
		if err != nil {
			log.Printf("[Permission] casbin enforce failed (role=%s perm=%s): %v", roleName, permCode, err)
			return false
		}
		return allowed
	}

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
	// 校验权限码必须存在于权限定义表，防止脏数据进 DB + cache。
	defined, err := s.permRepo.List()
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	valid := make(map[string]struct{}, len(defined))
	for _, p := range defined {
		valid[p.Code] = struct{}{}
	}
	for _, code := range permCodes {
		if strings.TrimSpace(code) == "" {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "permission code cannot be empty")
		}
		if _, ok := valid[code]; !ok {
			return pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("unknown permission code: %s", code))
		}
	}
	if err := s.permRepo.SyncRolePermissions(roleName, permCodes); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.reloadCasbin(); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, fmt.Sprintf("casbin policy reload failed: %v", err))
	}

	// 刷新该角色的缓存
	s.cacheMu.Lock()
	set := make(map[string]struct{}, len(permCodes))
	for _, c := range permCodes {
		set[c] = struct{}{}
	}
	s.cache[roleName] = set
	s.cacheMu.Unlock()
	if s.bus != nil {
		if err := s.bus.PublishInternal(context.Background(), EventPermissionsInvalidated, map[string]string{
			"role": roleName,
		}); err != nil {
			log.Printf("[Permission] publish invalidate: %v", err)
		}
	}
	return nil
}


// RenameRole 角色改名时迁移权限关联并刷新缓存与 Casbin 策略。
func (s *PermissionService) RenameRole(oldName, newName string) error {
	if oldName == newName {
		return nil
	}
	if err := s.permRepo.RenameRolePermissions(oldName, newName); err != nil {
		return err
	}
	if err := s.reloadCasbin(); err != nil {
		return err
	}
	return s.LoadCache()
}

// A load failure keeps the old in-memory policy active instead of failing open.
// reloadCasbin 在 DB 策略变更后重载 Casbin；失败时保留旧内存策略并返回错误，由调用方上抛。
func (s *PermissionService) reloadCasbin() error {
	s.cacheMu.RLock()
	enforcer := s.enforcer
	s.cacheMu.RUnlock()
	if enforcer == nil {
		return nil
	}
	err := enforcer.LoadPolicy()
	return err
}
