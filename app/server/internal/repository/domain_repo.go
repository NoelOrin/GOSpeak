package repository

import (
	"GOSpeak/internal/model"
	"strings"

	"gorm.io/gorm"
)

type DomainRepository struct {
	db *gorm.DB
}

func NewDomainRepository(db *gorm.DB) *DomainRepository {
	return &DomainRepository{db: db}
}

func (r *DomainRepository) Create(domain *model.Domain) error {
	return r.db.Create(domain).Error
}

func (r *DomainRepository) GetByUUID(uuid string) (*model.Domain, error) {
	var domain model.Domain
	err := r.db.Where("uuid = ?", uuid).First(&domain).Error
	return &domain, err
}

func (r *DomainRepository) GetByInviteCode(code string) (*model.Domain, error) {
	var domain model.Domain
	err := r.db.Where("invite_code = ?", code).First(&domain).Error
	return &domain, err
}

func (r *DomainRepository) List(page, pageSize int) ([]model.Domain, int64, error) {
	var domains []model.Domain
	var total int64
	r.db.Model(&model.Domain{}).Count(&total)
	err := r.db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&domains).Error
	return domains, total, err
}

func (r *DomainRepository) ListPublic(page, pageSize int, keyword string) ([]model.Domain, int64, error) {
	var domains []model.Domain
	var total int64
	q := r.db.Model(&model.Domain{}).Where("is_public = ?", true)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("(name LIKE ? OR description LIKE ?)", like, like)
	}
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&domains).Error
	return domains, total, err
}

func (r *DomainRepository) Update(domain *model.Domain) error {
	return r.db.Save(domain).Error
}

func (r *DomainRepository) Delete(uuid string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("domain_uuid = ?", uuid).Delete(&model.DomainMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("domain_uuid = ?", uuid).Delete(&model.Room{}).Error; err != nil {
			return err
		}
		return tx.Where("uuid = ?", uuid).Delete(&model.Domain{}).Error
	})
}

// --- DomainMember ---

func (r *DomainRepository) AddMember(member *model.DomainMember) error {
	return r.db.Create(member).Error
}

func (r *DomainRepository) UpdateMember(member *model.DomainMember) error {
	return r.db.Save(member).Error
}

func (r *DomainRepository) RemoveMember(domainUUID, userUUID string) error {
	return r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).Delete(&model.DomainMember{}).Error
}

func (r *DomainRepository) GetMember(domainUUID, userUUID string) (*model.DomainMember, error) {
	var member model.DomainMember
	err := r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).First(&member).Error
	return &member, err
}

func (r *DomainRepository) ListMembers(domainUUID string) ([]model.DomainMember, error) {
	var members []model.DomainMember
	err := r.db.Where("domain_uuid = ?", domainUUID).Order("joined_at ASC").Find(&members).Error
	return members, err
}

func (r *DomainRepository) ListUserDomains(userUUID string) ([]string, error) {
	var domainUUIDs []string
	err := r.db.Model(&model.DomainMember{}).Where("user_uuid = ?", userUUID).Pluck("domain_uuid", &domainUUIDs).Error
	return domainUUIDs, err
}

func (r *DomainRepository) CountMembers(domainUUID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.DomainMember{}).Where("domain_uuid = ?", domainUUID).Count(&count).Error
	return count, err
}
