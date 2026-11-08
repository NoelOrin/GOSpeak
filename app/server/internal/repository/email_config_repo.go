package repository

import (
	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type EmailConfigRepository struct {
	db *gorm.DB
}

func NewEmailConfigRepository(db *gorm.DB) *EmailConfigRepository {
	return &EmailConfigRepository{db: db}
}

func (r *EmailConfigRepository) Get() (*model.EmailConfig, error) {
	var cfg model.EmailConfig
	err := r.db.Order("id ASC").First(&cfg).Error
	return &cfg, err
}

func (r *EmailConfigRepository) Save(cfg *model.EmailConfig) error {
	if cfg.ID == 0 {
		cfg.ID = 1
	}
	return r.db.Save(cfg).Error
}
