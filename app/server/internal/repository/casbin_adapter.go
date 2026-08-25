// Package repository 提供数据访问实现。
package repository

import (
	"errors"
	"strings"

	"GOSpeak/internal/model"

	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"gorm.io/gorm"
)

// CasbinAdapter 将 Casbin 策略存储在现有 role_permissions 表中。
// p 规则使用 role, permission 两列；g 规则使用 child_role, parent_role 两列。
type CasbinAdapter struct {
	db *gorm.DB
}

func NewCasbinAdapter(db *gorm.DB) *CasbinAdapter {
	return &CasbinAdapter{db: db}
}

func (a *CasbinAdapter) LoadPolicy(m casbinmodel.Model) error {
	// Casbin 的 act 维度是权限码（如 domain:create），而 role_permissions 表存的是
	// permission_id 外键。这里 join permissions 取 code，保证 Enforce(role, code)
	// 能命中策略，与写入路径（按 code 解析为 id）保持一致。
	type row struct {
		RoleName string
		Code     string
	}
	var rules []row
	if err := a.db.
		Table("role_permissions").
		Select("role_permissions.role_name AS role_name, permissions.code AS code").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Scan(&rules).Error; err != nil {
		return err
	}

	// 自反 g 边：matcher 使用 g(r.sub, p.sub)，casbin 的 g 默认不自反，
	// 必须为每个出现过的角色加载 g, role, role，否则 g(role, role) 恒为 false。
	roles := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		if rule.RoleName == "" || rule.Code == "" {
			continue
		}
		if err := persist.LoadPolicyArray([]string{"p", rule.RoleName, rule.Code}, m); err != nil {
			return err
		}
		roles[rule.RoleName] = struct{}{}
	}
	for role := range roles {
		if err := persist.LoadPolicyArray([]string{"g", role, role}, m); err != nil {
			return err
		}
	}
	return nil
}

func (a *CasbinAdapter) SavePolicy(m casbinmodel.Model) error {
	return errors.New("casbin: full policy save is managed by permission service")
}

func (a *CasbinAdapter) AddPolicy(sec, ptype string, rule []string) error {
	if sec != "p" || ptype != "p" || len(rule) != 2 {
		return errors.New("casbin: unsupported policy rule")
	}
	return a.addRolePermission(rule[0], rule[1])
}

func (a *CasbinAdapter) RemovePolicy(sec, ptype string, rule []string) error {
	if sec != "p" || ptype != "p" || len(rule) != 2 {
		return errors.New("casbin: unsupported policy rule")
	}
	return a.removeRolePermission(rule[0], rule[1])
}

func (a *CasbinAdapter) RemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	if sec != "p" || ptype != "p" || fieldIndex != 0 || len(fieldValues) != 1 {
		return errors.New("casbin: unsupported filtered policy removal")
	}
	return a.db.Where("role_name = ?", fieldValues[0]).Delete(&model.RolePermission{}).Error
}

func (a *CasbinAdapter) addRolePermission(roleName, permissionCode string) error {
	var perm model.Permission
	if err := a.db.Where("code = ?", permissionCode).First(&perm).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var count int64
	if err := a.db.Model(&model.RolePermission{}).
		Where("role_name = ? AND permission_id = ?", roleName, perm.ID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return a.db.Create(&model.RolePermission{RoleName: roleName, PermissionID: perm.ID}).Error
}

func (a *CasbinAdapter) removeRolePermission(roleName, permissionCode string) error {
	var perm model.Permission
	if err := a.db.Where("code = ?", permissionCode).First(&perm).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return a.db.Where("role_name = ? AND permission_id = ?", roleName, perm.ID).
		Delete(&model.RolePermission{}).Error
}

func casbinPolicyLine(roleName, permissionCode string) string {
	return strings.Join([]string{"p", roleName, permissionCode}, ", ")
}
