package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
)

type ConversationService struct {
	convRepo    *repository.ConversationRepository
	messageRepo *repository.MessageRepository
}

func NewConversationService(
	convRepo *repository.ConversationRepository,
	messageRepo *repository.MessageRepository,
) *ConversationService {
	return &ConversationService{convRepo: convRepo, messageRepo: messageRepo}
}

// ConversationDTO is the client-facing view of a conversation.
type ConversationDTO struct {
	ConversationID      string `json:"conversation_id"`
	OtherIdentity       string `json:"other_identity"`
	OtherDisplayName    string `json:"other_display_name"`
	OtherAvatar         string `json:"other_avatar,omitempty"`
	LastContent         string `json:"last_content"`
	LastSenderIdentity  string `json:"last_sender_identity"`
	LastMessageAt       int64  `json:"last_message_at"`
	UnreadCount         int    `json:"unread_count"`
}

// List returns all conversations involving the given identity.
func (s *ConversationService) List(identity string, limit int) ([]ConversationDTO, error) {
	if identity == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "identity required")
	}
	rows, err := s.convRepo.ListByIdentity(identity, limit)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	out := make([]ConversationDTO, 0, len(rows))
	for _, cp := range rows {
		other := cp.IdentityB
		unread := cp.UnreadCountA
		if cp.IdentityA != identity {
			other = cp.IdentityA
			unread = cp.UnreadCountB
		}
		lat := int64(0)
		if cp.LastMessageAt != nil {
			lat = cp.LastMessageAt.UnixMilli()
		}
		out = append(out, ConversationDTO{
			ConversationID:     cp.ConversationID,
			OtherIdentity:      other,
			LastContent:        cp.LastContent,
			LastSenderIdentity: cp.LastSenderIdentity,
			LastMessageAt:      lat,
			UnreadCount:        unread,
		})
	}
	return out, nil
}

// GetMessages returns paginated messages for a conversation.
// The caller must be a participant (checked against IdentityA / IdentityB).
func (s *ConversationService) GetMessages(conversationID, identity, before string, limit int) (*MessageListResult, error) {
	if conversationID == "" || identity == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "conversation_id and identity required")
	}
	cp, err := s.convRepo.GetByID(conversationID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "conversation not found")
	}
	if cp.IdentityA != identity && cp.IdentityB != identity {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not a conversation participant")
	}

	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}

	rows, err := s.messageRepo.ListByConversation(conversationID, before, limit)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	out := &MessageListResult{Messages: make([]MessageDTO, 0, len(rows))}
	for i := range rows {
		out.Messages = append(out.Messages, toMessageDTO(&rows[i]))
	}
	if len(rows) == limit {
		out.NextCursor = rows[0].ID
	}
	return out, nil
}

// MarkRead resets the unread count for a conversation for the given identity.
func (s *ConversationService) MarkRead(conversationID, identity string) error {
	if conversationID == "" || identity == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "conversation_id and identity required")
	}
	return s.convRepo.ResetUnread(conversationID, identity)
}

// ConvTo returns the identity of the other participant in a conversation.
func ConvTo(cp *model.ConversationParticipant, self string) string {
	if cp.IdentityA != self {
		return cp.IdentityA
	}
	return cp.IdentityB
}
