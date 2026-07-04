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

func (r *UserRepository) List(page, pageSize int) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	r.db.Model(&model.User{}).Count(&total)
	err := r.db.Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error
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
func (r *UserRepository) IncrementTokenVersion(userID uint) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}
