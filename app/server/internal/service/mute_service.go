package service

import (
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
)

// MuteChecker 供 middleware 查询禁言状态的接口
type MuteChecker interface {
	IsMuted(userID uint) (bool, *model.Mute, error)
}

type MuteService struct {
	muteRepo *repository.MuteRepository
	userRepo *repository.UserRepository
	// onExpired 在临时禁言过期且 DB 记录被删除后触发完整 unmute 流程。
	onExpired MuteExpiryHandler
	expiryMu  sync.Mutex
	expiring  map[uint]bool
}

// MuteExpiryHandler 处理临时禁言到期后的业务通知（广播 unmute + SFU 恢复）。
type MuteExpiryHandler func(userID uint)

func NewMuteService(muteRepo *repository.MuteRepository, userRepo *repository.UserRepository) *MuteService {
	return &MuteService{muteRepo: muteRepo, userRepo: userRepo}
}

// SetOnExpired 注入临时禁言过期后的完整解禁回调。
func (s *MuteService) SetOnExpired(fn MuteExpiryHandler) {
	s.onExpired = fn
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
		if delErr := s.muteRepo.DeleteByUserID(userID); delErr != nil && !errors.Is(delErr, gorm.ErrRecordNotFound) {
			return false, nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
		}
		// 删除成功后走完整 unmute 流程：广播事件 + 恢复 SFU 媒体。
		s.notifyExpired(userID)
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

// IsMutedBatch 按用户名批量检查生效禁言，返回 identity -> muted 映射。
func (s *MuteService) IsMutedBatch(identities []string) (map[string]bool, error) {
	muted, err := s.muteRepo.IsMutedBatch(identities)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return muted, nil
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
// duration: 禁言秒数（permanent=true 时忽略）
func (s *MuteService) MuteUser(muterID, userID uint, duration int64, permanent bool, reason string) (*model.Mute, error) {
	if !permanent && duration <= 0 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "duration is required for temporary mute")
	}
	if userID == 0 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "user_id is required")
	}
	target, err := s.userRepo.GetByID(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	// 禁止禁言管理员；Bot 可禁言普通用户，但不可抬权到管理面。
	if target.Role == "admin" {
		muter, merr := s.userRepo.GetByID(muterID)
		if merr != nil || muter == nil || muter.Role != "admin" || muter.IsBot {
			return nil, pkg.NewAppError(pkg.FORBIDDEN, "cannot mute admin")
		}
	}

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

// notifyExpired 触发过期后的完整 unmute 回调，并防止并发调用重复触发。
func (s *MuteService) notifyExpired(userID uint) {
	if userID == 0 || s.onExpired == nil {
		return
	}
	s.expiryMu.Lock()
	if s.expiring == nil {
		s.expiring = make(map[uint]bool)
	}
	if s.expiring[userID] {
		s.expiryMu.Unlock()
		return
	}
	s.expiring[userID] = true
	s.expiryMu.Unlock()
	defer func() {
		s.expiryMu.Lock()
		delete(s.expiring, userID)
		s.expiryMu.Unlock()
	}()
	s.onExpired(userID)
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
	expired, err := s.muteRepo.ListExpired()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	// 清理过期记录
	if err := s.muteRepo.DeleteExpired(); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	for _, mute := range expired {
		s.notifyExpired(mute.UserID)
	}
	return s.muteRepo.ListActive()
}
