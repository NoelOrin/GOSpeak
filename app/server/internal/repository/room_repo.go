package repository

import (
	"GOSpeak/internal/model"
	"gorm.io/gorm"
)

type RoomRepository struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

func (r *RoomRepository) Create(room *model.Room) error {
	return r.db.Create(room).Error
}

func (r *RoomRepository) GetByID(id uint) (*model.Room, error) {
	var room model.Room
	err := r.db.First(&room, id).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepository) GetByUUID(uuid string) (*model.Room, error) {
	var room model.Room
	err := r.db.Where("uuid = ?", uuid).First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepository) GetByName(name string) (*model.Room, error) {
	var room model.Room
	err := r.db.Where("name = ?", name).First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// GetByGuildAndName 按 Server 和房间名精确查询，避免同名房间跨 Server 串用。
func (r *RoomRepository) GetByGuildAndName(guildUUID, name string) (*model.Room, error) {
	var room model.Room
	q := r.db.Where("name = ?", name)
	if guildUUID == "" {
		q = q.Where("guild_uuid = ?", "")
	} else {
		q = q.Where("guild_uuid = ?", guildUUID)
	}
	err := q.First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepository) List(page, pageSize int, roomType, guildUUID string) ([]model.Room, int64, error) {
	return r.list(page, pageSize, roomType, guildUUID, false)
}

func (r *RoomRepository) ListPlatform(page, pageSize int, roomType string) ([]model.Room, int64, error) {
	return r.list(page, pageSize, roomType, "", true)
}

func (r *RoomRepository) list(page, pageSize int, roomType, guildUUID string, platformOnly bool) ([]model.Room, int64, error) {
	var rooms []model.Room
	var total int64
	q := r.db.Model(&model.Room{})
	if roomType == model.RoomTypeText || roomType == model.RoomTypeVoice {
		q = q.Where("type = ?", roomType)
	}
	if platformOnly {
		q = q.Where("guild_uuid = ?", "")
	} else if guildUUID != "" {
		q = q.Where("guild_uuid = ?", guildUUID)
	}
	q.Count(&total)
	err := q.Order("created_at ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rooms).Error
	return rooms, total, err
}
func (r *RoomRepository) Update(room *model.Room) error {
	return r.db.Save(room).Error
}

func (r *RoomRepository) Delete(id uint) error {
	return r.db.Delete(&model.Room{}, id).Error
}
