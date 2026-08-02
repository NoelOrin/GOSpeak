// Package service — 房间业务逻辑：房间的 CRUD 操作。
package service

import (
	"errors"
	"sync"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// ErrRoomNotFound is returned when a room query finds no matching row.
var ErrRoomNotFound = pkg.NewAppError(pkg.NOT_FOUND, "room not found")

// RoomService 房间服务，提供房间的增删改查能力。
type RoomService struct {
	roomRepo *repository.RoomRepository
	mu       sync.Mutex
}

func NewRoomService(roomRepo *repository.RoomRepository) *RoomService {
	return &RoomService{roomRepo: roomRepo}
}

// Create 创建房间，并保证同一 Domain 内房间名唯一。
func (s *RoomService) Create(room *model.Room) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.roomRepo.GetByDomainAndName(room.DomainUUID, room.Name)
	if err == nil {
		return pkg.NewAppError(pkg.ALREADY_EXISTS, "room already exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.roomRepo.Create(room); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return pkg.NewAppError(pkg.ALREADY_EXISTS, "room already exists")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// CreateRoom creates a room from primitive parameters so the handler
// layer does not need to import model.
func (s *RoomService) CreateRoom(name, password, description string, limit uint, audioOnly, allowAudience bool, createdBy, roomType, domainUUID string) (*model.Room, error) {
	hashedPassword, err := pkg.HashPassword(password)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	room := &model.Room{
		Name:          name,
		Password:      hashedPassword,
		Description:   description,
		Limit:         limit,
		AudioOnly:     audioOnly,
		AllowAudience: allowAudience,
		CreatedBy:     createdBy,
		Type:          model.NormalizeRoomType(roomType),
		DomainUUID:    domainUUID,
	}
	// text rooms: force audio_only true / no SFU expectations
	if room.Type == model.RoomTypeText {
		room.AudioOnly = true
	}
	if err := s.Create(room); err != nil {
		return nil, err
	}
	return room, nil
}

// GetByID 按主键 ID 查询房间。
func (s *RoomService) GetByID(id uint) (*model.Room, error) {
	room, err := s.roomRepo.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRoomNotFound
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
			return nil, ErrRoomNotFound
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
			return nil, ErrRoomNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return room, nil
}

// GetByDomainAndName 按域和房间名精确查询。
func (s *RoomService) GetByDomainAndName(domainUUID, name string) (*model.Room, error) {
	room, err := s.roomRepo.GetByDomainAndName(domainUUID, name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRoomNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return room, nil
}

// List 分页查询房间列表，默认每页 20 条。
func (s *RoomService) List(page, pageSize int, roomType, domainUUID string) ([]model.Room, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if roomType != "" && roomType != model.RoomTypeText && roomType != model.RoomTypeVoice {
		return nil, 0, pkg.NewAppError(pkg.INVALID_PARAMS, "type must be text, voice, or empty")
	}
	rooms, total, err := s.roomRepo.List(page, pageSize, roomType, domainUUID)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return rooms, total, nil
}

// ListPlatform 分页查询平台级房间（无 DomainUUID）。
func (s *RoomService) ListPlatform(page, pageSize int, roomType string) ([]model.Room, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if roomType != "" && roomType != model.RoomTypeText && roomType != model.RoomTypeVoice {
		return nil, 0, pkg.NewAppError(pkg.INVALID_PARAMS, "type must be text, voice, or empty")
	}
	rooms, total, err := s.roomRepo.ListPlatform(page, pageSize, roomType)
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

// Delete 删除房间，先检查是否存在。
func (s *RoomService) Delete(id uint) error {
	if _, err := s.roomRepo.GetByID(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrRoomNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.roomRepo.Delete(id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}
