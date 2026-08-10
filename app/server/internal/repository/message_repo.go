package repository

import (
	"GOSpeak/internal/model"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *MessageRepository) CreateMentions(mentions []model.MessageMention) error {
	if len(mentions) == 0 {
		return nil
	}
	return r.db.Create(&mentions).Error
}

// EnsureMentions 幂等写入 @提及关系，job 重放/兜底并发时不产生重复行。
func (r *MessageRepository) EnsureMentions(mentions []model.MessageMention) error {
	if len(mentions) == 0 {
		return nil
	}
	for i := range mentions {
		mentions[i].ID = 0
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&mentions).Error
}

func (r *MessageRepository) GetByUUID(uuid string) (*model.Message, error) {
	var m model.Message
	err := r.db.Where("uuid = ?", uuid).First(&m).Error
	return &m, err
}

func (r *MessageRepository) UpdateContent(uuid, content string, editedAt time.Time) error {
	return r.db.Model(&model.Message{}).Where("uuid = ?", uuid).
		Updates(map[string]interface{}{"content": content, "edited_at": editedAt}).Error
}

func (r *MessageRepository) SoftDelete(uuid string) error {
	// 显式状态枚举 + 软删除时间双写，避免状态仅由 DeletedAt 派生。
	return r.db.Unscoped().Model(&model.Message{}).Where("uuid = ?", uuid).
		Updates(map[string]interface{}{
			"deleted_at": time.Now(),
			"status":     model.MessageStatusDeleted,
		}).Error
}

// ListBefore returns up to limit room messages older than beforeUUID (exclusive), ASC.
// beforeUUID empty = newest page. hasMore true if more older rows exist.
func (r *MessageRepository) ListBefore(roomUUID, beforeUUID string, limit int) ([]model.Message, bool, error) {
	return r.listBefore(r.db.Model(&model.Message{}).Where("room_uuid = ?", roomUUID), beforeUUID, limit)
}

// ListBeforeConversation returns up to limit private messages older than beforeUUID (exclusive), ASC.
func (r *MessageRepository) ListBeforeConversation(conversationID, beforeUUID string, limit int) ([]model.Message, bool, error) {
	return r.listBefore(r.db.Model(&model.Message{}).
		Where("conversation_type = ? AND conversation_id = ?", "private", conversationID), beforeUUID, limit)
}

func (r *MessageRepository) listBefore(q *gorm.DB, beforeUUID string, limit int) ([]model.Message, bool, error) {
	if limit < 1 {
		limit = 100
	}
	if beforeUUID != "" {
		var pivot model.Message
		if err := r.db.Where("uuid = ?", beforeUUID).First(&pivot).Error; err != nil {
			return nil, false, err
		}
		q = q.Where("(created_at < ?) OR (created_at = ? AND id < ?)", pivot.CreatedAt, pivot.CreatedAt, pivot.ID)
	}
	var rows []model.Message
	// fetch limit+1 DESC then reverse
	err := q.Order("created_at DESC").Order("id DESC").Limit(limit + 1).Find(&rows).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	// reverse to ASC
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, hasMore, nil
}

func (r *MessageRepository) AddReaction(re *model.MessageReaction) error {
	return r.db.Where(model.MessageReaction{
		MessageUUID: re.MessageUUID,
		UserID:      re.UserID,
		Emoji:       re.Emoji,
	}).FirstOrCreate(re).Error
}

func (r *MessageRepository) RemoveReaction(messageUUID, userID, emoji string) error {
	return r.db.Where("message_uuid = ? AND user_id = ? AND emoji = ?", messageUUID, userID, emoji).
		Delete(&model.MessageReaction{}).Error
}

func (r *MessageRepository) ListReactions(messageUUIDs []string) ([]model.MessageReaction, error) {
	if len(messageUUIDs) == 0 {
		return nil, nil
	}
	var rows []model.MessageReaction
	err := r.db.Where("message_uuid IN ?", messageUUIDs).Find(&rows).Error
	return rows, err
}

// Search 在文本房间内容中做包含匹配，返回最新消息。
func (r *MessageRepository) Search(roomUUID, query string, limit int) ([]model.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	var rows []model.Message
	err := r.db.Where("room_uuid = ? AND content LIKE ?", roomUUID, "%"+query+"%").
		Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// ListMentions 按消息 UUID 批量读取 @提及关系。
func (r *MessageRepository) ListMentions(messageUUIDs []string) ([]model.MessageMention, error) {
	if len(messageUUIDs) == 0 {
		return nil, nil
	}
	var rows []model.MessageMention
	err := r.db.Where("message_uuid IN ?", messageUUIDs).Find(&rows).Error
	return rows, err
}
