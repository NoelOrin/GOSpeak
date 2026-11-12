package repository

import (
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type DomainInviteRepository struct {
	db *gorm.DB
}

func NewDomainInviteRepository(db *gorm.DB) *DomainInviteRepository {
	return &DomainInviteRepository{db: db}
}

func (r *DomainInviteRepository) Create(invite *model.DomainInvite) error {
	return r.db.Create(invite).Error
}

func (r *DomainInviteRepository) GetByID(id uint) (*model.DomainInvite, error) {
	var invite model.DomainInvite
	err := r.db.Where("id = ?", id).First(&invite).Error
	return &invite, err
}

// ConsumeUse 以条件更新原子消耗一次使用额度。
// 返回 false 表示码已禁用或次数已耗尽（调用方应回滚事务并报错）。
// 该条件更新本身即并发保护，不引入分布式锁。
func (r *DomainInviteRepository) ConsumeUse(id uint) (bool, error) {
	res := r.db.Model(&model.DomainInvite{}).
		Where("id = ?", id).
		Where("disabled_at IS NULL").
		Where("max_uses = 0 OR used_count < max_uses").
		UpdateColumn("used_count", gorm.Expr("used_count + 1"))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// Disable 禁用邀请码，幂等（已禁用时保持原 disabled_at 不变）。
func (r *DomainInviteRepository) Disable(id uint, operatorUUID string) error {
	now := time.Now()
	return r.db.Model(&model.DomainInvite{}).
		Where("id = ? AND disabled_at IS NULL", id).
		Updates(map[string]interface{}{"disabled_at": &now}).Error
}

// UpdateCode 回填签名码（创建时先插行拿自增 ID，再生成依赖 ID 的码）。
func (r *DomainInviteRepository) UpdateCode(id uint, code string) error {
	return r.db.Model(&model.DomainInvite{}).
		Where("id = ?", id).
		UpdateColumn("code", code).Error
}

func (r *DomainInviteRepository) ListByDomain(domainUUID string) ([]model.DomainInvite, error) {
	var list []model.DomainInvite
	err := r.db.Where("domain_uuid = ?", domainUUID).
		Order("created_at DESC").Find(&list).Error
	return list, err
}

func (r *DomainInviteRepository) CreateRedemption(rec *model.DomainInviteRedemption) error {
	return r.db.Create(rec).Error
}

func (r *DomainInviteRepository) ListRedemptions(inviteID uint, page, pageSize int) ([]model.DomainInviteRedemption, int64, error) {
	var list []model.DomainInviteRedemption
	var total int64
	q := r.db.Model(&model.DomainInviteRedemption{}).Where("invite_id = ?", inviteID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}
