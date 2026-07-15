package repository

import (
	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepository) GetByID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByUUID(uuid string) (*model.User, error) {
	var user model.User
	err := r.db.Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByName(name string) (*model.User, error) {
	var user model.User
	err := r.db.Where("name = ?", name).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) List(page, pageSize int, excludeBots bool) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	query := r.db.Model(&model.User{})
	if excludeBots {
		query = query.Where("is_bot = ?", false)
	}
	query.Count(&total)
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error
	return users, total, err
}

func (r *UserRepository) Update(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepository) Delete(id uint) error {
	return r.db.Delete(&model.User{}, id).Error
}

func (r *UserRepository) UpdateEmail(userID uint, email string) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("email", email).Error
}

func (r *UserRepository) UpdateEmailVerified(userID uint, verified bool) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("email_verified", verified).Error
}
// IncrementTokenVersion 递增用户的 TokenVersion，使所有已签发的 access/refresh token 失效。
// UpdatePasswordAndInvalidate 原子地保存新密码并递增 TokenVersion，
// 避免两步独立写间失败导致旧 token 仍有效（auth_service 改密/重置场景）。
func (r *UserRepository) UpdatePasswordAndInvalidate(user *model.User) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(user).Error; err != nil {
			return err
		}
		return tx.Model(&model.User{}).Where("id = ?", user.ID).
			UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
	})
}


// IncrementTokenVersion 递增用户 TokenVersion，使已签发 token 失效。
func (r *UserRepository) IncrementTokenVersion(userID uint) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}


// UpdateRoleAndInvalidate 原子更新角色并递增 TokenVersion，使旧 JWT 立即失效。
func (r *UserRepository) UpdateRoleAndInvalidate(userID uint, role string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("role", role).Error; err != nil {
			return err
		}
		return tx.Model(&model.User{}).Where("id = ?", userID).
			UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
	})
}
