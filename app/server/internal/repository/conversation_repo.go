package repository

import (
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ConversationRepository struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

// Upsert inserts or updates a conversation participant row (PK = ConversationID).
func (r *ConversationRepository) Upsert(cp *model.ConversationParticipant) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "conversation_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"last_message_id",
			"last_content",
			"last_sender_identity",
			"last_message_at",
		}),
	}).Create(cp).Error
}

// ListByIdentity returns all conversations involving the given identity,
// ordered by most recent activity descending.
func (r *ConversationRepository) ListByIdentity(identity string, limit int) ([]model.ConversationParticipant, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []model.ConversationParticipant
	err := r.db.
		Where("identity_a = ? OR identity_b = ?", identity, identity).
		Order("COALESCE(last_message_at, created_at) DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// ListByIdentityCursor returns a cursor-paginated conversation list for the
// given identity, ordered by most recent activity descending. beforeID is the
// conversation_id of the previous page's last row; limit is clamped to 1..100.
func (r *ConversationRepository) ListByIdentityCursor(identity, beforeID string, limit int) ([]model.ConversationParticipant, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows []model.ConversationParticipant
	q := r.db.
		Where("identity_a = ? OR identity_b = ?", identity, identity).
		Order("COALESCE(last_message_at, created_at) DESC, conversation_id DESC")
	if beforeID != "" {
		var pivot model.ConversationParticipant
		if err := r.db.First(&pivot, "conversation_id = ?", beforeID).Error; err != nil {
			return nil, false, err
		}
		pivotAt := pivot.CreatedAt
		if pivot.LastMessageAt != nil {
			pivotAt = *pivot.LastMessageAt
		}
		q = q.Where(
			"(COALESCE(last_message_at, created_at), conversation_id) < (?, ?)",
			pivotAt, beforeID,
		)
	}
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return rows, hasMore, nil
}

// GetByID retrieves a conversation participant row by its primary key.
func (r *ConversationRepository) GetByID(conversationID string) (*model.ConversationParticipant, error) {
	var cp model.ConversationParticipant
	if err := r.db.First(&cp, "conversation_id = ?", conversationID).Error; err != nil {
		return nil, err
	}
	return &cp, nil
}

// IncrementUnread bumps the unread counter for the side that did NOT send the message.
// senderIdentity is the message sender; the receiver's counter gets incremented.
func (r *ConversationRepository) IncrementUnread(conversationID, senderIdentity string) error {
	// Determine which side the sender is on
	cp, err := r.GetByID(conversationID)
	if err != nil {
		return err
	}
	if cp.IdentityA == senderIdentity {
		// sender is A → increment B's counter
		return r.db.Model(&model.ConversationParticipant{}).
			Where("conversation_id = ?", conversationID).
			Update("unread_count_b", gorm.Expr("unread_count_b + 1")).Error
	}
	// sender is B → increment A's counter
	return r.db.Model(&model.ConversationParticipant{}).
		Where("conversation_id = ?", conversationID).
		Update("unread_count_a", gorm.Expr("unread_count_a + 1")).Error
}

// ResetUnread zeroes the unread counter for the side the given identity is on.
func (r *ConversationRepository) ResetUnread(conversationID, identity string) error {
	cp, err := r.GetByID(conversationID)
	if err != nil {
		return err
	}
	if cp.IdentityA == identity {
		return r.db.Model(&model.ConversationParticipant{}).
			Where("conversation_id = ?", conversationID).
			Update("unread_count_a", 0).Error
	}
	return r.db.Model(&model.ConversationParticipant{}).
		Where("conversation_id = ?", conversationID).
		Update("unread_count_b", 0).Error
}

// UpdateLastMessage updates the conversation's summary fields with the latest message info.
func (r *ConversationRepository) UpdateLastMessage(convID, msgID, content, senderID string, t time.Time) error {
	return r.db.Model(&model.ConversationParticipant{}).
		Where("conversation_id = ?", convID).
		Updates(map[string]interface{}{
			"last_message_id":      msgID,
			"last_content":         content,
			"last_sender_identity": senderID,
			"last_message_at":      t,
		}).Error
}
