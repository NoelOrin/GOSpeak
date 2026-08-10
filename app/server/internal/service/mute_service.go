package service

import (
	"context"
	"errors"
	"log"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
)

const (
	// EventMuteChanged 是禁言状态变更的 NATS 内部失效事件。
	EventMuteChanged = "cache:mute-changed"

	// muteCacheTTL 控制 Worker 禁言判定的最大缓存窗口；禁言/解禁会主动失效。
	muteCacheTTL = 5 * time.Second
)

type muteCacheEntry struct {
	muted    bool
	mute     *model.Mute
	identity string
	expires  time.Time
}

// MuteChecker 供 middleware 查询禁言状态的接口
type MuteChecker interface {
	IsMuted(userID uint) (bool, *model.Mute, error)
}

type MuteService struct {
	muteRepo *repository.MuteRepository
	userRepo *repository.UserRepository
	bus      cacheBus
	// onExpired 在临时禁言过期且 DB 记录被删除后触发完整 unmute 流程。
	onExpired MuteExpiryHandler
	expiryMu  sync.Mutex
	expiring  map[uint]bool

	byIdentity map[string]muteCacheEntry
	byUserID   map[uint]muteCacheEntry
	cacheMu    sync.RWMutex
}

// MuteExpiryHandler 处理临时禁言到期后的业务通知（广播 unmute + SFU 恢复）。
type MuteExpiryHandler func(userID uint)

func NewMuteService(muteRepo *repository.MuteRepository, userRepo *repository.UserRepository) *MuteService {
	return &MuteService{
		muteRepo:   muteRepo,
		userRepo:   userRepo,
		byIdentity: make(map[string]muteCacheEntry),
		byUserID:   make(map[uint]muteCacheEntry),
	}
}

// SetEventBus 注入 NATS 内部事件发布器，用于禁言状态变更后的缓存失效广播。
func (s *MuteService) SetEventBus(b cacheBus) {
	s.bus = b
}

// OnRemoteInvalidate 收到其他实例的禁言变更事件后清空本地缓存。
func (s *MuteService) OnRemoteInvalidate(payload interface{}) {
	var userID uint
	var identity string
	switch p := payload.(type) {
	case map[string]string:
		userID = parseUserID(p["user_id"])
		identity = p["identity"]
	case map[string]interface{}:
		switch v := p["user_id"].(type) {
		case string:
			userID = parseUserID(v)
		case float64:
			userID = uint(v)
		}
		identity, _ = p["identity"].(string)
	}
	s.invalidateMute(userID, identity, false)
}

func parseUserID(raw string) uint {
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return uint(n)
}

// invalidateMute 清理本地禁言缓存，并可选广播 NATS 失效事件。
func (s *MuteService) invalidateMute(userID uint, identity string, publish bool) {
	s.cacheMu.Lock()
	if userID > 0 {
		if entry, ok := s.byUserID[userID]; ok && identity == "" {
			identity = entry.identity
		}
		delete(s.byUserID, userID)
	}
	if identity != "" {
		delete(s.byIdentity, identity)
	}
	s.cacheMu.Unlock()

	if publish && s.bus != nil {
		payload := map[string]string{
			"user_id":  strconv.FormatUint(uint64(userID), 10),
			"identity": identity,
		}
		if err := s.bus.PublishInternal(context.Background(), EventMuteChanged, payload); err != nil {
			log.Printf("[Mute] publish invalidate: %v", err)
		}
	}
}

// SetOnExpired 注入临时禁言过期后的完整解禁回调。
func (s *MuteService) SetOnExpired(fn MuteExpiryHandler) {
	s.onExpired = fn
}

