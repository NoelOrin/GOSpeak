package service

import (
	"context"
	"crypto/rand"
	"log"
	"time"
	"unicode/utf8"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/oklog/ulid/v2"
)

const (
	EventMessageNew  = "message:new"
	MaxMessageRunes  = 500
	DefaultListLimit = 50
	MaxListLimit     = 100
)

type MessageEventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}

type MessageSendInput struct {
	RoomKey        string
	SenderIdentity string
	SenderDisplay  string
	SenderRole     string
	Content        string
	ReplyToID      string
}

type MessageDTO struct {
	ID             string `json:"id"`
	Room           string `json:"room"`
	Content        string `json:"content"`
	ReplyToID      string `json:"replyTo,omitempty"`
	SenderIdentity string `json:"senderIdentity"`
	SenderDisplay  string `json:"senderDisplay"`
	SenderRole     string `json:"senderRole"`
	Timestamp      int64  `json:"timestamp"`
}

type MessageListResult struct {
	Messages   []MessageDTO `json:"messages"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

type MessageService struct {
	repo *repository.MessageRepository
	bus  MessageEventBus
}

func NewMessageService(repo *repository.MessageRepository, bus MessageEventBus) *MessageService {
	return &MessageService{repo: repo, bus: bus}
}

func (s *MessageService) Send(ctx context.Context, in MessageSendInput) (*MessageDTO, error) {
	if in.RoomKey == "" || in.SenderIdentity == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "room and sender required")
	}
	content := in.Content
	if content == "" || utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid content")
	}
	now := time.Now().UTC()
	id, err := ulid.New(ulid.Timestamp(now), rand.Reader)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	row := &model.Message{
		ID:             id.String(),
		RoomUUID:       in.RoomKey,
		SenderIdentity: in.SenderIdentity,
		SenderDisplay:  in.SenderDisplay,
		SenderRole:     in.SenderRole,
		Content:        content,
		ReplyToID:      in.ReplyToID,
		Status:         model.MessageStatusActive,
		CreatedAt:      now,
	}
	if err := s.repo.Create(row); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	dto := toMessageDTO(row)
	if s.bus != nil {
		if err := s.bus.PublishRoom(ctx, in.RoomKey, EventMessageNew, dto); err != nil {
			log.Printf("[IM] publish message:new room=%s id=%s: %v", in.RoomKey, dto.ID, err)
		}
	}
	return &dto, nil
}

func (s *MessageService) List(_ context.Context, roomKey, before string, limit int) (*MessageListResult, error) {
	if roomKey == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "room required")
	}
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}
	rows, err := s.repo.ListByRoom(roomKey, before, limit)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
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

func toMessageDTO(m *model.Message) MessageDTO {
	return MessageDTO{
		ID:             m.ID,
		Room:           m.RoomUUID,
		Content:        m.Content,
		ReplyToID:      m.ReplyToID,
		SenderIdentity: m.SenderIdentity,
		SenderDisplay:  m.SenderDisplay,
		SenderRole:     m.SenderRole,
		Timestamp:      m.CreatedAt.UnixMilli(),
	}
}
