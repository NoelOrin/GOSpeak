// Package service — 消息业务逻辑：文本消息的发送、编辑、删除、反应、历史查询。
// 采用 broadcast-first + job queue 模式：先广播给在线客户端，再异步持久化。
// Job queue 失败时同步回退到 DB。
package service

import (
	"context"
	"encoding/json"
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

// MessageDTO is the public data transfer object for messages.
// Used for both broadcast payloads and API responses.
type MessageDTO struct {
	UUID           string     `json:"uuid"`
	RoomUUID       string     `json:"room_uuid"`
	AuthorID       string     `json:"author_id"`
	AuthorUUID     string     `json:"author_uuid"`
	AuthorName     string     `json:"author_name"`
	AuthorAvatar   string     `json:"author_avatar"`
	Content        string     `json:"content"`
	ReplyTo        string     `json:"reply_to,omitempty"`
	Mentions       []string   `json:"mentions,omitempty"`
	EditedAt       *time.Time `json:"edited_at,omitempty"`
	Deleted        bool       `json:"deleted"`
	CreatedAt      time.Time  `json:"created_at"`
	ClientNonce    string     `json:"client_nonce,omitempty"`
	ConversationID string     `json:"conversation_id,omitempty"`
	TargetIdentity string     `json:"target_identity,omitempty"`
}

// Narrow interfaces for testability.

// MessageActor separates the display/author identity from the stable user UUID.
// Domain membership must be checked with UserUUID; AuthorID stays the username.

// Narrow interfaces for testability.

// MessageActor separates the display/author identity from the stable user UUID.
// Domain membership must be checked with UserUUID; AuthorID stays the username.
type MessageActor struct {
	Identity string
	UserUUID string
}

// DomainMemberChecker verifies a user belongs to a room's owning domain.

// DomainMemberChecker verifies a user belongs to a room's owning domain.
type DomainMemberChecker interface {
	IsMember(domainUUID, userUUID string) bool
}

// MessageEventBus broadcasts events to online clients in a room.

// MessageEventBus broadcasts events to online clients in a room.
type MessageEventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}

// MessageJobQueue enqueues durable async persist/mutate jobs.

// MessageJobQueue enqueues durable async persist/mutate jobs.
type MessageJobQueue interface {
	Publish(ctx context.Context, job bus.JobEnvelope) error
}

// roomByUUID looks up a room by its UUID.
// Satisfied by *repository.RoomRepository.

// roomByUUID looks up a room by its UUID.
// Satisfied by *repository.RoomRepository.
type roomByUUID interface {
	GetByUUID(uuid string) (*model.Room, error)
}

// MessageService provides text message operations with broadcast-first semantics.
// userByName looks up a user by name.
// Satisfied by *repository.UserRepository.

// MessageService provides text message operations with broadcast-first semantics.
// userByName looks up a user by name.
// Satisfied by *repository.UserRepository.
type userByName interface {
	GetByName(name string) (*model.User, error)
	GetByNames(names []string) (map[string]*model.User, error)
}

// MessageService provides text message operations with broadcast-first semantics.

// MessageService provides text message operations with broadcast-first semantics.
type MessageService struct {
	msgRepo       *repository.MessageRepository
	roomRepo      roomByUUID
	domainChecker DomainMemberChecker
	userRepo      userByName
	bus           MessageEventBus
	queue         MessageJobQueue
}

// NewMessageService creates a MessageService.
// Room repo is required; bus and queue are optional (set via setters).
// NewMessageService creates a MessageService.
// Room repo is required; bus and queue are optional (set via setters).
func NewMessageService(msgRepo *repository.MessageRepository, roomRepo roomByUUID, domainChecker DomainMemberChecker) *MessageService {
	return &MessageService{
		msgRepo:       msgRepo,
		roomRepo:      roomRepo,
		domainChecker: domainChecker,
	}
}

// SetEventBus sets the event bus for broadcasting to online clients.
// SetUserRepo sets the user repository for fetching author info.

// SetEventBus sets the event bus for broadcasting to online clients.
// SetUserRepo sets the user repository for fetching author info.
func (s *MessageService) SetUserRepo(repo userByName) {
	s.userRepo = repo
}

// enrichAuthorInfo fills AuthorName and AuthorAvatar for a batch of MessageDTOs.

