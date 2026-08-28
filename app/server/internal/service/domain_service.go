package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"
)

var ErrDomainNotFound = pkg.NewAppError(pkg.NOT_FOUND, "domain not found")
var ErrDomainMemberNotFound = pkg.NewAppError(pkg.NOT_FOUND, "domain member not found")
var ErrAlreadyMember = pkg.NewAppError(pkg.ALREADY_EXISTS, "already a member of this domain")
var ErrDomainRoleNotFound = pkg.NewAppError(pkg.NOT_FOUND, "domain role not found")

const (
	DomainRoleOwner  = model.DomainRoleOwner
	DomainRoleAdmin  = model.DomainRoleAdmin
	DomainRoleMember = model.DomainRoleMember
	DomainRoleGuest  = model.DomainRoleGuest

	// EventDomainMembershipChanged 是 Domain 成员变更的 NATS 内部失效事件。
	EventDomainMembershipChanged = "cache:domain-membership-changed"

	// domainMemberCacheTTL 控制 Worker 从读副本/缓存读取成员关系的最大陈旧窗口。
	// 踢人/退出/删除等变更会立即发布 NATS 失效，缓存只是分区降级兜底。
	domainMemberCacheTTL = 30 * time.Second
)

type DomainService struct {
	domainRepo *repository.DomainRepository
	roleRepo   *repository.DomainRoleRepository
	bus        cacheBus

	memberCache   map[string]domainMemberCacheEntry
	memberCacheMu sync.RWMutex

	authEnforcer *casbin.SyncedEnforcer
	useCasbin    bool
}

type domainMemberCacheEntry struct {
	ok      bool
	expires time.Time
}

func NewDomainService(domainRepo *repository.DomainRepository, roleRepo *repository.DomainRoleRepository) *DomainService {
	return &DomainService{
		domainRepo:  domainRepo,
		roleRepo:    roleRepo,
		memberCache: make(map[string]domainMemberCacheEntry),
	}
}

// SetEventBus 注入 NATS 内部事件发布器，用于成员变更后的缓存失效广播。
func (s *DomainService) SetEventBus(b cacheBus) {
	s.bus = b
}

// OnRemoteInvalidate 收到其他实例的成员变更事件后清空本地缓存。
func (s *DomainService) OnRemoteInvalidate(payload interface{}) {
	domainUUID, userUUID := parseInvalidationPayload(payload)
	s.invalidateMembership(domainUUID, userUUID, false)
}

func parseInvalidationPayload(payload interface{}) (string, string) {
	switch p := payload.(type) {
	case map[string]string:
		return p["domain_uuid"], p["user_uuid"]
	case map[string]interface{}:
		domainUUID, _ := p["domain_uuid"].(string)
		userUUID, _ := p["user_uuid"].(string)
		return domainUUID, userUUID
	}
	log.Printf("[Domain] invalid membership invalidation payload type: %T", payload)
	return "", ""
}

func domainCacheKey(domainUUID, userUUID string) string {
	return fmt.Sprintf("%s|%s", domainUUID, userUUID)
}

// invalidateMembership 清理本地成员缓存，并可选广播 NATS 失效事件。
func (s *DomainService) invalidateMembership(domainUUID, userUUID string, publish bool) {
	s.memberCacheMu.Lock()
	if domainUUID != "" && userUUID != "" {
		delete(s.memberCache, domainCacheKey(domainUUID, userUUID))
	} else if domainUUID != "" {
		prefix := domainUUID + "|"
		for key := range s.memberCache {
			if strings.HasPrefix(key, prefix) {
				delete(s.memberCache, key)
			}
		}
	} else {
		s.memberCache = make(map[string]domainMemberCacheEntry)
	}
	s.memberCacheMu.Unlock()

	if publish && s.bus != nil {
		payload := map[string]string{"domain_uuid": domainUUID, "user_uuid": userUUID}
		if err := s.bus.PublishInternal(context.Background(), EventDomainMembershipChanged, payload); err != nil {
			log.Printf("[Domain] publish membership invalidate: %v", err)
		}
	}
}

