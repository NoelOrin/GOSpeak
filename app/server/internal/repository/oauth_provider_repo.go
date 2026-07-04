package repository

import (
	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type OAuthProviderRepository struct {
	db *gorm.DB
}

func NewOAuthProviderRepository(db *gorm.DB) *OAuthProviderRepository {
	return &OAuthProviderRepository{db: db}
}

func (r *OAuthProviderRepository) GetByName(name string) (*model.OAuthProvider, error) {
	var provider model.OAuthProvider
	err := r.db.Where("name = ?", name).First(&provider).Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *OAuthProviderRepository) List() ([]model.OAuthProvider, error) {
	var providers []model.OAuthProvider
	err := r.db.Find(&providers).Error
	return providers, err
}

func (r *OAuthProviderRepository) Create(provider *model.OAuthProvider) error {
	return r.db.Create(provider).Error
}

func (r *OAuthProviderRepository) Update(provider *model.OAuthProvider) error {
	return r.db.Save(provider).Error
}

func (r *OAuthProviderRepository) Delete(id uint) error {
	return r.db.Delete(&model.OAuthProvider{}, id).Error
}
