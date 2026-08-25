package repository

import (
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type GuestBanRepo struct{ db *gorm.DB }

func NewGuestBanRepo(db *gorm.DB) *GuestBanRepo { return &GuestBanRepo{db: db} }

func (r *GuestBanRepo) Create(b *model.DomainGuestBan) error { return r.db.Create(b).Error }

func (r *GuestBanRepo) FindActive(domainUUID, userUUID string) *model.DomainGuestBan {
	var b model.DomainGuestBan
	err := r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("id DESC").First(&b).Error
	if err != nil {
		return nil
	}
	return &b
}

func (r *GuestBanRepo) ListByDomain(domainUUID string) ([]model.DomainGuestBan, error) {
	var list []model.DomainGuestBan
	err := r.db.Where("domain_uuid = ?", domainUUID).Order("id DESC").Find(&list).Error
	return list, err
}

// Delete 物理删除 = 解封（记录可后续进审计表，本期不留痕）。
func (r *GuestBanRepo) Delete(domainUUID, userUUID string) error {
	return r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).
		Delete(&model.DomainGuestBan{}).Error
}
