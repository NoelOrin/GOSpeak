package repository

import (
	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type SFUConfigRepository struct {
	db *gorm.DB
}

func NewSFUConfigRepository(db *gorm.DB) *SFUConfigRepository {
	return &SFUConfigRepository{db: db}
}

// GetByProvider 获取指定 provider 的配置。未找到返回 gorm.ErrRecordNotFound。
func (r *SFUConfigRepository) GetByProvider(provider string) (*model.SFUConfig, error) {
	var cfg model.SFUConfig
	err := r.db.Where("provider = ?", provider).First(&cfg).Error
	return &cfg, err
}

// Save 按 provider 主键 upsert 配置。
func (r *SFUConfigRepository) Save(cfg *model.SFUConfig) error {
	return r.db.Save(cfg).Error
}

// ListProviders 返回所有已配置的 provider 列表。
func (r *SFUConfigRepository) ListProviders() ([]model.SFUConfig, error) {
	var cfgs []model.SFUConfig
	err := r.db.Find(&cfgs).Error
	return cfgs, err
}

// GetActiveProvider 返回当前激活的 provider 名称。默认 "livekit"。
func (r *SFUConfigRepository) GetActiveProvider() (string, error) {
	var ap model.SFUActiveProvider
	err := r.db.First(&ap).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "livekit", nil
		}
		return "", err
	}
	return ap.Provider, nil
}

// SetActiveProvider 设置当前激活的 provider（唯一行 ID=1）。
func (r *SFUConfigRepository) SetActiveProvider(provider string) error {
	return r.db.Where("id = 1").Assign(&model.SFUActiveProvider{Provider: provider}).FirstOrCreate(&model.SFUActiveProvider{ID: 1}).Error
}
