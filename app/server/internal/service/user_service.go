// Package service 业务逻辑层。
// 此文件提供用户相关业务：查询资料、列表、删除、角色变更、头像上传。
package service

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// ErrUserNotFound is returned when a user query finds no matching row.
var ErrUserNotFound = pkg.NewAppError(pkg.NOT_FOUND, "user not found")

// avatarStorage 头像上传所需的最小存储能力。
type avatarStorage interface {
	GetConfig() (*model.StorageConfig, error)
	UploadFromReader(key string, reader io.Reader, size int64, contentType string) (string, error)
}

// UserService 用户服务，支持按 ID / UUID / 用户名查询以及 CRUD 操作。
type UserService struct {
	userRepo *repository.UserRepository
	storage  avatarStorage
}

func NewUserService(userRepo *repository.UserRepository, storage ...avatarStorage) *UserService {
	svc := &UserService{userRepo: userRepo}
	if len(storage) > 0 {
		svc.storage = storage[0]
	}
	return svc
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
func (s *UserService) List(page, pageSize int, excludeBots bool, keyword string) ([]model.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	users, total, err := s.userRepo.List(page, pageSize, excludeBots, keyword)
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.userRepo.Delete(id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// UpdateRole 变更用户角色（含 ban），并递增 TokenVersion 使旧 token 立即失效。
func (s *UserService) UpdateRole(id uint, role string) error {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if !model.IsValidRole(role) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "invalid role")
	}
	if user.Role == role {
		return nil
	}

	status := model.UserStatusActive
	if role == "ban" {
		status = model.UserStatusBanned
	}
	if err := s.userRepo.UpdateRoleStatusAndInvalidate(user.ID, role, status); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// UpdateProfile 更新当前用户的显示名称和头像。
func (s *UserService) UpdateProfile(uuid, displayName, avatar string) (*model.User, error) {
	user, err := s.userRepo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

// UploadAvatar 校验并上传头像，成功后写回 users.avatar。
func (s *UserService) UploadAvatar(uuid, filename, contentType string, size int64, reader io.Reader) (string, *model.User, error) {
	if s.storage == nil {
		return "", nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "storage service is not configured")
	}
	if reader == nil {
		return "", nil, pkg.NewAppError(pkg.INVALID_PARAMS, "avatar file is required")
	}

	cfg, err := s.storage.GetConfig()
	if err != nil {
		return "", nil, err
	}

	maxMB := cfg.MaxFileSize
	if maxMB <= 0 {
		maxMB = 5
	}
	maxBytes := int64(maxMB) * 1024 * 1024
	if size > maxBytes {
		return "", nil, pkg.NewAppError(pkg.STORAGE_FILE_TOO_LARGE)
	}

	contentType = strings.TrimSpace(strings.ToLower(contentType))
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	if !allowedTypes[contentType] {
		return "", nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid image type, allowed: jpeg, png, gif, webp")
	}

	ext := filepath.Ext(filename)
	if ext == "" {
		switch contentType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		default:
			ext = ".bin"
		}
	}
	objectKey := fmt.Sprintf("avatars/%s_%d%s", uuid, time.Now().UnixMilli(), ext)

	avatarURL, err := s.storage.UploadFromReader(objectKey, reader, size, contentType)
	if err != nil {
		return "", nil, err
	}

	user, err := s.UpdateProfile(uuid, "", avatarURL)
	if err != nil {
		return "", nil, err
	}
	return avatarURL, user, nil
}
