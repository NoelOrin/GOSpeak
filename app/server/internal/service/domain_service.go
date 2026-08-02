package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"strings"

	"gorm.io/gorm"
)

var ErrDomainNotFound = pkg.NewAppError(pkg.NOT_FOUND, "domain not found")
var ErrDomainMemberNotFound = pkg.NewAppError(pkg.NOT_FOUND, "domain member not found")
var ErrAlreadyMember = pkg.NewAppError(pkg.ALREADY_EXISTS, "already a member of this domain")

const (
	DomainRoleOwner  = "owner"
	DomainRoleAdmin  = "admin"
	DomainRoleMember = "member"
	DomainRoleGuest  = "guest"
)

type DomainService struct {
	domainRepo *repository.DomainRepository
}

func NewDomainService(domainRepo *repository.DomainRepository) *DomainService {
	return &DomainService{domainRepo: domainRepo}
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
	if err := s.domainRepo.Create(domain); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	member := &model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: ownerUUID, RoleName: DomainRoleOwner,
	}
	if err := s.domainRepo.AddMember(member); err != nil {
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

func (s *DomainService) Delete(uuid string) error {
	if _, err := s.domainRepo.GetByUUID(uuid); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.domainRepo.Delete(uuid)
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
	return s.domainRepo.RemoveMember(domainUUID, userUUID)
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
	return s.domainRepo.RemoveMember(domainUUID, targetUserUUID)
}

func (s *DomainService) ListMembers(domainUUID string) ([]model.DomainMember, error) {
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

func (s *DomainService) IsMember(domainUUID, userUUID string) bool {
	_, err := s.domainRepo.GetMember(domainUUID, userUUID)
	return err == nil
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
	if err == nil && oldMember != nil {
		oldMember.RoleName = DomainRoleAdmin
		_ = s.domainRepo.UpdateMember(oldMember)
	}
	newMember.RoleName = DomainRoleOwner
	_ = s.domainRepo.UpdateMember(newMember)
	domain.OwnerUUID = newOwnerUUID
	return s.domainRepo.Update(domain)
}
