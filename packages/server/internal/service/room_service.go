package service

import (
	"go_rtc/internal/model"
	"go_rtc/internal/repository"
)

type RoomService struct {
	roomRepo *repository.RoomRepository
}

func NewRoomService(roomRepo *repository.RoomRepository) *RoomService {
	return &RoomService{roomRepo: roomRepo}
}

func (s *RoomService) Create(room *model.Room) error {
	return s.roomRepo.Create(room)
}

func (s *RoomService) GetByID(id uint) (*model.Room, error) {
	return s.roomRepo.GetByID(id)
}

func (s *RoomService) GetByUUID(uuid string) (*model.Room, error) {
	return s.roomRepo.GetByUUID(uuid)
}

func (s *RoomService) List(page, pageSize int) ([]model.Room, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.roomRepo.List(page, pageSize)
}

func (s *RoomService) Update(room *model.Room) error {
	return s.roomRepo.Update(room)
}

func (s *RoomService) Delete(id uint) error {
	_, err := s.roomRepo.GetByID(id)
	if err != nil {
		return err
	}
	return s.roomRepo.Delete(id)
}