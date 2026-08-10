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

// CreateWithOwner 在同一事务内创建 Domain 与 owner 成员，避免成员写入失败留下孤儿域。
func (r *DomainRepository) CreateWithOwner(domain *model.Domain, owner *model.DomainMember) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(domain).Error; err != nil {
			return err
		}
		if owner != nil && owner.DomainUUID == "" {
			owner.DomainUUID = domain.UUID
		}
		if err := tx.Create(owner).Error; err != nil {
			return err
		}
		return SeedDefaultDomainRoles(tx, domain.UUID)
	})
}

// TransferOwnership 原子转移 Domain 归属：旧 owner 降级为 admin、新 owner 升为 owner、域记录同步更新。
func (r *DomainRepository) TransferOwnership(domain *model.Domain, oldMember, newMember *model.DomainMember) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if oldMember != nil {
			if err := tx.Save(oldMember).Error; err != nil {
				return err
			}
		}
		if err := tx.Save(newMember).Error; err != nil {
			return err
		}
		return tx.Save(domain).Error
	})
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

func (r *DomainRepository) ListMembers(domainUUID string) ([]model.DomainMemberInfo, error) {
	var members []model.DomainMemberInfo
	err := r.db.Raw(`
		SELECT dm.id, dm.domain_uuid, dm.user_uuid, dm.nickname, dm.role_name, dm.joined_at,
	       u.name, u.display_name
	FROM domain_members dm
	LEFT JOIN users u ON u.uuid = dm.user_uuid
	WHERE dm.domain_uuid = ?
	ORDER BY dm.joined_at ASC
	`, domainUUID).Scan(&members).Error
	return members, err
}

func (r *DomainRepository) ListUserDomains(userUUID string) ([]string, error) {
	var domainUUIDs []string
	err := r.db.Model(&model.DomainMember{}).Where("user_uuid = ?", userUUID).Pluck("domain_uuid", &domainUUIDs).Error
	return domainUUIDs, err
}

// ListUserDomainDetails 返回用户加入的 Domain 批量详情（含成员数与房间数），单次查询完成。
func (r *DomainRepository) ListUserDomainDetails(userUUID string) ([]model.DomainDetail, error) {
	details := make([]model.DomainDetail, 0)
	err := r.db.Raw(`
		SELECT d.id, d.uuid, d.name, d.icon_url, d.description, d.owner_uuid, d.invite_code, d.is_public, d.created_at, d.updated_at,
		       (SELECT COUNT(*) FROM domain_members m2 WHERE m2.domain_uuid = d.uuid) AS member_count,
		       (SELECT COUNT(*) FROM room r2 WHERE r2.domain_uuid = d.uuid) AS room_count
		FROM domains d
		WHERE EXISTS (SELECT 1 FROM domain_members m WHERE m.domain_uuid = d.uuid AND m.user_uuid = ?)
		ORDER BY d.created_at DESC
	`, userUUID).Scan(&details).Error
	return details, err
}

func (r *DomainRepository) CountMembers(domainUUID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.DomainMember{}).Where("domain_uuid = ?", domainUUID).Count(&count).Error
	return count, err
}
