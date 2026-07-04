// Package service — 房间业务逻辑：房间的 CRUD 操作。
package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// RoomService 房间服务，提供房间的增删改查能力。
type RoomService struct {
	roomRepo *repository.RoomRepository
}

func NewRoomService(roomRepo *repository.RoomRepository) *RoomService {
	return &RoomService{roomRepo: roomRepo}
}

// Create 创建房间，透传 repository 层结果。
func (s *RoomService) Create(room *model.Room) error {
	if err := s.roomRepo.Create(room); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// CreateRoom creates a room from primitive parameters so the handler
// layer does not need to import model.
func (s *RoomService) CreateRoom(name, password, description string, limit uint, audioOnly, allowAudience bool, createdBy string) (*model.Room, error) {
	room := &model.Room{
		Name:          name,
		Password:      password,
		Description:   description,
		Limit:         limit,
		AudioOnly:     audioOnly,
		AllowAudience: allowAudience,
		CreatedBy:     createdBy,
	}
	if err := s.roomRepo.Create(room); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return room, nil
}

// GetByID 按主键 ID 查询房间。
func (s *RoomService) GetByID(id uint) (*model.Room, error) {
	room, err := s.roomRepo.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "room not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return room, nil
}

// GetByUUID 按 UUID 查询房间。
func (s *RoomService) GetByUUID(uuid string) (*model.Room, error) {
	room, err := s.roomRepo.GetByUUID(uuid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "room not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return room, nil
}

// GetByName 按名称查询房间。
func (s *RoomService) GetByName(name string) (*model.Room, error) {
	room, err := s.roomRepo.GetByName(name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "room not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return room, nil
}

// List 分页查询房间列表，默认每页 20 条。
func (s *RoomService) List(page, pageSize int) ([]model.Room, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	rooms, total, err := s.roomRepo.List(page, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return rooms, total, nil
}

// Update 更新房间信息。
func (s *RoomService) Update(room *model.Room) error {
	if err := s.roomRepo.Update(room); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// Delete 删除房间：先验证存在性，再执行删除。
func (s *RoomService) Delete(id uint) error {
	_, err := s.roomRepo.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.NewAppError(pkg.NOT_FOUND, "room not found")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.roomRepo.Delete(id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}
