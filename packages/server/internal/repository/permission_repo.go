package repository

import (
	"go_rtc/internal/model"

	"gorm.io/gorm"
)

type PermissionRepository struct {
	db *gorm.DB
}

func NewPermissionRepository(db *gorm.DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// List 所有权限
func (r *PermissionRepository) List() ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.Find(&perms).Error
	return perms, err
}

// GetByCode 按权限码查询
func (r *PermissionRepository) GetByCode(code string) (*model.Permission, error) {
	var perm model.Permission
	err := r.db.Where("code = ?", code).First(&perm).Error
	return &perm, err
}

// CreateIfNotExists 种子用：不存在则创建
func (r *PermissionRepository) CreateIfNotExists(perm *model.Permission) error {
	return r.db.Where("code = ?", perm.Code).FirstOrCreate(perm).Error
}

// GetRolePermissions 获取指定角色的所有权限码
func (r *PermissionRepository) GetRolePermissions(roleName string) ([]string, error) {
	var codes []string
	err := r.db.Model(&model.RolePermission{}).
		Select("permissions.code").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_name = ?", roleName).
		Pluck("code", &codes).Error
	return codes, err
}

// GetAllRolePermissions 获取所有角色的权限映射
func (r *PermissionRepository) GetAllRolePermissions() (map[string][]string, error) {
	var rps []model.RolePermission
	if err := r.db.Find(&rps).Error; err != nil {
		return nil, err
	}

	// 获取所有权限 id → code 映射
	perms, err := r.List()
	if err != nil {
		return nil, err
	}
	idToCode := make(map[uint]string, len(perms))
	for _, p := range perms {
		idToCode[p.ID] = p.Code
	}

	result := make(map[string][]string)
	for _, rp := range rps {
		if code, ok := idToCode[rp.PermissionID]; ok {
			result[rp.RoleName] = append(result[rp.RoleName], code)
		}
	}
	return result, nil
}

// SyncRolePermissions 同步角色权限（全量覆盖指定角色的权限）
func (r *PermissionRepository) SyncRolePermissions(roleName string, permCodes []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 删除该角色的旧权限
		if err := tx.Where("role_name = ?", roleName).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		// 插入新权限
		for _, code := range permCodes {
			var perm model.Permission
			if err := tx.Where("code = ?", code).First(&perm).Error; err != nil {
				continue // 跳过不存在的权限码
			}
			rp := model.RolePermission{
				RoleName:     roleName,
				PermissionID: perm.ID,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
