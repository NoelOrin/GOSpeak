package repository

import (
	"errors"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

// ClusterFenceRepository 提供 Agent 主备写面 fence 的持久化读写。
type ClusterFenceRepository struct {
	db *gorm.DB
}

func NewClusterFenceRepository(db *gorm.DB) *ClusterFenceRepository {
	return &ClusterFenceRepository{db: db}
}

// Acquire 抢占 DB 写面：单行记录，每次抢占递增 Epoch。
// 旧 leader 的 Verify 会因 LeaderID/Epoch 不匹配而失败。
func (r *ClusterFenceRepository) Acquire(leaderID string) (uint64, error) {
	const fenceID = 1
	var epoch uint64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var fence model.ClusterLeaderFence
		err := tx.First(&fence, fenceID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fence = model.ClusterLeaderFence{ID: fenceID, LeaderID: leaderID, Epoch: 1}
			if err := tx.Create(&fence).Error; err != nil {
				return err
			}
			epoch = fence.Epoch
			return nil
		}
		if err != nil {
			return err
		}
		fence.LeaderID = leaderID
		fence.Epoch++
		if err := tx.Save(&fence).Error; err != nil {
			return err
		}
		epoch = fence.Epoch
		return nil
	})
	return epoch, err
}

// Verify 校验当前进程仍持有 DB 写面。
func (r *ClusterFenceRepository) Verify(leaderID string, epoch uint64) (bool, error) {
	const fenceID = 1
	var fence model.ClusterLeaderFence
	err := r.db.First(&fence, fenceID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return fence.LeaderID == leaderID && fence.Epoch == epoch, nil
}
