// Package service — 消息业务逻辑：文本消息的发送、编辑、删除、反应、历史查询。
// 采用 broadcast-first + job queue 模式：先广播给在线客户端，再异步持久化。
// Job queue 失败时同步回退到 DB。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/google/uuid"
)

const MaxMessageRunes = 2000

// MessageDTO is the public data transfer object for messages.
// Used for both broadcast payloads and API responses.
type MessageDTO struct {
	UUID        string     `json:"uuid"`
	RoomUUID    string     `json:"room_uuid"`
	AuthorID    string     `json:"author_id"`
	Content     string     `json:"content"`
	ReplyTo     string     `json:"reply_to,omitempty"`
	Mentions    []string   `json:"mentions,omitempty"`
	EditedAt    *time.Time `json:"edited_at,omitempty"`
	Deleted     bool       `json:"deleted"`
	CreatedAt   time.Time  `json:"created_at"`
	ClientNonce string     `json:"client_nonce,omitempty"`
}

// Narrow interfaces for testability.

// MessageEventBus broadcasts events to online clients in a room.
type MessageEventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}

// MessageJobQueue enqueues durable async persist/mutate jobs.
type MessageJobQueue interface {
	Publish(ctx context.Context, job bus.JobEnvelope) error
}

// roomByUUID looks up a room by its UUID.
// Satisfied by *repository.RoomRepository.
type roomByUUID interface {
	GetByUUID(uuid string) (*model.Room, error)
}

// MessageService provides text message operations with broadcast-first semantics.
type MessageService struct {
	msgRepo *repository.MessageRepository
	roomRepo roomByUUID
	bus      MessageEventBus
	queue    MessageJobQueue
}

// NewMessageService creates a MessageService.
// Room repo is required; bus and queue are optional (set via setters).
func NewMessageService(msgRepo *repository.MessageRepository, roomRepo roomByUUID) *MessageService {
	return &MessageService{
		msgRepo:  msgRepo,
		roomRepo: roomRepo,
	}
}

// SetEventBus sets the event bus for broadcasting to online clients.
func (s *MessageService) SetEventBus(b MessageEventBus) {
	s.bus = b
}

// SetJobQueue sets the job queue for async persistence.
func (s *MessageService) SetJobQueue(q MessageJobQueue) {
	s.queue = q
}

