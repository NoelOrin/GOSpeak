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

func (r *SFUConfigRepository) Get() (*model.SFUConfig, error) {
	var cfg model.SFUConfig
	err := r.db.Order("id ASC").First(&cfg).Error
	return &cfg, err
}

func (r *SFUConfigRepository) Save(cfg *model.SFUConfig) error {
	if cfg.ID == 0 {
		cfg.ID = 1
	}
	return r.db.Save(cfg).Error
}