// enrichAuthorInfo fills AuthorName and AuthorAvatar for a batch of MessageDTOs.
func (s *MessageService) enrichAuthorInfo(items []MessageDTO) {
	if s.userRepo == nil {
		return
	}
	// Collect unique author IDs
	authorIDs := make(map[string]struct{})
	for _, item := range items {
		if item.AuthorID != "" {
			authorIDs[item.AuthorID] = struct{}{}
		}
	}
	if len(authorIDs) == 0 {
		return
	}
	// Fetch users
	names := make([]string, 0, len(authorIDs))
	for id := range authorIDs {
		names = append(names, id)
	}
	var users map[string]*model.User
	if got, err := s.userRepo.GetByNames(names); err == nil {
		users = got
	}
	// Enrich DTOs
	for i := range items {
		if user, ok := users[items[i].AuthorID]; ok {
			items[i].AuthorName = user.DisplayName
			if items[i].AuthorName == "" {
				items[i].AuthorName = user.Name
			}
			items[i].AuthorAvatar = user.Avatar
		}
	}
}
func (s *MessageService) SetEventBus(b MessageEventBus) {
	s.bus = b
}

// SetJobQueue sets the job queue for async persistence.

// SetJobQueue sets the job queue for async persistence.
func (s *MessageService) SetJobQueue(q MessageJobQueue) {
	s.queue = q
}

func (s *MessageService) requireDomainMembership(room *model.Room, actor MessageActor) error {
	if room.DomainUUID == "" {
		return nil
	}
	if actor.UserUUID == "" {
		return pkg.NewAppError(pkg.TOKEN_WRONG, "user_uuid is required")
	}
	if s.domainChecker == nil || !s.domainChecker.IsMember(room.DomainUUID, actor.UserUUID) {
		return pkg.NewAppError(pkg.FORBIDDEN, "not a member of this domain")
	}
	return nil
}

// broadcastRoomKey constructs the domain-scoped composite room key used by the
// signal layer's fanout.  This mirrors signal.roomKey so that broadcasts
// actually reach clients joined via Hub.OnRoomJoin.

// broadcastRoomKey constructs the domain-scoped composite room key used by the
// signal layer's fanout.  This mirrors signal.roomKey so that broadcasts
// actually reach clients joined via Hub.OnRoomJoin.
func broadcastRoomKey(domainUUID, roomName string) string {
	if domainUUID == "" {
		return roomName
	}
	return domainUUID + ":" + roomName
}

// Send creates and broadcasts a new text message, then enqueues async persist.
// Returns the MessageDTO on success.

// Send creates and broadcasts a new text message, then enqueues async persist.
// Returns the MessageDTO on success.
func (s *MessageService) Send(roomUUID string, actor MessageActor, content, replyTo, clientNonce string, mentions []string) (*MessageDTO, error) {
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
	if err := s.requireDomainMembership(room, actor); err != nil {
		return nil, err
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
		AuthorID:    actor.Identity,
		AuthorUUID:  actor.UserUUID,
		Content:     content,
		ReplyTo:     replyTo,
		Mentions:    mentions,
		Deleted:     false,
		CreatedAt:   now,
		ClientNonce: clientNonce,
	}

	// 1) broadcast first；广播失败时立即同步落库，避免消息仅存在于易失广播面。
	if s.bus != nil {
		if err := s.bus.PublishRoom(context.Background(), broadcastRoomKey(room.DomainUUID, room.Name), "message:created", dto); err != nil {
			if perr := s.persistMessageSync(msgUUID, roomUUID, actor.Identity, actor.UserUUID, content, replyTo, mentions, now); perr != nil {
				return nil, perr
			}
			return dto, nil
		}
	}

	// 2) enqueue persist; fall back to sync DB write on failure
	payload, _ := json.Marshal(map[string]interface{}{
		"uuid":         msgUUID,
		"room_uuid":    roomUUID,
		"author_id":    actor.Identity,
		"author_uuid":  actor.UserUUID,
		"content":      content,
		"reply_to":     replyTo,
		"mentions":     mentions,
		"created_at":   now,
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
		if err := s.persistMessageSync(msgUUID, roomUUID, actor.Identity, actor.UserUUID, content, replyTo, mentions, now); err != nil {
			return nil, err
		}
	}
	return dto, nil
}

// persistMessageSync 同步落库一条消息及其 @提及关系，供广播/队列不可用时的兜底路径使用。
func (s *MessageService) persistMessageSync(msgUUID, roomUUID, authorID, authorUUID, content, replyTo string, mentions []string, now time.Time) error {
	m := &model.Message{
		UUID: msgUUID, RoomUUID: roomUUID, AuthorID: authorID,
		AuthorUUID: authorUUID, Content: content, ReplyTo: replyTo,
		ConversationType: model.ConversationTypeRoom, Status: model.MessageStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.msgRepo.Create(m); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if len(mentions) > 0 {
		rows := make([]model.MessageMention, 0, len(mentions))
		for _, uid := range mentions {
			rows = append(rows, model.MessageMention{MessageUUID: msgUUID, UserID: uid})
		}
		if err := s.msgRepo.EnsureMentions(rows); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

// Edit updates a message's content, broadcasts the change, then enqueues async mutate.
// Only the original author may edit.
