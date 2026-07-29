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
	return &room, err
}

func (r *RoomRepository) GetByUUID(uuid string) (*model.Room, error) {
	var room model.Room
	err := r.db.Where("uuid = ?", uuid).First(&room).Error
	return &room, err
}

func (r *RoomRepository) GetByName(name string) (*model.Room, error) {
	var room model.Room
	err := r.db.Where("name = ?", name).First(&room).Error
	return &room, err
}

func (r *RoomRepository) List(page, pageSize int, guildUUID string) ([]model.Room, int64, error) {
	var rooms []model.Room
	var total int64
	query := r.db.Model(&model.Room{})
	if guildUUID != "" {
		query = query.Where("guild_uuid = ?", guildUUID)
	}
	query.Count(&total)
	err := query.Order("created_at ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rooms).Error
	return rooms, total, err
}

func (r *RoomRepository) Update(room *model.Room) error {
	return r.db.Save(room).Error
}

func (r *RoomRepository) Delete(id uint) error {
	return r.db.Delete(&model.Room{}, id).Error
}
