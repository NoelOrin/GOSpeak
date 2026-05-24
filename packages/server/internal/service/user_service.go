package service

import (
	"go_rtc/internal/model"
	"go_rtc/internal/repository"
)

type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

func (s *UserService) GetByID(id uint) (*model.User, error) {
	return s.userRepo.GetByID(id)
}

func (s *UserService) GetByUUID(uuid string) (*model.User, error) {
	return s.userRepo.GetByUUID(uuid)
}

func (s *UserService) List(page, pageSize int) ([]model.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.userRepo.List(page, pageSize)
}

func (s *UserService) Update(user *model.User) error {
	return s.userRepo.Update(user)
}

func (s *UserService) Delete(id uint) error {
	_, err := s.userRepo.GetByID(id)
	if err != nil {
		return err
	}
	return s.userRepo.Delete(id)
}