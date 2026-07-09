package repository

import (
	"GOSpeak/internal/model"
	"time"

	"gorm.io/gorm"
)

type BotAPIKeyRepository struct {
	db *gorm.DB
}

func NewBotAPIKeyRepository(db *gorm.DB) *BotAPIKeyRepository {
	return &BotAPIKeyRepository{db: db}
}

// Create 持久化 Bot API Key 记录（KeyHash 已哈希）。
func (r *BotAPIKeyRepository) Create(key *model.BotAPIKey) error {
	return r.db.Create(key).Error
}

// GetByKeyHash 按哈希查找未吊销且在有效期内的 key。
func (r *BotAPIKeyRepository) GetByKeyHash(hash string) (*model.BotAPIKey, error) {
	var key model.BotAPIKey
	err := r.db.Where("key_hash = ? AND revoked = ?", hash, false).First(&key).Error
	return &key, err
}

// ListActive 返回所有未吊销的 key，供 Bot Key 校验时逐一比对哈希。
func (r *BotAPIKeyRepository) ListActive() ([]model.BotAPIKey, error) {
	var keys []model.BotAPIKey
	err := r.db.Where("revoked = ?", false).Find(&keys).Error
	return keys, err
}

// List 返回某创建者名下的所有 key（管理员可传空字符串查全部）。
func (r *BotAPIKeyRepository) List(createdBy string) ([]model.BotAPIKey, error) {
	var keys []model.BotAPIKey
	q := r.db.Order("created_at DESC")
	if createdBy != "" {
		q = q.Where("created_by = ?", createdBy)
	}
	err := q.Find(&keys).Error
	return keys, err
}

// GetByUUID 按 UUID 查询（用于吊销/详情）。
func (r *BotAPIKeyRepository) GetByUUID(uuid string) (*model.BotAPIKey, error) {
	var key model.BotAPIKey
	err := r.db.Where("uuid = ?", uuid).First(&key).Error
	return &key, err
}

// Revoke 吊销指定 key（逻辑删除）。
func (r *BotAPIKeyRepository) Revoke(uuid string) error {
	return r.db.Model(&model.BotAPIKey{}).
		Where("uuid = ?", uuid).
		Update("revoked", true).Error
}

// TouchLastUsed 刷新最后使用时间。
func (r *BotAPIKeyRepository) TouchLastUsed(uuid string, t time.Time) error {
	return r.db.Model(&model.BotAPIKey{}).
		Where("uuid = ?", uuid).
		Update("last_used_at", t).Error
}
