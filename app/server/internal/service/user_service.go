// Package service 业务逻辑层。
// 此文件提供用户相关业务：查询资料、列表、删除、角色变更。
package service

import (
	"errors"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// ErrUserNotFound is returned when a user query finds no matching row.
var ErrUserNotFound = pkg.NewAppError(pkg.NOT_FOUND, "user not found")

// UserService 用户服务，支持按 ID / UUID / 用户名查询以及 CRUD 操作。
type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

// GetByID 按主键 ID 查询用户，不存在返回 NOT_FOUND。
func (s *UserService) GetByID(id uint) (*model.User, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return user, nil
}

// GetByUUID 按 UUID 查询用户，handler 层通常用此方法获取当前登录用户。
func (s *UserService) GetByUUID(uuid string) (*model.User, error) {
	user, err := s.userRepo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return user, nil
}

// GetByName 按用户名查询，用于注册重名检测和登录验证。
func (s *UserService) GetByName(name string) (*model.User, error) {
	user, err := s.userRepo.GetByName(name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return user, nil
}

// List 分页查询用户列表，page 从 1 开始，pageSize 最大 100。
func (s *UserService) List(page, pageSize int, excludeBots bool) ([]model.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	users, total, err := s.userRepo.List(page, pageSize, excludeBots)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return users, total, nil
}

// Update 更新用户信息，直接透传 repository 层结果。
func (s *UserService) Update(user *model.User) error {
	if err := s.userRepo.Update(user); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// Delete 删除用户。
func (s *UserService) Delete(id uint) error {
	if _, err := s.userRepo.GetByID(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrUserNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.userRepo.Delete(id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// UpdateRole 变更用户角色，仅允许 admin 和 user 两种取值。
func (s *UserService) UpdateRole(id uint, role string) error {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrUserNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if !model.IsValidRole(role) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "invalid role")
	}

	user.Role = role
	if err := s.userRepo.Update(user); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// UpdateProfile 更新当前用户的显示名称和头像。
func (s *UserService) UpdateProfile(uuid, displayName, avatar string) (*model.User, error) {
	user, err := s.userRepo.GetByUUID(uuid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrUserNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if displayName != "" {
		user.DisplayName = displayName
	}
	if avatar != "" {
		user.Avatar = avatar
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return user, nil
}
