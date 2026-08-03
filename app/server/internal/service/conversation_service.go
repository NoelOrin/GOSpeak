package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/google/uuid"
)

const (
	DefaultListLimit = 50
	MaxListLimit     = 200
)

type MessageListResult struct {
	Messages    []MessageDTO `json:"messages"`
	NextCursor  string       `json:"next_cursor,omitempty"`
	UnreadCount int          `json:"unread_count,omitempty"` // 离线拉取时返回该会话当前未读数
}

type ConversationService struct {
	convRepo    *repository.ConversationRepository
	messageRepo *repository.MessageRepository
	eventBus    bus.EventBus
}

func NewConversationService(
	convRepo *repository.ConversationRepository,
	messageRepo *repository.MessageRepository,
) *ConversationService {
	return &ConversationService{convRepo: convRepo, messageRepo: messageRepo}
}

// SetEventBus injects the event bus used to broadcast private-message events.
func (s *ConversationService) SetEventBus(b bus.EventBus) {
	s.eventBus = b
}

// ConversationDTO is the client-facing view of a conversation.
type ConversationDTO struct {
	ConversationID     string `json:"conversation_id"`
	OtherIdentity      string `json:"other_identity"`
	OtherDisplayName   string `json:"other_display_name"`
	OtherAvatar        string `json:"other_avatar,omitempty"`
	LastContent        string `json:"last_content"`
	LastSenderIdentity string `json:"last_sender_identity"`
	LastMessageAt      int64  `json:"last_message_at"`
	UnreadCount        int    `json:"unread_count"`
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

// toMessageDTO converts a model.Message to a MessageDTO.
func toMessageDTO(m *model.Message) MessageDTO {
	return MessageDTO{
		UUID:      m.UUID,
		RoomUUID:  m.RoomUUID,
		AuthorID:  m.AuthorID,
		Content:   m.Content,
		ReplyTo:   m.ReplyTo,
		EditedAt:  m.EditedAt,
		Deleted:   m.DeletedAt.Valid,
		CreatedAt: m.CreatedAt,
	}
}

// 私聊字段从持久化模型回填，方便前端直接插入会话。
func toPrivateMessageDTO(m *model.Message) MessageDTO {
	dto := toMessageDTO(m)
	if m.ConversationID != nil {
		dto.ConversationID = *m.ConversationID
	}
	if m.TargetIdentity != nil {
		dto.TargetIdentity = *m.TargetIdentity
	}
	return dto
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

	rows, hasMore, err := s.messageRepo.ListBeforeConversation(conversationID, before, limit)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	unread := cp.UnreadCountB
	if cp.IdentityA == identity {
		unread = cp.UnreadCountA
	}
	out := &MessageListResult{UnreadCount: unread, Messages: make([]MessageDTO, 0, len(rows))}
	for i := range rows {
		out.Messages = append(out.Messages, toPrivateMessageDTO(&rows[i]))
	}
	if hasMore && len(rows) > 0 {
		out.NextCursor = rows[0].UUID
	}
	return out, nil
}

// MarkRead resets the unread count for a conversation for the given identity.
func (s *ConversationService) MarkRead(conversationID, identity string) error {
	if conversationID == "" || identity == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "conversation_id and identity required")
	}
	cp, err := s.convRepo.GetByID(conversationID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "conversation not found")
	}
	if cp.IdentityA != identity && cp.IdentityB != identity {
		return pkg.NewAppError(pkg.FORBIDDEN, "not a conversation participant")
	}
	if err := s.convRepo.ResetUnread(conversationID, identity); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// ConvTo returns the identity of the other participant in a conversation.
func ConvTo(cp *model.ConversationParticipant, self string) string {
	if cp.IdentityA != self {
		return cp.IdentityA
	}
	return cp.IdentityB
}

// SendDirect creates and broadcasts a private message, then persists it.
func (s *ConversationService) SendDirect(senderIdentity, targetIdentity, content, clientNonce string) (*MessageDTO, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content is required")
	}
	if utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content too long")
	}
	if senderIdentity == "" || targetIdentity == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "sender and target required")
	}
	if senderIdentity == targetIdentity {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "cannot send to self")
	}

	// Generate conversation ID: sorted identities → MD5 hex (32 chars)
	identities := []string{senderIdentity, targetIdentity}
	sort.Strings(identities)
	hashBytes := md5.Sum([]byte(identities[0] + ":" + identities[1]))
	convID := hex.EncodeToString(hashBytes[:])

	now := time.Now().UTC()
	msgUUID := uuid.New().String()

	// Upsert conversation participant row
	cp := &model.ConversationParticipant{
		ConversationID:     convID,
		IdentityA:          identities[0],
		IdentityB:          identities[1],
		LastContent:        content,
		LastSenderIdentity: senderIdentity,
		LastMessageAt:      &now,
	}
	if err := s.convRepo.Upsert(cp); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// Increment unread for receiver
	if err := s.convRepo.IncrementUnread(convID, senderIdentity); err != nil {
		log.Printf("[Conversation] increment unread %s failed: %v", convID, err)
	}

	dto := &MessageDTO{
		UUID:           msgUUID,
		AuthorID:       senderIdentity,
		Content:        content,
		Deleted:        false,
		CreatedAt:      now,
		ClientNonce:    clientNonce,
		ConversationID: convID,
		TargetIdentity: targetIdentity,
	}

	// 定向投递到双方的个人房间，避免 private:new 广播给所有在线连接。
	// Hub.OnConnect 会把每个已认证连接加入 __user:{identity}。
	if s.eventBus != nil {
		payload, _ := json.Marshal(dto)
		if err := s.eventBus.PublishRoom(context.Background(), "__user:"+senderIdentity, "private:new", payload); err != nil {
			log.Printf("[Conversation] publish private:new to sender %s failed: %v", senderIdentity, err)
		}
		if err := s.eventBus.PublishRoom(context.Background(), "__user:"+targetIdentity, "private:new", payload); err != nil {
			log.Printf("[Conversation] publish private:new to receiver %s failed: %v", targetIdentity, err)
		}
	}
	// Persist message
	convIDPtr := convID
	targetPtr := targetIdentity
	msg := &model.Message{
		UUID:             msgUUID,
		AuthorID:         senderIdentity,
		Content:          content,
		CreatedAt:        now,
		UpdatedAt:        now,
		ConversationType: "private",
		ConversationID:   &convIDPtr,
		TargetIdentity:   &targetPtr,
	}
	if err := s.messageRepo.Create(msg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	return dto, nil
}
