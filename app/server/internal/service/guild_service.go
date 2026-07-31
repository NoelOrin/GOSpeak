package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"strings"

	"gorm.io/gorm"
)

var ErrGuildNotFound = pkg.NewAppError(pkg.NOT_FOUND, "guild not found")
var ErrGuildMemberNotFound = pkg.NewAppError(pkg.NOT_FOUND, "guild member not found")
var ErrAlreadyMember = pkg.NewAppError(pkg.ALREADY_EXISTS, "already a member of this guild")
var ErrGuildRoomLimit = pkg.NewAppError(pkg.FORBIDDEN, "guild room limit reached")

const (
	GuildRoleOwner  = "owner"
	GuildRoleAdmin  = "admin"
	GuildRoleMember = "member"
	GuildRoleGuest  = "guest"
)

type GuildService struct {
	guildRepo *repository.GuildRepository
}

func NewGuildService(guildRepo *repository.GuildRepository) *GuildService {
	return &GuildService{guildRepo: guildRepo}
}

func (s *GuildService) Create(name, description, ownerUUID string, isPublic bool) (*model.Guild, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "guild name is required")
	}
	if len(name) > 100 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "guild name too long")
	}
	guild := &model.Guild{
		Name: name, Description: description, OwnerUUID: ownerUUID, IsPublic: isPublic,
	}
	if err := s.guildRepo.Create(guild); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	member := &model.GuildMember{
		GuildUUID: guild.UUID, UserUUID: ownerUUID, RoleName: GuildRoleOwner,
	}
	if err := s.guildRepo.AddMember(member); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return guild, nil
}

func (s *GuildService) GetByUUID(uuid string) (*model.Guild, error) {
	guild, err := s.guildRepo.GetByUUID(uuid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrGuildNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return guild, nil
}

func (s *GuildService) GetByInviteCode(code string) (*model.Guild, error) {
	guild, err := s.guildRepo.GetByInviteCode(code)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrGuildNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return guild, nil
}

func (s *GuildService) List(page, pageSize int) ([]model.Guild, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.guildRepo.List(page, pageSize)
}

func (s *GuildService) ListPublic(page, pageSize int, keyword string) ([]model.Guild, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.guildRepo.ListPublic(page, pageSize, keyword)
}

func (s *GuildService) Update(guild *model.Guild) error {
	return s.guildRepo.Update(guild)
}

func (s *GuildService) Delete(uuid string) error {
	if _, err := s.guildRepo.GetByUUID(uuid); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrGuildNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.guildRepo.Delete(uuid)
}

func (s *GuildService) Join(inviteCode, userUUID string) (*model.Guild, error) {
	guild, err := s.guildRepo.GetByInviteCode(inviteCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrGuildNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if existing, err := s.guildRepo.GetMember(guild.UUID, userUUID); err == nil && existing != nil {
		return nil, ErrAlreadyMember
	}
	member := &model.GuildMember{
		GuildUUID: guild.UUID, UserUUID: userUUID, RoleName: GuildRoleMember,
	}
	if err := s.guildRepo.AddMember(member); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return guild, nil
}

func (s *GuildService) Leave(guildUUID, userUUID string) error {
	guild, err := s.guildRepo.GetByUUID(guildUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrGuildNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if guild.OwnerUUID == userUUID {
		return pkg.NewAppError(pkg.FORBIDDEN, "owner cannot leave, transfer ownership first")
	}
	return s.guildRepo.RemoveMember(guildUUID, userUUID)
}

func (s *GuildService) Kick(guildUUID, targetUserUUID string) error {
	member, err := s.guildRepo.GetMember(guildUUID, targetUserUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrGuildMemberNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if member.RoleName == GuildRoleOwner {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot kick guild owner")
	}
	return s.guildRepo.RemoveMember(guildUUID, targetUserUUID)
}

func (s *GuildService) ListMembers(guildUUID string) ([]model.GuildMember, error) {
	if _, err := s.guildRepo.GetByUUID(guildUUID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrGuildNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.guildRepo.ListMembers(guildUUID)
}

func (s *GuildService) ListUserGuilds(userUUID string) ([]string, error) {
	return s.guildRepo.ListUserGuilds(userUUID)
}

func (s *GuildService) CheckRoomLimit(guildUUID string) error {
	guild, err := s.guildRepo.GetByUUID(guildUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrGuildNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if guild.MaxRooms == 0 {
		return nil
	}
	count, err := s.guildRepo.CountRooms(guildUUID)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if count >= int64(guild.MaxRooms) {
		return ErrGuildRoomLimit
	}
	return nil
}

func (s *GuildService) IsMember(guildUUID, userUUID string) bool {
	_, err := s.guildRepo.GetMember(guildUUID, userUUID)
	return err == nil
}

func (s *GuildService) IsOwner(guildUUID, userUUID string) bool {
	guild, err := s.guildRepo.GetByUUID(guildUUID)
	if err != nil {
		return false
	}
	return guild.OwnerUUID == userUUID
}

func (s *GuildService) HasGuildRole(guildUUID, userUUID, minRole string) bool {
	member, err := s.guildRepo.GetMember(guildUUID, userUUID)
	if err != nil {
		return false
	}
	return guildRoleLevel(member.RoleName) >= guildRoleLevel(minRole)
}

func guildRoleLevel(role string) int {
	switch role {
	case GuildRoleOwner:
		return 4
	case GuildRoleAdmin:
		return 3
	case GuildRoleMember:
		return 2
	case GuildRoleGuest:
		return 1
	default:
		return 0
	}
}

func (s *GuildService) TransferOwnership(guildUUID, currentOwnerUUID, newOwnerUUID string) error {
	guild, err := s.guildRepo.GetByUUID(guildUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrGuildNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if guild.OwnerUUID != currentOwnerUUID {
		return pkg.NewAppError(pkg.FORBIDDEN, "only owner can transfer ownership")
	}
	newMember, err := s.guildRepo.GetMember(guildUUID, newOwnerUUID)
	if err != nil {
		return ErrGuildMemberNotFound
	}
	oldMember, err := s.guildRepo.GetMember(guildUUID, currentOwnerUUID)
	if err == nil && oldMember != nil {
		oldMember.RoleName = GuildRoleAdmin
		_ = s.guildRepo.UpdateMember(oldMember)
	}
	newMember.RoleName = GuildRoleOwner
	_ = s.guildRepo.UpdateMember(newMember)
	guild.OwnerUUID = newOwnerUUID
	return s.guildRepo.Update(guild)
}
