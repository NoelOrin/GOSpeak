package repository

import (
	"errors"
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type GuestBanRepo struct {
	db *gorm.DB
}

func NewGuestBanRepo(db *gorm.DB) *GuestBanRepo {
	return &GuestBanRepo{db: db}
}

func (r *GuestBanRepo) Create(b *model.DomainGuestBan) error { return r.db.Create(b).Error }

// FindActive 只把 RecordNotFound 视为未封禁；数据库异常必须上抛，调用方 fail-closed。
func (r *GuestBanRepo) FindActive(domainUUID, userUUID string) (*model.DomainGuestBan, error) {
	var b model.DomainGuestBan
	err := r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("id DESC").First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *GuestBanRepo) ListByDomain(domainUUID string) ([]model.DomainGuestBan, error) {
	var list []model.DomainGuestBan
	err := r.db.Where("domain_uuid = ?", domainUUID).Order("id DESC").Find(&list).Error
	return list, err
}

func (r *GuestBanRepo) Delete(domainUUID, userUUID string) error {
	return r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).
		Delete(&model.DomainGuestBan{}).Error
}
