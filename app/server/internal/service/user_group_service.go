package service

import (
	"errors"
	"strings"
	"unicode/utf8"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

const MaxUserGroupNameRunes = 50

var ErrUserGroupNotFound = pkg.NewAppError(pkg.NOT_FOUND, "user group not found")

type UserGroupService struct {
	repo *repository.UserGroupRepository
}

func NewUserGroupService(repo *repository.UserGroupRepository) *UserGroupService {
	return &UserGroupService{repo: repo}
}

func (s *UserGroupService) List(userID uint) ([]model.UserGroup, error) {
	groups, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return groups, nil
}

func (s *UserGroupService) Create(userID uint, groupName string) (*model.UserGroup, error) {
	name, err := validateUserGroupName(groupName)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.FindByName(userID, name)
	if err == nil && existing != nil {
		return nil, pkg.NewAppError(pkg.ALREADY_EXISTS, "user group already exists")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	group := &model.UserGroup{UserID: userID, GroupName: name}
	if err := s.repo.Create(group); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return group, nil
}

func (s *UserGroupService) Rename(id, userID uint, groupName string) error {
	group, err := s.getOwnedGroup(id, userID)
	if err != nil {
		return err
	}
	name, err := validateUserGroupName(groupName)
	if err != nil {
		return err
	}
	if group.GroupName == name {
		return nil
	}
	existing, err := s.repo.FindByName(userID, name)
	if err == nil && existing != nil {
		return pkg.NewAppError(pkg.ALREADY_EXISTS, "user group already exists")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.repo.Rename(id, name); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *UserGroupService) Delete(id, userID uint) error {
	if _, err := s.getOwnedGroup(id, userID); err != nil {
		return err
	}
	if err := s.repo.Delete(id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *UserGroupService) getOwnedGroup(id, userID uint) (*model.UserGroup, error) {
	group, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserGroupNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if group.UserID != userID {
		return nil, ErrUserGroupNotFound
	}
	return group, nil
}

func validateUserGroupName(groupName string) (string, error) {
	name := strings.TrimSpace(groupName)
	if name == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "group name is required")
	}
	if utf8.RuneCountInString(name) > MaxUserGroupNameRunes {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "group name too long")
	}
	return name, nil
}
