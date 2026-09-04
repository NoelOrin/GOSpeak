package repository

import (
	"GOSpeak/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PluginConfigRepository struct {
	db *gorm.DB
}

func NewPluginConfigRepository(db *gorm.DB) *PluginConfigRepository {
	return &PluginConfigRepository{db: db}
}

func (r *PluginConfigRepository) Get(name string) (*model.PluginConfig, error) {
	var row model.PluginConfig
	if err := r.db.Where("name = ?", name).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *PluginConfigRepository) List() ([]model.PluginConfig, error) {
	var rows []model.PluginConfig
	if err := r.db.Order("name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *PluginConfigRepository) Upsert(row *model.PluginConfig) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "config_json", "updated_at"}),
	}).Create(row).Error
}