func (s *DomainService) Create(name, description, ownerUUID string, isPublic bool) (*model.Domain, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain name is required")
	}
	if len(name) > 100 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain name too long")
	}
	domain := &model.Domain{
		Name: name, Description: description, OwnerUUID: ownerUUID, IsPublic: isPublic,
	}
	member := &model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: ownerUUID, RoleName: DomainRoleOwner,
	}
	if err := s.domainRepo.CreateWithOwner(domain, member); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return domain, nil
}

func (s *DomainService) GetByUUID(uuid string) (*model.Domain, error) {
	domain, err := s.domainRepo.GetByUUID(uuid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return domain, nil
}

func (s *DomainService) GetByInviteCode(code string) (*model.Domain, error) {
	domain, err := s.domainRepo.GetByInviteCode(code)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return domain, nil
}

func (s *DomainService) List(page, pageSize int) ([]model.Domain, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.domainRepo.List(page, pageSize)
}

func (s *DomainService) ListPublic(page, pageSize int, keyword string) ([]model.Domain, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.domainRepo.ListPublic(page, pageSize, keyword)
}

func (s *DomainService) Update(domain *model.Domain) error {
	return s.domainRepo.Update(domain)
}

// ResetInviteCode 重置指定域的邀请码，使旧链接失效并返回更新后的域。
func (s *DomainService) ResetInviteCode(domainUUID string) (*model.Domain, error) {
	if _, err := s.domainRepo.GetByUUID(domainUUID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	code := model.GenerateInviteCode()
	domain, err := s.domainRepo.ResetInviteCode(domainUUID, code)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return domain, nil
}

func (s *DomainService) Delete(uuid string) error {
	if _, err := s.domainRepo.GetByUUID(uuid); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.domainRepo.Delete(uuid); err != nil {
		return err
	}
	s.invalidateMembership(uuid, "", true)
	return nil
}

func (s *DomainService) Join(inviteCode, userUUID string) (*model.Domain, error) {
	domain, err := s.domainRepo.GetByInviteCode(inviteCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if existing, err := s.domainRepo.GetMember(domain.UUID, userUUID); err == nil && existing != nil {
		return nil, ErrAlreadyMember
	}
	member := &model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: userUUID, RoleName: DomainRoleMember,
	}
	if err := s.domainRepo.AddMember(member); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.invalidateMembership(domain.UUID, userUUID, true)
	return domain, nil
}

func (s *DomainService) Leave(domainUUID, userUUID string) error {
	domain, err := s.domainRepo.GetByUUID(domainUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if domain.OwnerUUID == userUUID {
		return pkg.NewAppError(pkg.FORBIDDEN, "owner cannot leave, transfer ownership first")
	}
	if err := s.domainRepo.RemoveMember(domainUUID, userUUID); err != nil {
		return err
	}
	s.invalidateMembership(domainUUID, userUUID, true)
	return nil
}

func (s *DomainService) Kick(domainUUID, targetUserUUID string) error {
	member, err := s.domainRepo.GetMember(domainUUID, targetUserUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainMemberNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if member.RoleName == DomainRoleOwner {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot kick domain owner")
	}
	if err := s.domainRepo.RemoveMember(domainUUID, targetUserUUID); err != nil {
		return err
	}
	s.invalidateMembership(domainUUID, targetUserUUID, true)
	return nil
}

func (s *DomainService) ListMembers(domainUUID string) ([]model.DomainMemberInfo, error) {
	if _, err := s.domainRepo.GetByUUID(domainUUID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.domainRepo.ListMembers(domainUUID)
}

func (s *DomainService) ListUserDomains(userUUID string) ([]string, error) {
	return s.domainRepo.ListUserDomains(userUUID)
}

// ListUserDomainDetails 返回用户加入的 Domain 批量详情（含成员数与房间数）。
func (s *DomainService) ListUserDomainDetails(userUUID string) ([]model.DomainDetail, error) {
	return s.domainRepo.ListUserDomainDetails(userUUID)
}

func (s *DomainService) IsMember(domainUUID, userUUID string) bool {
	key := domainCacheKey(domainUUID, userUUID)
	s.memberCacheMu.RLock()
	if entry, ok := s.memberCache[key]; ok && time.Now().Before(entry.expires) {
		ok := entry.ok
		s.memberCacheMu.RUnlock()
		return ok
	}
	s.memberCacheMu.RUnlock()

	_, err := s.domainRepo.GetMember(domainUUID, userUUID)
	isMember := err == nil
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	s.memberCacheMu.Lock()
	s.memberCache[key] = domainMemberCacheEntry{ok: isMember, expires: time.Now().Add(domainMemberCacheTTL)}
	s.memberCacheMu.Unlock()
	return isMember
}

func (s *DomainService) IsOwner(domainUUID, userUUID string) bool {
	domain, err := s.domainRepo.GetByUUID(domainUUID)
	if err != nil {
		return false
	}
	return domain.OwnerUUID == userUUID
}

func (s *DomainService) HasDomainRole(domainUUID, userUUID, minRole string) bool {
	member, err := s.domainRepo.GetMember(domainUUID, userUUID)
	if err != nil {
		return false
	}
	return domainRoleLevel(member.RoleName) >= domainRoleLevel(minRole)
}

func domainRoleLevel(role string) int {
	switch role {
	case DomainRoleOwner:
		return 4
	case DomainRoleAdmin:
		return 3
	case DomainRoleMember:
		return 2
	case DomainRoleGuest:
		return 1
	default:
		return 0
	}
}

func (s *DomainService) TransferOwnership(domainUUID, currentOwnerUUID, newOwnerUUID string) error {
	domain, err := s.domainRepo.GetByUUID(domainUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if domain.OwnerUUID != currentOwnerUUID {
		return pkg.NewAppError(pkg.FORBIDDEN, "only owner can transfer ownership")
	}
	newMember, err := s.domainRepo.GetMember(domainUUID, newOwnerUUID)
	if err != nil {
		return ErrDomainMemberNotFound
	}
	oldMember, err := s.domainRepo.GetMember(domainUUID, currentOwnerUUID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if oldMember != nil {
		oldMember.RoleName = DomainRoleAdmin
	}
	newMember.RoleName = DomainRoleOwner
	domain.OwnerUUID = newOwnerUUID
	if err := s.domainRepo.TransferOwnership(domain, oldMember, newMember); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.invalidateMembership(domain.UUID, "", true)
	return nil
}

func (s *DomainService) HasDomainPermission(domainUUID, userUUID, permCode string) bool {
	if domainUUID == "" || userUUID == "" || permCode == "" {
		return false
	}
	s.memberCacheMu.RLock()
	enforcer := s.authEnforcer
	useCasbin := s.useCasbin
	s.memberCacheMu.RUnlock()
	if useCasbin && !s.reloadDomainCasbin(enforcer) {
		return false
	}
	if useCasbin && enforcer != nil {
		allowed, err := enforcer.Enforce(userUUID, domainUUID, permCode)
		return err == nil && allowed
	}
	member, err := s.domainRepo.GetMember(domainUUID, userUUID)
	if err != nil {
		return false
	}
	if member.RoleName == model.DomainRoleOwner || member.RoleName == model.DomainRoleAdmin {
		_, ok := model.AssignableDomainPermissionsSet()[permCode]
		return ok
	}
	codes, err := s.roleRepo.GetRolePermissions(domainUUID, member.RoleName)
	if err != nil {
		return false
	}
	for _, code := range codes {
		if code == permCode {
			return true
		}
	}
	return false
}

func (s *DomainService) ListDomainRoles(domainUUID string) ([]model.DomainRole, error) {
	return s.roleRepo.ListRoles(domainUUID)
}

func (s *DomainService) GetDomainRolePermissions(domainUUID, roleName string) ([]string, error) {
	if roleName == model.DomainRoleOwner || roleName == model.DomainRoleAdmin {
		return append([]string(nil), model.AssignableDomainPermissions...), nil
	}
	if _, err := s.roleRepo.GetRole(domainUUID, roleName); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDomainRoleNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.roleRepo.GetRolePermissions(domainUUID, roleName)
}

func (s *DomainService) CreateDomainRole(domainUUID, name string, permissions []string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "role name is required")
	}
	if len(name) > 32 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "role name too long")
	}
	if model.IsSystemDomainRole(name) {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot create system role")
	}
	if err := validateDomainPermissions(permissions); err != nil {
		return err
	}
	if _, err := s.roleRepo.GetRole(domainUUID, name); err == nil {
		return pkg.NewAppError(pkg.ALREADY_EXISTS, "domain role already exists")
	} else if err != gorm.ErrRecordNotFound {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	role := &model.DomainRole{DomainUUID: domainUUID, Name: name}
	if err := s.roleRepo.CreateRoleWithPermissions(role, permissions); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *DomainService) UpdateDomainRolePermissions(domainUUID, roleName string, permissions []string) error {
	if roleName == model.DomainRoleOwner || roleName == model.DomainRoleAdmin {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot modify fixed system role permissions")
	}
	if _, err := s.roleRepo.GetRole(domainUUID, roleName); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainRoleNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := validateDomainPermissions(permissions); err != nil {
		return err
	}
	if err := s.roleRepo.SyncRolePermissions(domainUUID, roleName, permissions); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *DomainService) DeleteDomainRole(domainUUID, roleName string) error {
	if model.IsSystemDomainRole(roleName) {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot delete system role")
	}
	if _, err := s.roleRepo.GetRole(domainUUID, roleName); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainRoleNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	inUse, err := s.roleRepo.RoleInUse(domainUUID, roleName)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if inUse {
		return pkg.NewAppError(pkg.ALREADY_EXISTS, "role is assigned to members")
	}
	if err := s.roleRepo.DeleteRole(domainUUID, roleName); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *DomainService) SetMemberRole(domainUUID, operatorUUID, targetUserUUID, roleName string) error {
	if targetUserUUID == operatorUUID {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot change your own role")
	}
	if roleName == model.DomainRoleOwner {
		return pkg.NewAppError(pkg.FORBIDDEN, "owner role cannot be assigned")
	}
	if roleName == model.DomainRoleAdmin && !s.IsOwner(domainUUID, operatorUUID) {
		return pkg.NewAppError(pkg.FORBIDDEN, "only domain owner can assign admin role")
	}
	target, err := s.domainRepo.GetMember(domainUUID, targetUserUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainMemberNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if target.RoleName == model.DomainRoleOwner {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot change owner role")
	}
	if _, err := s.roleRepo.GetRole(domainUUID, roleName); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainRoleNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	target.RoleName = roleName
	if err := s.domainRepo.UpdateMember(target); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *DomainService) MyDomainPermissions(domainUUID, userUUID string) (string, []string, error) {
	member, err := s.domainRepo.GetMember(domainUUID, userUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil, ErrDomainMemberNotFound
		}
		return "", nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	codes, err := s.GetDomainRolePermissions(domainUUID, member.RoleName)
	if err != nil {
		return "", nil, err
	}
	return member.RoleName, codes, nil
}

func validateDomainPermissions(codes []string) error {
	allowed := model.AssignableDomainPermissionsSet()
	for _, code := range codes {
		if _, ok := allowed[code]; !ok {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "invalid domain permission: "+code)
		}
	}
	return nil
}
