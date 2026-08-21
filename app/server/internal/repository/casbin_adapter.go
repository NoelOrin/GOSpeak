// Package repository 提供数据访问实现。
package repository

import (
	"errors"
	"fmt"
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
	var rules []model.RolePermission
	if err := a.db.Find(&rules).Error; err != nil {
		return err
	}

	for _, rule := range rules {
		if err := persist.LoadPolicyArray([]string{"p", "p", rule.RoleName, fmt.Sprintf("%d", rule.PermissionID)}, m); err != nil {
			return err
		}
	}

	var roles []model.Role
	_ = roles
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
