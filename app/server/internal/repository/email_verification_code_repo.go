package repository

import (
	"GOSpeak/internal/model"
	"time"

	"gorm.io/gorm"
)

type EmailVerificationCodeRepository struct {
	db *gorm.DB
}

func NewEmailVerificationCodeRepository(db *gorm.DB) *EmailVerificationCodeRepository {
	return &EmailVerificationCodeRepository{db: db}
}

func (r *EmailVerificationCodeRepository) Create(code *model.EmailVerificationCode) error {
	return r.db.Create(code).Error
}

func (r *EmailVerificationCodeRepository) GetLatestByEmailAndScene(email, scene string) (*model.EmailVerificationCode, error) {
	var code model.EmailVerificationCode
	err := r.db.Where("email = ? AND scene = ?", email, scene).Order("created_at DESC").First(&code).Error
	if err != nil {
		return nil, err
	}
	return &code, nil
}

func (r *EmailVerificationCodeRepository) Update(code *model.EmailVerificationCode) error {
	return r.db.Save(code).Error
}

func (r *EmailVerificationCodeRepository) CountRecentByEmail(email string, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&model.EmailVerificationCode{}).Where("email = ? AND created_at >= ?", email, since).Count(&count).Error
	return count, err
}

func (r *EmailVerificationCodeRepository) CountRecentByIP(ip string, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&model.EmailVerificationCode{}).Where("ip_address = ? AND created_at >= ?", ip, since).Count(&count).Error
	return count, err
}
