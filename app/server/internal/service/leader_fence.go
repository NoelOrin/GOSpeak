package service

import (
	"errors"
	"fmt"
	"sync"

	"GOSpeak/internal/repository"
)

// ErrLeaderFenceLost 表示当前进程不再持有 DB 写面，调用方必须停止写路径。
var ErrLeaderFenceLost = errors.New("agent leader fence lost")

// LeaderFenceService 将 NATS 主备锁与 DB 写面 fence 绑定：
// NATS 锁负责选主，DB fence 负责在分区场景下阻止旧 leader 继续写。
type LeaderFenceService struct {
	repo     *repository.ClusterFenceRepository
	leaderID string

	mu     sync.RWMutex
	epoch  uint64
	active bool
}

func NewLeaderFenceService(repo *repository.ClusterFenceRepository, leaderID string) *LeaderFenceService {
	return &LeaderFenceService{repo: repo, leaderID: leaderID}
}

// Acquire 在 NATS 锁抢占成功后调用；失败时调用方不得进入写面。
func (s *LeaderFenceService) Acquire() error {
	epoch, err := s.repo.Acquire(s.leaderID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.epoch = epoch
	s.active = true
	s.mu.Unlock()
	return nil
}

// Verify 校验 DB 写面仍由本进程持有。失败时立即停用本地 fence。
func (s *LeaderFenceService) Verify() error {
	s.mu.RLock()
	active := s.active
	epoch := s.epoch
	s.mu.RUnlock()
	if !active {
		return ErrLeaderFenceLost
	}
	ok, err := s.repo.Verify(s.leaderID, epoch)
	if err != nil {
		return fmt.Errorf("verify leader fence: %w", err)
	}
	if !ok {
		s.Deactivate()
		return ErrLeaderFenceLost
	}
	return nil
}

// Deactivate 在锁丢失或优雅退出时停用本地写面。
func (s *LeaderFenceService) Deactivate() {
	s.mu.Lock()
	s.active = false
	s.mu.Unlock()
}

// Active 返回本地写面是否仍处于激活状态。
func (s *LeaderFenceService) Active() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}
