package repository

import (
	"go_rtc/internal/model"

	"gorm.io/gorm"
)

type OAuthAccountRepository struct {
	db *gorm.DB
}

func NewOAuthAccountRepository(db *gorm.DB) *OAuthAccountRepository {
	return &OAuthAccountRepository{db: db}
}

func (r *OAuthAccountRepository) GetByProviderAndUID(provider, uid string) (*model.OAuthAccount, error) {
	var account model.OAuthAccount
	err := r.db.Where("provider = ? AND provider_uid = ?", provider, uid).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *OAuthAccountRepository) GetByUserID(userID uint) ([]model.OAuthAccount, error) {
	var accounts []model.OAuthAccount
	err := r.db.Where("user_id = ?", userID).Find(&accounts).Error
	return accounts, err
}

func (r *OAuthAccountRepository) Create(account *model.OAuthAccount) error {
	return r.db.Create(account).Error
}

func (r *OAuthAccountRepository) Update(account *model.OAuthAccount) error {
	return r.db.Save(account).Error
}
