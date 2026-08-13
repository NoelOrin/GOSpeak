package repository

import (
	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type RoleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) List() ([]model.Role, error) {
	var roles []model.Role
	err := r.db.Find(&roles).Error
	return roles, err
}

func (r *RoleRepository) CreateIfNotExists(role *model.Role) error {
	return r.db.Where("name = ?", role.Name).FirstOrCreate(role).Error
}

func (r *RoleRepository) Create(role *model.Role) error {
	return r.db.Create(role).Error
}

func (r *RoleRepository) Delete(id uint) error {
	result := r.db.Delete(&model.Role{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *RoleRepository) Update(id uint, name string) (*model.Role, error) {
	var role model.Role
	if err := r.db.First(&role, id).Error; err != nil {
		return nil, err
	}
	role.Name = name
	if err := r.db.Save(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}