// IsMuted 检查用户是否被禁言
// 返回: (是否禁言, 禁言记录, 错误)
// 自动处理过期记录删除
func (s *MuteService) IsMuted(userID uint) (bool, *model.Mute, error) {
	s.cacheMu.RLock()
	if entry, ok := s.byUserID[userID]; ok && time.Now().Before(entry.expires) {
		muted, mute := entry.muted, entry.mute
		s.cacheMu.RUnlock()
		return muted, mute, nil
	}
	s.cacheMu.RUnlock()

	mute, err := s.muteRepo.GetByUserID(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			s.cacheMute(userID, "", false, nil)
			return false, nil, nil
		}
		return false, nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}

	// 永久禁言
	if mute.Permanent {
		s.cacheMute(userID, "", true, mute)
		return true, mute, nil
	}

	// 临时禁言：检查过期
	if mute.ExpiresAt != nil && mute.ExpiresAt.Before(time.Now()) {
		if delErr := s.muteRepo.DeleteByUserID(userID); delErr != nil && !errors.Is(delErr, gorm.ErrRecordNotFound) {
			return false, nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
		}
		// 删除成功后走完整 unmute 流程：广播事件 + 恢复 SFU 媒体。
		s.invalidateMute(userID, "", true)
		s.notifyExpired(userID)
		s.cacheMute(userID, "", false, nil)
		return false, nil, nil
	}

	s.cacheMute(userID, "", true, mute)
	return true, mute, nil
}

// IsMutedByIdentity 通过 identity（用户名）检查禁言状态
func (s *MuteService) IsMutedByIdentity(identity string) (bool, *model.Mute, error) {
	s.cacheMu.RLock()
	if entry, ok := s.byIdentity[identity]; ok && time.Now().Before(entry.expires) {
		// 批量查询缓存只有 bool 结果，无 mute 详情；命中禁言时回源补详情。
		if !entry.muted || entry.mute != nil {
			muted, mute := entry.muted, entry.mute
			s.cacheMu.RUnlock()
			return muted, mute, nil
		}
	}
	s.cacheMu.RUnlock()

	user, err := s.userRepo.GetByName(identity)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			s.cacheMute(0, identity, false, nil)
			return false, nil, nil
		}
		return false, nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	muted, mute, err := s.IsMuted(user.ID)
	if err != nil {
		return false, nil, err
	}
	s.cacheMute(user.ID, identity, muted, mute)
	return muted, mute, nil
}

// IsMutedBatch 按用户名批量检查生效禁言，返回 identity -> muted 映射。
func (s *MuteService) IsMutedBatch(identities []string) (map[string]bool, error) {
	result := make(map[string]bool, len(identities))
	var missing []string
	now := time.Now()
	s.cacheMu.RLock()
	for _, id := range identities {
		if entry, ok := s.byIdentity[id]; ok && now.Before(entry.expires) {
			result[id] = entry.muted
			continue
		}
		missing = append(missing, id)
	}
	s.cacheMu.RUnlock()
	if len(missing) == 0 {
		return result, nil
	}

	muted, err := s.muteRepo.IsMutedBatch(missing)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.cacheMu.Lock()
	for _, id := range missing {
		isMuted := muted[id]
		result[id] = isMuted
		s.byIdentity[id] = muteCacheEntry{
			muted:    isMuted,
			identity: id,
			expires:  time.Now().Add(muteCacheTTL),
		}
	}
	s.cacheMu.Unlock()
	return result, nil
}

func (s *MuteService) cacheMute(userID uint, identity string, muted bool, mute *model.Mute) {
	entry := muteCacheEntry{
		muted:    muted,
		mute:     mute,
		identity: identity,
		expires:  time.Now().Add(muteCacheTTL),
	}
	s.cacheMu.Lock()
	if userID > 0 {
		s.byUserID[userID] = entry
	}
	if identity != "" {
		s.byIdentity[identity] = entry
	}
	s.cacheMu.Unlock()
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

	s.invalidateMute(userID, target.Name, true)
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
	identity := ""
	s.cacheMu.RLock()
	if entry, ok := s.byUserID[userID]; ok {
		identity = entry.identity
	}
	s.cacheMu.RUnlock()
	if identity == "" {
		if user, userErr := s.userRepo.GetByID(userID); userErr == nil && user != nil {
			identity = user.Name
		}
	}
	s.invalidateMute(userID, identity, true)
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
