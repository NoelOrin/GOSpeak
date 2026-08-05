package repository

import (
	"GOSpeak/internal/model"
	"time"

	"gorm.io/gorm"
)

type BotTokenRepository struct {
	db *gorm.DB
}

func NewBotTokenRepository(db *gorm.DB) *BotTokenRepository {
	return &BotTokenRepository{db: db}
}

func (r *BotTokenRepository) Create(token *model.BotToken) error {
	return r.db.Create(token).Error
}

func (r *BotTokenRepository) List() ([]model.BotToken, error) {
	var tokens []model.BotToken
	err := r.db.Order("created_at DESC").Find(&tokens).Error
	return tokens, err
}

func (r *BotTokenRepository) GetActiveByUserUUID(userUUID string) (*model.BotToken, error) {
	var token model.BotToken
	err := r.db.Where("user_uuid = ? AND revoked = ? AND expires_at > ?", userUUID, false, time.Now()).
		Order("created_at DESC").First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *BotTokenRepository) GetByUUID(uuid string) (*model.BotToken, error) {
	var token model.BotToken
	err := r.db.Where("uuid = ?", uuid).First(&token).Error
	return &token, err
}

func (r *BotTokenRepository) Revoke(uuid string) error {
	return r.db.Model(&model.BotToken{}).
		Where("uuid = ?", uuid).
		Update("revoked", true).Error
}
