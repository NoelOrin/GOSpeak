package repository

import (
	"GOSpeak/internal/model"
	"time"

	"gorm.io/gorm"
)

type MuteRepository struct {
	db *gorm.DB
}

func NewMuteRepository(db *gorm.DB) *MuteRepository {
	return &MuteRepository{db: db}
}

// Create 创建禁言记录（新记录覆盖旧记录：unique index on user_id）
func (r *MuteRepository) Create(mute *model.Mute) error {
	return r.db.Create(mute).Error
}

// GetByUserID 查询用户当前禁言记录
func (r *MuteRepository) GetByUserID(userID uint) (*model.Mute, error) {
	var mute model.Mute
	err := r.db.Where("user_id = ?", userID).First(&mute).Error
	return &mute, err
}

// DeleteByUserID 删除用户禁言记录
func (r *MuteRepository) DeleteByUserID(userID uint) error {
	result := r.db.Where("user_id = ?", userID).Delete(&model.Mute{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteExpired 删除所有已过期禁言记录（expires_at <= now 且非永久禁言）
func (r *MuteRepository) DeleteExpired() error {
	return r.db.Where("permanent = ? AND expires_at IS NOT NULL AND expires_at <= ?", false, time.Now()).Delete(&model.Mute{}).Error
}

// ListActive 查询所有生效禁言记录（未过期 + 永久禁言）
func (r *MuteRepository) ListActive() ([]model.Mute, error) {
	var mutes []model.Mute
	err := r.db.Where(
		"permanent = ? OR (expires_at IS NOT NULL AND expires_at > ?)",
		true, time.Now(),
	).Find(&mutes).Error
	return mutes, err
}

// Upsert 覆盖插入：先删后插（事务），保证单记录覆盖
func (r *MuteRepository) Upsert(mute *model.Mute) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", mute.UserID).Delete(&model.Mute{}).Error; err != nil {
			return err
		}
		return tx.Create(mute).Error
	})
}
