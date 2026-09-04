package repository

import (
	"GOSpeak/internal/model"

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

func (r *PermissionRepository) HasRolePermissions(roleName string) (bool, error) {

	var count int64
	err := r.db.Model(&model.RolePermission{}).Where("role_name = ?", roleName).Count(&count).Error
	return count > 0, err
}

// RenameRolePermissions 角色改名时把权限关联行迁移到新角色名。
func (r *PermissionRepository) RenameRolePermissions(oldName, newName string) error {
	return r.db.Model(&model.RolePermission{}).
		Where("role_name = ?", oldName).
		Update("role_name", newName).Error
}

func (r *PermissionRepository) SeedRolePermissionsIfEmpty(roleName string, permCodes []string) error {
	hasPerms, err := r.HasRolePermissions(roleName)
	if err != nil {
		return err
	}
	if !hasPerms {
		return r.SyncRolePermissions(roleName, permCodes)
	}
	return nil
}

// EnsureRolePermissions 将默认权限码合并进角色（只增不删）。
// 用于版本升级时把 DefaultRolePermissions 中新增的权限补齐到已存在的角色，
// 而不会清除管理员在运行时对角色做的自定义调整。
func (r *PermissionRepository) EnsureRolePermissions(roleName string, permCodes []string) error {
	for _, code := range permCodes {
		var count int64
		err := r.db.Model(&model.RolePermission{}).
			Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
			Where("role_permissions.role_name = ? AND permissions.code = ?", roleName, code).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		var perm model.Permission
		if err := r.db.Where("code = ?", code).First(&perm).Error; err != nil {
			continue
		}
		rp := model.RolePermission{RoleName: roleName, PermissionID: perm.ID}
		if err := r.db.Create(&rp).Error; err != nil {
			return err
		}
	}
	return nil
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