// Send creates and broadcasts a new text message, then enqueues async persist.
// Returns the MessageDTO on success.
func (s *MessageService) Send(roomUUID, authorID, content, replyTo, clientNonce string, mentions []string) (*MessageDTO, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content is required")
	}
	if utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content too long")
	}

	room, err := s.roomRepo.GetByUUID(roomUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}
	if model.NormalizeRoomType(room.Type) != model.RoomTypeText {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not a text room")
	}
	if replyTo != "" {
		parent, err := s.msgRepo.GetByUUID(replyTo)
		if err != nil || parent.RoomUUID != roomUUID {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid reply_to")
		}
	}

	now := time.Now().UTC()
	msgUUID := uuid.New().String()
	dto := &MessageDTO{
		UUID:        msgUUID,
		RoomUUID:    roomUUID,
		AuthorID:    authorID,
		Content:     content,
		ReplyTo:     replyTo,
		Mentions:    mentions,
		Deleted:     false,
		CreatedAt:   now,
		ClientNonce: clientNonce,
	}

	// 1) broadcast first (fire-and-forget; bus nil or error is acceptable)
	if s.bus != nil {
		_ = s.bus.PublishRoom(context.Background(), room.Name, "message:created", dto)
	}

	// 2) enqueue persist; fall back to sync DB write on failure
	payload, _ := json.Marshal(map[string]interface{}{
		"uuid":        msgUUID,
		"room_uuid":   roomUUID,
		"author_id":   authorID,
		"content":     content,
		"reply_to":    replyTo,
		"mentions":    mentions,
		"created_at":  now,
		"client_nonce": clientNonce,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      msgUUID,
			Type:    "chat.persist",
			Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		m := &model.Message{
			UUID: msgUUID, RoomUUID: roomUUID, AuthorID: authorID,
			Content: content, ReplyTo: replyTo,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := s.msgRepo.Create(m); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if len(mentions) > 0 {
			var rows []model.MessageMention
			for _, uid := range mentions {
				rows = append(rows, model.MessageMention{MessageUUID: msgUUID, UserID: uid})
			}
			_ = s.msgRepo.CreateMentions(rows)
		}
	}
	return dto, nil
}

// Edit updates a message's content, broadcasts the change, then enqueues async mutate.
// Only the original author may edit.
func (s *MessageService) Edit(roomUUID, messageUUID, authorID, content string) (*MessageDTO, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content is required")
	}
	if utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content too long")
	}

	msg, err := s.msgRepo.GetByUUID(messageUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.RoomUUID != roomUUID {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.AuthorID != authorID {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not your message")
	}

	room, err := s.roomRepo.GetByUUID(roomUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}
	if model.NormalizeRoomType(room.Type) != model.RoomTypeText {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not a text room")
	}

	now := time.Now().UTC()
	dto := &MessageDTO{
		UUID:      messageUUID,
		RoomUUID:  roomUUID,
		AuthorID:  authorID,
		Content:   content,
		ReplyTo:   msg.ReplyTo,
		EditedAt:  &now,
		Deleted:   false,
		CreatedAt: msg.CreatedAt,
	}

	// 1) broadcast first
	if s.bus != nil {
		_ = s.bus.PublishRoom(context.Background(), room.Name, "message:updated", dto)
	}

	// 2) enqueue mutate
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "edit",
		"message_uuid": messageUUID,
		"content":      content,
		"timestamp":    now,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      messageUUID + "-edit-" + fmt.Sprintf("%d", now.UnixNano()),
			Type:    "chat.mutate",
			Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		if err := s.msgRepo.UpdateContent(messageUUID, content, now); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return dto, nil
}

// Delete soft-deletes a message, broadcasts the deletion, then enqueues async mutate.
// canDeleteOthers allows moderators to delete other users' messages.
func (s *MessageService) Delete(roomUUID, messageUUID, actorID string, canDeleteOthers bool) error {
	msg, err := s.msgRepo.GetByUUID(messageUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.RoomUUID != roomUUID {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.AuthorID != actorID && !canDeleteOthers {
		return pkg.NewAppError(pkg.FORBIDDEN, "not your message")
	}

	room, err := s.roomRepo.GetByUUID(roomUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}

	// 1) broadcast first
	dto := &MessageDTO{
		UUID:      messageUUID,
		RoomUUID:  roomUUID,
		AuthorID:  msg.AuthorID,
		Content:   "",
		Deleted:   true,
		CreatedAt: msg.CreatedAt,
	}
	if s.bus != nil {
		_ = s.bus.PublishRoom(context.Background(), room.Name, "message:deleted", dto)
	}

	// 2) enqueue mutate
	now := time.Now().UTC()
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "delete",
		"message_uuid": messageUUID,
		"timestamp":    now,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      messageUUID + "-del-" + fmt.Sprintf("%d", now.UnixNano()),
			Type:    "chat.mutate",
			Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		if err := s.msgRepo.SoftDelete(messageUUID); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

// React adds a reaction emoji to a message, broadcasts, then enqueues async mutate.
func (s *MessageService) React(roomUUID, messageUUID, userID, emoji string) error {
	if emoji == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "emoji is required")
	}
	msg, err := s.msgRepo.GetByUUID(messageUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.RoomUUID != roomUUID {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}

	room, err := s.roomRepo.GetByUUID(roomUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}

	// 1) broadcast first
	if s.bus != nil {
		_ = s.bus.PublishRoom(context.Background(), room.Name, "message:reaction", map[string]interface{}{
			"action":       "added",
			"message_uuid": messageUUID,
			"user_id":      userID,
			"emoji":        emoji,
		})
	}

	// 2) enqueue mutate
	now := time.Now().UTC()
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "react",
		"message_uuid": messageUUID,
		"user_id":      userID,
		"emoji":        emoji,
		"timestamp":    now,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      messageUUID + "-react-" + userID + "-" + emoji,
			Type:    "chat.mutate",
			Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		if err := s.msgRepo.AddReaction(&model.MessageReaction{
			MessageUUID: messageUUID,
			UserID:      userID,
			Emoji:       emoji,
		}); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

// Unreact removes a reaction emoji from a message, broadcasts, then enqueues async mutate.
func (s *MessageService) Unreact(roomUUID, messageUUID, userID, emoji string) error {
	if emoji == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "emoji is required")
	}
	msg, err := s.msgRepo.GetByUUID(messageUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.RoomUUID != roomUUID {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}

	room, err := s.roomRepo.GetByUUID(roomUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}

	// 1) broadcast first
	if s.bus != nil {
		_ = s.bus.PublishRoom(context.Background(), room.Name, "message:reaction", map[string]interface{}{
			"action":       "removed",
			"message_uuid": messageUUID,
			"user_id":      userID,
			"emoji":        emoji,
		})
	}

	// 2) enqueue mutate
	now := time.Now().UTC()
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "unreact",
		"message_uuid": messageUUID,
		"user_id":      userID,
		"emoji":        emoji,
		"timestamp":    now,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      messageUUID + "-unreact-" + userID + "-" + emoji,
			Type:    "chat.mutate",
			Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		if err := s.msgRepo.RemoveReaction(messageUUID, userID, emoji); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

// ListHistory returns paginated message history for a room, newest first.
// limit is clamped to [50, 200]; default 100.
// Returns items (ASC, oldest-first), hasMore, nextBefore cursor, error.
func (s *MessageService) ListHistory(roomUUID, before string, limit int) (items []MessageDTO, hasMore bool, nextBefore string, err error) {
	if limit <= 0 {
		limit = 100
	}
	if limit < 50 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, more, repoErr := s.msgRepo.ListBefore(roomUUID, before, limit)
	if repoErr != nil {
		return nil, false, "", pkg.NewAppError(pkg.INTERNAL_ERROR, repoErr.Error())
	}

	items = make([]MessageDTO, len(rows))
	for i, m := range rows {
		deleted := m.DeletedAt.Valid
		content := m.Content
		if deleted {
			content = ""
		}
		items[i] = MessageDTO{
			UUID:      m.UUID,
			RoomUUID:  m.RoomUUID,
			AuthorID:  m.AuthorID,
			Content:   content,
			ReplyTo:   m.ReplyTo,
			EditedAt:  m.EditedAt,
			Deleted:   deleted,
			CreatedAt: m.CreatedAt,
		}
	}

	if more && len(items) > 0 {
		nextBefore = items[0].UUID
	}

	return items, more, nextBefore, nil
}

// PersistFromJob is called by the jobs consumer to persist a message from a "chat.persist" job.
func (s *MessageService) PersistFromJob(payload []byte) error {
	var data struct {
		UUID        string    `json:"uuid"`
		RoomUUID    string    `json:"room_uuid"`
		AuthorID    string    `json:"author_id"`
		Content     string    `json:"content"`
		ReplyTo     string    `json:"reply_to"`
		Mentions    []string  `json:"mentions"`
		CreatedAt   time.Time `json:"created_at"`
		ClientNonce string    `json:"client_nonce"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	m := &model.Message{
		UUID: data.UUID, RoomUUID: data.RoomUUID, AuthorID: data.AuthorID,
		Content: data.Content, ReplyTo: data.ReplyTo,
		CreatedAt: data.CreatedAt, UpdatedAt: data.CreatedAt,
	}
	if err := s.msgRepo.Create(m); err != nil {
		return err
	}

	if len(data.Mentions) > 0 {
		var rows []model.MessageMention
		for _, uid := range data.Mentions {
			rows = append(rows, model.MessageMention{MessageUUID: data.UUID, UserID: uid})
		}
		return s.msgRepo.CreateMentions(rows)
	}
	return nil
}

// MutateFromJob is called by the jobs consumer to apply a mutation (edit/delete/react/unreact)
// from a "chat.mutate" job.
func (s *MessageService) MutateFromJob(payload []byte) error {
	var data struct {
		Action      string    `json:"action"`
		MessageUUID string    `json:"message_uuid"`
		Content     string    `json:"content,omitempty"`
		UserID      string    `json:"user_id,omitempty"`
		Emoji       string    `json:"emoji,omitempty"`
		Timestamp   time.Time `json:"timestamp,omitempty"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	if _, err := s.msgRepo.GetByUUID(data.MessageUUID); err != nil {
		return fmt.Errorf("message not ready")
	}

	ts := data.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	switch data.Action {
	case "edit":
		return s.msgRepo.UpdateContent(data.MessageUUID, data.Content, ts)
	case "delete":
		return s.msgRepo.SoftDelete(data.MessageUUID)
	case "react":
		return s.msgRepo.AddReaction(&model.MessageReaction{
			MessageUUID: data.MessageUUID,
			UserID:      data.UserID,
			Emoji:       data.Emoji,
		})
	case "unreact":
		return s.msgRepo.RemoveReaction(data.MessageUUID, data.UserID, data.Emoji)
	default:
		return fmt.Errorf("unknown mutate action: %s", data.Action)
	}
}
