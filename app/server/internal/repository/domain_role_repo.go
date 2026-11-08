package repository

import (
	"fmt"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type DomainRoleRepository struct {
	db *gorm.DB
}

func NewDomainRoleRepository(db *gorm.DB) *DomainRoleRepository {
	return &DomainRoleRepository{db: db}
}

func (r *DomainRoleRepository) ListRoles(domainUUID string) ([]model.DomainRole, error) {
	var roles []model.DomainRole
	err := r.db.Where("domain_uuid = ?", domainUUID).Order("id ASC").Find(&roles).Error
	return roles, err
}

func (r *DomainRoleRepository) GetRole(domainUUID, name string) (*model.DomainRole, error) {
	var role model.DomainRole
	err := r.db.Where("domain_uuid = ? AND name = ?", domainUUID, name).First(&role).Error
	return &role, err
}

func (r *DomainRoleRepository) GetRolePermissions(domainUUID, roleName string) ([]string, error) {
	var codes []string
	err := r.db.Model(&model.DomainRolePermission{}).
		Where("domain_uuid = ? AND role_name = ?", domainUUID, roleName).
		Order("permission_code ASC").
		Pluck("permission_code", &codes).Error
	return codes, err
}

func (r *DomainRoleRepository) CreateRoleWithPermissions(role *model.DomainRole, permissions []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(role).Error; err != nil {
			return err
		}
		for _, code := range permissions {
			rp := model.DomainRolePermission{
				DomainUUID:     role.DomainUUID,
				RoleName:       role.Name,
				PermissionCode: code,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *DomainRoleRepository) SyncRolePermissions(domainUUID, roleName string, permissions []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("domain_uuid = ? AND role_name = ?", domainUUID, roleName).
			Delete(&model.DomainRolePermission{}).Error; err != nil {
			return err
		}
		for _, code := range permissions {
			rp := model.DomainRolePermission{
				DomainUUID:     domainUUID,
				RoleName:       roleName,
				PermissionCode: code,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *DomainRoleRepository) DeleteRole(domainUUID, name string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("domain_uuid = ? AND role_name = ?", domainUUID, name).
			Delete(&model.DomainRolePermission{}).Error; err != nil {
			return err
		}
		return tx.Where("domain_uuid = ? AND name = ?", domainUUID, name).
			Delete(&model.DomainRole{}).Error
	})
}

func (r *DomainRoleRepository) RoleInUse(domainUUID, name string) (bool, error) {
	var count int64
	err := r.db.Model(&model.DomainMember{}).
		Where("domain_uuid = ? AND role_name = ?", domainUUID, name).
		Count(&count).Error
	return count > 0, err
}

// SeedDefaultDomainRoles 为域创建系统角色；重复调用幂等。owner 不存权限行。
func SeedDefaultDomainRoles(db *gorm.DB, domainUUID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, name := range []string{
			model.DomainRoleOwner,
			model.DomainRoleAdmin,
			model.DomainRoleMember,
			model.DomainRoleGuest,
		} {
			var count int64
			if err := tx.Model(&model.DomainRole{}).
				Where("domain_uuid = ? AND name = ?", domainUUID, name).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			role := model.DomainRole{DomainUUID: domainUUID, Name: name, IsSystem: true}
			if err := tx.Create(&role).Error; err != nil {
				return err
			}
			for _, code := range model.DefaultDomainRolePermissions[name] {
				rp := model.DomainRolePermission{
					DomainUUID:     domainUUID,
					RoleName:       name,
					PermissionCode: code,
				}
				if err := tx.Create(&rp).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// SeedDefaults 为指定域播种系统角色（owner/admin/member/guest），幂等。
// 复用包级 SeedDefaultDomainRoles 作为单一来源，避免逻辑分叉。
func (r *DomainRoleRepository) SeedDefaults(domainUUID string) error {
	return SeedDefaultDomainRoles(r.db, domainUUID)
}

// BackfillDomainRoleDefaults 为所有存量域补播系统角色，幂等。
// 覆盖域角色体系上线前创建的域与 EnsureDefaultDomain 建出的默认域。
func BackfillDomainRoleDefaults(db *gorm.DB) error {
	var uuids []string
	if err := db.Model(&model.Domain{}).Order("created_at ASC").Pluck("uuid", &uuids).Error; err != nil {
		return err
	}
	for _, uuid := range uuids {
		if err := SeedDefaultDomainRoles(db, uuid); err != nil {
			return fmt.Errorf("seed domain roles for %s: %w", uuid, err)
		}
	}
	return nil
}
