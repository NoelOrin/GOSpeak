package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"time"

	"gorm.io/gorm"
)

// MuteChecker 供 middleware 查询禁言状态的接口
type MuteChecker interface {
	IsMuted(userID uint) (bool, *model.Mute, error)
}

type MuteService struct {
	muteRepo *repository.MuteRepository
	userRepo *repository.UserRepository
}

func NewMuteService(muteRepo *repository.MuteRepository, userRepo *repository.UserRepository) *MuteService {
	return &MuteService{muteRepo: muteRepo, userRepo: userRepo}
}

// IsMuted 检查用户是否被禁言
// 返回: (是否禁言, 禁言记录, 错误)
// 自动处理过期记录删除
func (s *MuteService) IsMuted(userID uint) (bool, *model.Mute, error) {
	mute, err := s.muteRepo.GetByUserID(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil, nil
		}
		return false, nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}

	// 永久禁言
	if mute.Permanent {
		return true, mute, nil
	}

	// 临时禁言：检查过期
	if mute.ExpiresAt != nil && mute.ExpiresAt.Before(time.Now()) {
		if delErr := s.muteRepo.DeleteByUserID(userID); delErr != nil {
			return false, nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
		}
		return false, nil, nil
	}

	return true, mute, nil
}

// IsMutedByIdentity 通过 identity（用户名）检查禁言状态
func (s *MuteService) IsMutedByIdentity(identity string) (bool, *model.Mute, error) {
	user, err := s.userRepo.GetByName(identity)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil, nil
		}
		return false, nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	return s.IsMuted(user.ID)
}

// IsMutedByUUID 通过 UUID 检查禁言状态（供 middleware 使用）
func (s *MuteService) IsMutedByUUID(uuid string) (bool, error) {
	user, err := s.userRepo.GetByUUID(uuid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	muted, _, err := s.IsMuted(user.ID)
	return muted, err
}

// MuteUser 禁言用户
// duration: 禁言秒数（0 = 永久）
func (s *MuteService) MuteUser(muterID, userID uint, duration int64, permanent bool, reason string) (*model.Mute, error) {
	mute := &model.Mute{
		UserID:    userID,
		MuterID:   muterID,
		Duration:  duration,
		Permanent: permanent,
		Reason:    reason,
	}

	if !permanent && duration > 0 {
		expiresAt := time.Now().Add(time.Duration(duration) * time.Second)
		mute.ExpiresAt = &expiresAt
		mute.Duration = duration
	}

	if err := s.muteRepo.Upsert(mute); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to create mute record")
	}

	return mute, nil
}

// UnmuteUser 取消禁言
func (s *MuteService) UnmuteUser(userID uint) error {
	if err := s.muteRepo.DeleteByUserID(userID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.NewAppError(pkg.NOT_FOUND, "mute record not found")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	return nil
}

// GetMuteStatus 查询禁言状态
func (s *MuteService) GetMuteStatus(userID uint) (*model.Mute, error) {
	muted, mute, err := s.IsMuted(userID)
	if err != nil {
		return nil, err
	}
	if !muted {
		return nil, nil
	}
	return mute, nil
}

// ListActiveMutes 获取所有生效禁言
func (s *MuteService) ListActiveMutes() ([]model.Mute, error) {
	// 清理过期记录
	if err := s.muteRepo.DeleteExpired(); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	return s.muteRepo.ListActive()
}
