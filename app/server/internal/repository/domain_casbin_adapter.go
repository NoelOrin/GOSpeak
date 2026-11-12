package repository

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"

	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"gorm.io/gorm"
)

const platformAdminSubject = "platform:admin"

// DomainCasbinAdapter loads resource authorization policies from existing
// relational tables. Policy reload reads the current database state, so
// membership and role changes take effect without a separate invalidation bus.
type DomainCasbinAdapter struct {
	db *gorm.DB
}

func NewDomainCasbinAdapter(db *gorm.DB) *DomainCasbinAdapter {
	return &DomainCasbinAdapter{db: db}
}

func domainRoleSubject(domainUUID, roleName string) string {
	return "domain:" + domainUUID + ":role:" + roleName
}

func (a *DomainCasbinAdapter) LoadPolicy(m casbinmodel.Model) error {
	var admins []struct {
		UUID string
	}
	if err := a.db.Model(&model.User{}).
		Where("role = ? AND status = ? AND is_bot = ?", "admin", model.UserStatusActive, false).
		Select("uuid").
		Scan(&admins).Error; err != nil {
		return err
	}
	for _, admin := range admins {
		if admin.UUID == "" {
			continue
		}
		if err := persist.LoadPolicyArray([]string{"g", admin.UUID, platformAdminSubject}, m); err != nil {
			return err
		}
	}
	for _, code := range []string{
		permcode.PermRoomCreate,
		permcode.PermRoomRead,
		permcode.PermRoomUpdate,
		permcode.PermRoomDelete,
	} {
		if err := persist.LoadPolicyArray([]string{"p", platformAdminSubject, "*", code}, m); err != nil {
			return err
		}
	}

	var domains []model.Domain
	if err := a.db.Find(&domains).Error; err != nil {
		return err
	}
	domainUUIDs := make([]string, 0, len(domains))
	for _, domain := range domains {
		if domain.UUID == "" || domain.OwnerUUID == "" {
			continue
		}
		domainUUIDs = append(domainUUIDs, domain.UUID)
		if err := persist.LoadPolicyArray([]string{
			"g", domain.OwnerUUID, domainRoleSubject(domain.UUID, model.DomainRoleOwner),
		}, m); err != nil {
			return err
		}
	}

	type memberRow struct {
		DomainUUID string
		UserUUID   string
		RoleName   string
	}
	var members []memberRow
	if err := a.db.Model(&model.DomainMember{}).
		Select("domain_uuid, user_uuid, role_name").
		Scan(&members).Error; err != nil {
		return err
	}
	for _, member := range members {
		if member.DomainUUID == "" || member.UserUUID == "" || member.RoleName == "" {
			continue
		}
		if err := persist.LoadPolicyArray([]string{
			"g", member.UserUUID, domainRoleSubject(member.DomainUUID, member.RoleName),
		}, m); err != nil {
			return err
		}
	}

	for _, code := range model.AssignableDomainPermissions {
		for _, domainUUID := range domainUUIDs {
			for _, roleName := range []string{model.DomainRoleOwner, model.DomainRoleAdmin} {
				if err := persist.LoadPolicyArray([]string{
					"p", domainRoleSubject(domainUUID, roleName), domainUUID, code,
				}, m); err != nil {
					return err
				}
			}
		}
	}

	type permissionRow struct {
		DomainUUID     string
		RoleName       string
		PermissionCode string
	}
	var customPermissions []permissionRow
	if err := a.db.Model(&model.DomainRolePermission{}).
		Select("domain_uuid, role_name, permission_code").
		Where("role_name NOT IN ?", []string{model.DomainRoleOwner, model.DomainRoleAdmin}).
		Scan(&customPermissions).Error; err != nil {
		return err
	}
	for _, item := range customPermissions {
		if item.DomainUUID == "" || item.RoleName == "" || item.PermissionCode == "" {
			continue
		}
		if err := persist.LoadPolicyArray([]string{
			"p", domainRoleSubject(item.DomainUUID, item.RoleName), item.DomainUUID, item.PermissionCode,
		}, m); err != nil {
			return err
		}
	}
	return nil
}

func (a *DomainCasbinAdapter) SavePolicy(m casbinmodel.Model) error {
	return gorm.ErrInvalidData
}

func (a *DomainCasbinAdapter) AddPolicy(sec, ptype string, rule []string) error {
	return gorm.ErrInvalidData
}

func (a *DomainCasbinAdapter) RemovePolicy(sec, ptype string, rule []string) error {
	return gorm.ErrInvalidData
}

func (a *DomainCasbinAdapter) RemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	return gorm.ErrInvalidData
}
