package repository

import (
	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(msg *model.Message) error {
	return r.db.Create(msg).Error
}

// ListByRoom returns up to limit active messages in ascending id order.
// If beforeULID is non-empty, only rows with id < beforeULID.
func (r *MessageRepository) ListByRoom(roomUUID, beforeULID string, limit int) ([]model.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	q := r.db.Where("room_uuid = ? AND status = ?", roomUUID, model.MessageStatusActive)
	if beforeULID != "" {
		q = q.Where("id < ?", beforeULID)
	}
	var rows []model.Message
	err := q.Order("id DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}
