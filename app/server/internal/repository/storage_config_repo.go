package repository

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/storage"

	"gorm.io/gorm"
)

// StorageConfigRepository 存储配置仓库
type StorageConfigRepository struct {
	db *gorm.DB
}

// NewStorageConfigRepository 创建存储配置仓库
func NewStorageConfigRepository(db *gorm.DB) *StorageConfigRepository {
	return &StorageConfigRepository{db: db}
}

// GetConfig 获取存储配置（ID=1 单行配置）
func (r *StorageConfigRepository) GetConfig() (*model.StorageConfig, error) {
	var cfg model.StorageConfig
	err := r.db.First(&cfg, 1).Error
	if err != nil {
		return nil, err
	}
	// 解密 AccessKey 和 SecretKey
	if cfg.AccessKey != "" {
		decrypted, err := storage.DecryptSecret(cfg.AccessKey)
		if err != nil {
			return nil, err
		}
		cfg.AccessKey = decrypted
	}
	if cfg.SecretKey != "" {
		decrypted, err := storage.DecryptSecret(cfg.SecretKey)
		if err != nil {
			return nil, err
		}
		cfg.SecretKey = decrypted
	}
	return &cfg, nil
}

// SaveConfig 保存存储配置（upsert ID=1）
func (r *StorageConfigRepository) SaveConfig(cfg *model.StorageConfig) error {
	cfg.ID = 1
	// 加密 AccessKey 和 SecretKey
	if cfg.AccessKey != "" {
		encrypted, err := storage.EncryptSecret(cfg.AccessKey)
		if err != nil {
			return err
		}
		cfg.AccessKey = encrypted
	}
	if cfg.SecretKey != "" {
		encrypted, err := storage.EncryptSecret(cfg.SecretKey)
		if err != nil {
			return err
		}
		cfg.SecretKey = encrypted
	}
	return r.db.Save(cfg).Error
}
