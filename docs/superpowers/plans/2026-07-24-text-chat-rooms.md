# Text Chat Rooms Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship Discord-style text rooms on flat `Room.type`, with broadcast-first async-persist messages, dual text+voice slots, vertical split UI, and virtual-scroll history.

**Architecture:** Extend existing `Room` with `type=text|voice`. New `Message` / `MessageReaction` / `MessageMention` tables. `MessageService` validates, assigns UUID, **broadcasts via EventBus first**, then enqueues `chat.persist` / `chat.mutate` on `bus.JobQueue` (sync DB fallback if queue down). Hub dual slots (`textRoom`/`voiceRoom`); text rooms reject `room:join:sfu`. Frontend: `chatStore` + `components/textRoom/*` + main-area vertical Split.

**Tech Stack:** Go 1.24+ (Gin, GORM, Socket.IO, NATS JobQueue), SolidJS, TypeScript, Vite, `@tanstack/solid-virtual`, Vitest, go test

**Spec:** [`docs/superpowers/specs/2026-07-24-text-chat-rooms-design.md`](../specs/2026-07-24-text-chat-rooms-design.md)

## Global Constraints

- Layering: `model → repository → service → handler → router` only downward.
- Service returns `*pkg.AppError`; Handler uses `pkg.HandleError`.
- Response shape `{ code, msg, data }`; errors `data: null`.
- No unnecessary code comments; no emoji in code (docs OK).
- Message content max **2000 rune**; history limit default **100**, clamp **50–200**.
- Broadcast **before** DB persist; JetStream down → sync write.
- Dual slot: max 1 text + 1 voice per connection.
- UI: upper = voice, lower = text, draggable split.
- Conventional Commits; commit after each task.
- Do not reuse `components/chat/micControl|speakerControl` for text UI — put text under `components/textRoom/`.

---

## File Map

### Create

| Path | Responsibility |
|------|----------------|
| `app/server/internal/model/message.go` | Message entity |
| `app/server/internal/model/message_reaction.go` | Reaction entity |
| `app/server/internal/model/message_mention.go` | Mention entity |
| `app/server/internal/repository/message_repo.go` | Message CRUD + cursor list + reactions |
| `app/server/internal/service/message_service.go` | Validate, broadcast-first, enqueue |
| `app/server/internal/service/message_service_test.go` | Unit tests with mocks |
| `app/server/internal/handler/message_handler.go` | REST endpoints |
| `app/server/internal/router/routes/message/routes.go` | Route registration |
| `app/server/internal/signal/message_bridge.go` | Socket message handlers |
| `app/server/internal/signal/message_bridge_test.go` | Hub message tests |
| `app/server/internal/jobs/chat.go` | `chat.persist` / `chat.mutate` handlers |
| `app/web/src/api/message.ts` | REST client |
| `app/web/src/types/message.ts` | Message DTOs |
| `app/web/src/stores/chatStore.ts` | textSlot + buffer + merge |
| `app/web/src/stores/chatStore.test.ts` | merge/nonce tests |
| `app/web/src/components/textRoom/TextRoomPanel.tsx` | Shell |
| `app/web/src/components/textRoom/MessageList.tsx` | Virtual list |
| `app/web/src/components/textRoom/MessageItem.tsx` | Row |
| `app/web/src/components/textRoom/MessageInput.tsx` | Composer |
| `app/web/src/components/textRoom/ReactionBar.tsx` | Reactions |

### Modify

| Path | Change |
|------|--------|
| `app/server/internal/model/room.go` | Add `Type` |
| `app/server/internal/model/permission.go` | Seed message perms + role defaults |
| `app/server/internal/permcode/permcode.go` | `message:send/read/delete_others` |
| `app/server/internal/repository/db.go` | AutoMigrate new models |
| `app/server/internal/repository/room_repo.go` | List filter by type |
| `app/server/internal/service/room_service.go` | CreateRoom type; List type filter |
| `app/server/internal/handler/room_handler.go` | Create/List accept type |
| `app/server/internal/signal/events.go` | `message:*` constants |
| `app/server/internal/signal/hub.go` | Dual slots; text SFU reject; register handlers |
| `app/server/internal/signal/types.go` | RoomInfo.Type if needed |
| `app/server/internal/jobs/handlers.go` | Dispatch chat jobs |
| `app/server/server/gin.go` | DI MessageService/Handler; wire jobs |
| `app/server/internal/router/router.go` | Register message routes |
| `app/web/src/types/room.ts` | `type` field |
| `app/web/src/api/room.ts` | Create/list type |
| `app/web/src/socket/events.ts` | `message:*` |
| `app/web/src/stores/socketStore.ts` | voice-slot awareness (minimal) |
| `app/web/src/layouts/layout.tsx` | Vertical split + TextRoomPanel |
| `app/web/src/components/room/roomList.tsx` | Type badge / filter (if present) |
| `app/web/package.json` | Add `@tanstack/solid-virtual` |

---

### Task 1: Room.Type + message permcode seeds

**Files:**
- Modify: `app/server/internal/model/room.go`
- Modify: `app/server/internal/permcode/permcode.go`
- Modify: `app/server/internal/model/permission.go`
- Modify: `app/server/internal/service/room_service.go`
- Modify: `app/server/internal/handler/room_handler.go`
- Modify: `app/server/internal/repository/room_repo.go`
- Test: `app/server/internal/service/room_type_test.go` (create)

**Interfaces:**
- Consumes: existing Room CRUD
- Produces: `Room.Type` string `"text"|"voice"`; `CreateRoom(..., roomType string)`; `List(page, pageSize int, roomType string)`; permcodes `message:send|read|delete_others`

- [ ] **Step 1: Add Type to Room model**

In `app/server/internal/model/room.go`, add field after `AllowAudience`:

```go
Type string `gorm:"size:16;not null;default:voice;index" json:"type"`
```

Add helpers in same file:

```go
const (
	RoomTypeText  = "text"
	RoomTypeVoice = "voice"
)

func NormalizeRoomType(t string) string {
	switch t {
	case RoomTypeText:
		return RoomTypeText
	default:
		return RoomTypeVoice
	}
}
```

- [ ] **Step 2: Add permcode constants**

In `app/server/internal/permcode/permcode.go` after room perms:

```go
PermMessageSend         = "message:send"
PermMessageRead         = "message:read"
PermMessageDeleteOthers = "message:delete_others"
```

- [ ] **Step 3: Seed permissions**

In `app/server/internal/model/permission.go`:

1. Add aliases:
```go
PermMessageSend         = permcode.PermMessageSend
PermMessageRead         = permcode.PermMessageRead
PermMessageDeleteOthers = permcode.PermMessageDeleteOthers
```

2. Append to `DefaultPermissions`:
```go
{Code: PermMessageSend, Name: "发送消息", Description: "在文字房间发送消息"},
{Code: PermMessageRead, Name: "查看消息", Description: "查看文字房间历史消息"},
{Code: PermMessageDeleteOthers, Name: "删除他人消息", Description: "删除其他用户的消息"},
```

3. Admin role: append three message perms.  
4. User role: append `PermMessageSend`, `PermMessageRead`.  
5. BotScopedPermissions: append `PermMessageSend`, `PermMessageRead` (optional but useful).

Note: existing DBs use seed-if-empty; if perms already seeded, document that admin must assign new codes manually OR add a one-shot sync in permission repo if project already has SyncPermissions pattern. Prefer calling existing permission sync on boot if present (`SyncPermissions` / seed all DefaultPermissions upsert by code). If only seed-if-empty exists, add upsert-by-code for new permission rows only (do not wipe role_permissions).

- [ ] **Step 4: Room repo List filter**

Change signature:

```go
func (r *RoomRepository) List(page, pageSize int, roomType string) ([]model.Room, int64, error) {
	var rooms []model.Room
	var total int64
	q := r.db.Model(&model.Room{})
	if roomType == model.RoomTypeText || roomType == model.RoomTypeVoice {
		q = q.Where("type = ?", roomType)
	}
	q.Count(&total)
	err := q.Order("created_at ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rooms).Error
	return rooms, total, err
}
```

Update all callers of `List` (room service, hub `roomStore` interface, tests) to pass `""` for all types.

- [ ] **Step 5: RoomService CreateRoom + List**

```go
func (s *RoomService) CreateRoom(name, password, description string, limit uint, audioOnly, allowAudience bool, createdBy, roomType string) (*model.Room, error) {
	room := &model.Room{
		Name:          name,
		Password:      password,
		Description:   description,
		Limit:         limit,
		AudioOnly:     audioOnly,
		AllowAudience: allowAudience,
		CreatedBy:     createdBy,
		Type:          model.NormalizeRoomType(roomType),
	}
	// text rooms: force audio_only true / no SFU expectations
	if room.Type == model.RoomTypeText {
		room.AudioOnly = true
	}
	if err := s.roomRepo.Create(room); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return room, nil
}

func (s *RoomService) List(page, pageSize int, roomType string) ([]model.Room, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if roomType != "" && roomType != model.RoomTypeText && roomType != model.RoomTypeVoice {
		return nil, 0, pkg.NewAppError(pkg.INVALID_PARAMS, "type must be text, voice, or empty")
	}
	rooms, total, err := s.roomRepo.List(page, pageSize, roomType)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return rooms, total, nil
}
```

Update handler Create/List request bodies to include `type` / `Type string \`json:"type"\``.

- [ ] **Step 6: Write type normalization test**

Create `app/server/internal/model/room_type_test.go`:

```go
package model

import "testing"

func TestNormalizeRoomType(t *testing.T) {
	if NormalizeRoomType("text") != RoomTypeText {
		t.Fatal("text")
	}
	if NormalizeRoomType("") != RoomTypeVoice {
		t.Fatal("empty -> voice")
	}
	if NormalizeRoomType("weird") != RoomTypeVoice {
		t.Fatal("weird -> voice")
	}
}
```

- [ ] **Step 7: Run test**

```bash
cd app/server && go test ./internal/model/ -run TestNormalizeRoomType -count=1
```

Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add app/server/internal/model/room.go app/server/internal/model/room_type_test.go \
  app/server/internal/permcode/permcode.go app/server/internal/model/permission.go \
  app/server/internal/repository/room_repo.go app/server/internal/service/room_service.go \
  app/server/internal/handler/room_handler.go
# also any hub roomStore signature fixes
git commit -m "feat(room): add type text|voice and message permcodes"
```

---

### Task 2: Message models + AutoMigrate

**Files:**
- Create: `app/server/internal/model/message.go`
- Create: `app/server/internal/model/message_reaction.go`
- Create: `app/server/internal/model/message_mention.go`
- Modify: `app/server/internal/repository/db.go` (`autoMigrate`)

**Interfaces:**
- Produces: `model.Message`, `model.MessageReaction`, `model.MessageMention` with TableName

- [ ] **Step 1: Create message.go**

```go
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Message struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UUID      string         `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	RoomUUID  string         `gorm:"size:36;index:idx_msg_room_created,priority:1;not null" json:"room_uuid"`
	AuthorID  string         `gorm:"size:64;index;not null" json:"author_id"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	ReplyTo   string         `gorm:"size:36;index" json:"reply_to,omitempty"`
	EditedAt  *time.Time     `json:"edited_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedAt time.Time      `gorm:"index:idx_msg_room_created,priority:2" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (m *Message) BeforeCreate(_ *gorm.DB) error {
	if m.UUID == "" {
		m.UUID = uuid.New().String()
	}
	return nil
}

func (m *Message) TableName() string { return "messages" }
```

- [ ] **Step 2: Create message_reaction.go**

```go
package model

import "time"

type MessageReaction struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	MessageUUID string    `gorm:"size:36;uniqueIndex:idx_react_unique,priority:1;not null" json:"message_uuid"`
	UserID      string    `gorm:"size:64;uniqueIndex:idx_react_unique,priority:2;not null" json:"user_id"`
	Emoji       string    `gorm:"size:32;uniqueIndex:idx_react_unique,priority:3;not null" json:"emoji"`
	CreatedAt   time.Time `json:"created_at"`
}

func (MessageReaction) TableName() string { return "message_reactions" }
```

- [ ] **Step 3: Create message_mention.go**

```go
package model

type MessageMention struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	MessageUUID string `gorm:"size:36;index;not null" json:"message_uuid"`
	UserID      string `gorm:"size:64;index;not null" json:"user_id"`
}

func (MessageMention) TableName() string { return "message_mentions" }
```

- [ ] **Step 4: AutoMigrate**

In `autoMigrate()` append:

```go
&model.Message{},
&model.MessageReaction{},
&model.MessageMention{},
```

- [ ] **Step 5: Compile check**

```bash
cd app/server && go build ./internal/model/ ./internal/repository/
```

Expected: success

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/model/message.go \
  app/server/internal/model/message_reaction.go \
  app/server/internal/model/message_mention.go \
  app/server/internal/repository/db.go
git commit -m "feat(message): add message models and auto-migrate"
```

---

### Task 3: Message repository (cursor history)

**Files:**
- Create: `app/server/internal/repository/message_repo.go`
- Create: `app/server/internal/repository/message_repo_test.go`

**Interfaces:**
- Produces:
  - `NewMessageRepository(db *gorm.DB) *MessageRepository`
  - `Create(msg *model.Message) error`
  - `CreateMentions(mentions []model.MessageMention) error`
  - `GetByUUID(uuid string) (*model.Message, error)`
  - `UpdateContent(uuid, content string, editedAt time.Time) error`
  - `SoftDelete(uuid string) error`
  - `ListBefore(roomUUID, beforeUUID string, limit int) (items []model.Message, hasMore bool, err error)`
  - `AddReaction(r *model.MessageReaction) error`
  - `RemoveReaction(messageUUID, userID, emoji string) error`
  - `ListReactions(messageUUIDs []string) ([]model.MessageReaction, error)`

- [ ] **Step 1: Write failing repo test (sqlite in-memory)**

```go
package repository

import (
	"testing"
	"time"

	"GOSpeak/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupMsgDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:msg_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Message{}, &model.MessageReaction{}, &model.MessageMention{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMessageRepo_ListBefore(t *testing.T) {
	db := setupMsgDB(t)
	repo := NewMessageRepository(db)
	room := "room-1"
	for i := 0; i < 5; i++ {
		m := &model.Message{
			RoomUUID:  room,
			AuthorID:  "u1",
			Content:   "m",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := repo.Create(m); err != nil {
			t.Fatal(err)
		}
	}
	items, hasMore, err := repo.ListBefore(room, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 got %d", len(items))
	}
	if !hasMore {
		t.Fatal("want hasMore")
	}
	// items ASC: older first among the page of newest
	if items[0].CreatedAt.After(items[1].CreatedAt) {
		t.Fatal("want ASC order in returned page")
	}
	next, hasMore2, err := repo.ListBefore(room, items[0].UUID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) == 0 {
		t.Fatal("want older page")
	}
	_ = hasMore2
}
```

- [ ] **Step 2: Run test — expect fail**

```bash
cd app/server && go test ./internal/repository/ -run TestMessageRepo_ListBefore -count=1
```

Expected: FAIL (undefined NewMessageRepository)

- [ ] **Step 3: Implement message_repo.go**

```go
package repository

import (
	"GOSpeak/internal/model"
	"time"

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

func (r *MessageRepository) CreateMentions(mentions []model.MessageMention) error {
	if len(mentions) == 0 {
		return nil
	}
	return r.db.Create(&mentions).Error
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
	return r.db.Where("uuid = ?", uuid).Delete(&model.Message{}).Error
}

// ListBefore returns up to limit messages older than beforeUUID (exclusive), ASC.
// beforeUUID empty = newest page. hasMore true if more older rows exist.
func (r *MessageRepository) ListBefore(roomUUID, beforeUUID string, limit int) ([]model.Message, bool, error) {
	if limit < 1 {
		limit = 100
	}
	q := r.db.Model(&model.Message{}).Where("room_uuid = ?", roomUUID)
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
```

- [ ] **Step 4: Run test — expect pass**

```bash
cd app/server && go test ./internal/repository/ -run TestMessageRepo_ListBefore -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/repository/message_repo.go app/server/internal/repository/message_repo_test.go
git commit -m "feat(message): repository with cursor ListBefore"
```

---

### Task 4: MessageService (broadcast-first + job queue)

**Files:**
- Create: `app/server/internal/service/message_service.go`
- Create: `app/server/internal/service/message_service_test.go`

**Interfaces:**
- Consumes: `MessageRepository`, `RoomRepository` (GetByUUID), optional `eventBus`, optional `jobPublisher`
- Produces:
```go
type MessageDTO struct {
	UUID      string     `json:"uuid"`
	RoomUUID  string     `json:"room_uuid"`
	AuthorID  string     `json:"author_id"`
	Content   string     `json:"content"`
	ReplyTo   string     `json:"reply_to,omitempty"`
	Mentions  []string   `json:"mentions,omitempty"`
	EditedAt  *time.Time `json:"edited_at,omitempty"`
	Deleted   bool       `json:"deleted"`
	CreatedAt time.Time  `json:"created_at"`
	ClientNonce string   `json:"client_nonce,omitempty"`
}

type MessageService struct { /* ... */ }

func NewMessageService(msgRepo *repository.MessageRepository, roomRepo *repository.RoomRepository) *MessageService
func (s *MessageService) SetEventBus(b MessageEventBus)
func (s *MessageService) SetJobQueue(q MessageJobQueue)
func (s *MessageService) Send(roomUUID, authorID, content, replyTo, clientNonce string, mentions []string) (*MessageDTO, error)
func (s *MessageService) Edit(roomUUID, messageUUID, authorID, content string) (*MessageDTO, error)
func (s *MessageService) Delete(roomUUID, messageUUID, actorID string, canDeleteOthers bool) error
func (s *MessageService) React(roomUUID, messageUUID, userID, emoji string) error
func (s *MessageService) Unreact(roomUUID, messageUUID, userID, emoji string) error
func (s *MessageService) ListHistory(roomUUID, before string, limit int) (items []MessageDTO, hasMore bool, nextBefore string, err error)
func (s *MessageService) PersistFromJob(payload []byte) error
func (s *MessageService) MutateFromJob(payload []byte) error
```

Narrow interfaces for testability:

```go
type MessageEventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}

type MessageJobQueue interface {
	Publish(ctx context.Context, job bus.JobEnvelope) error
}
```

- [ ] **Step 1: Write failing unit test for Send broadcast-then-queue**

```go
package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/signal"
)

type fakeBus struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeBus) PublishRoom(ctx context.Context, room, event string, payload interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, event)
	return nil
}

type fakeQueue struct {
	mu   sync.Mutex
	jobs []bus.JobEnvelope
	err  error
}

func (q *fakeQueue) Publish(ctx context.Context, job bus.JobEnvelope) error {
	if q.err != nil {
		return q.err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, job)
	return nil
}

type memRoomRepo struct {
	rooms map[string]*model.Room
}

// implement only GetByUUID used by service — adapt if service takes interface
```

Prefer defining small interfaces inside message_service.go for room lookup:

```go
type roomByUUID interface {
	GetByUUID(uuid string) (*model.Room, error)
}
```

Inject via constructor. Test: Send on text room → bus gets `signal.EventMessageCreated` (define constant in Task 5 first or use string `"message:created"`), queue gets type `chat.persist`. Content > 2000 runes → INVALID_PARAMS. Voice room → INVALID_PARAMS/FORBIDDEN.

- [ ] **Step 2: Run test — expect fail**

```bash
cd app/server && go test ./internal/service/ -run TestMessageService_Send -count=1
```

Expected: FAIL

- [ ] **Step 3: Implement MessageService core**

Key logic in `Send`:

```go
const MaxMessageRunes = 2000

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
	// 1) broadcast first (room name for socket is room.Name — Hub rooms keyed by name)
	// Spec uses room uuid in DB; Hub BroadcastToRoom uses room **name**.
	// Store both: publish to room.Name for socket fanout.
	if s.bus != nil {
		_ = s.bus.PublishRoom(context.Background(), room.Name, "message:created", dto)
	}
	// 2) enqueue persist
	payload, _ := json.Marshal(map[string]interface{}{
		"uuid": msgUUID, "room_uuid": roomUUID, "author_id": authorID,
		"content": content, "reply_to": replyTo, "mentions": mentions,
		"created_at": now,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID: msgUUID, Type: "chat.persist", Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		// sync fallback
		m := &model.Message{
			UUID: msgUUID, RoomUUID: roomUUID, AuthorID: authorID,
			Content: content, ReplyTo: replyTo, CreatedAt: now, UpdatedAt: now,
		}
		if err := s.msgRepo.Create(m); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		var rows []model.MessageMention
		for _, uid := range mentions {
			rows = append(rows, model.MessageMention{MessageUUID: msgUUID, UserID: uid})
		}
		_ = s.msgRepo.CreateMentions(rows)
	}
	return dto, nil
}
```

Implement Edit/Delete/React similarly (broadcast then `chat.mutate`).

`ListHistory`: clamp limit 50–200 default 100; map soft-deleted to `Deleted:true, Content:""`; compute `nextBefore` from first item uuid when hasMore.

`PersistFromJob` / `MutateFromJob`: used by jobs package.

- [ ] **Step 4: Run tests**

```bash
cd app/server && go test ./internal/service/ -run TestMessageService_ -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/service/message_service.go app/server/internal/service/message_service_test.go
git commit -m "feat(message): service broadcast-first with async persist"
```

---

### Task 5: Jobs handlers for chat.persist / chat.mutate

**Files:**
- Create: `app/server/internal/jobs/chat.go`
- Modify: `app/server/internal/jobs/handlers.go`
- Modify: `app/server/server/gin.go` (wire MessageService into job Handle)

**Interfaces:**
- Consumes: `MessageService.PersistFromJob` / `MutateFromJob`
- Produces: job types handled in `jobs.Handle`

- [ ] **Step 1: Extend Handle signature**

```go
type ChatPersister interface {
	PersistFromJob(payload []byte) error
	MutateFromJob(payload []byte) error
}

func Handle(job bus.JobEnvelope, hub StreamRegistrar, cleaner SFUCleaner, chat ChatPersister) error {
	switch job.Type {
	case "srs":
		return handleSRS(job.Payload, hub)
	case "livekit":
		return handleLiveKit(job.Payload, cleaner)
	case "sfu_cleanup":
		return handleCleanup(job.Payload, cleaner)
	case "chat.persist":
		if chat == nil {
			return nil
		}
		return chat.PersistFromJob(job.Payload)
	case "chat.mutate":
		if chat == nil {
			return nil
		}
		return chat.MutateFromJob(job.Payload)
	default:
		log.Printf("[Jobs] ignore unknown type=%s", job.Type)
		return nil
	}
}
```

- [ ] **Step 2: Mutate retry semantics**

In `MutateFromJob`, if target message missing return `fmt.Errorf("message not ready")` so JetStream Nak retries. Cap is NATS redelivery; no infinite custom loop.

- [ ] **Step 3: Wire gin.go**

Construct `messageRepo`, `messageSvc` early; pass `messageSvc` into `jobs.Handle`:

```go
return jobs.Handle(job, signalHub, signalHub, messageSvc)
```

Update all `jobs.Handle` call sites/tests.

- [ ] **Step 4: Compile**

```bash
cd app/server && go build ./internal/jobs/ ./server/
```

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/jobs/ app/server/server/gin.go
git commit -m "feat(jobs): handle chat.persist and chat.mutate"
```

---

### Task 6: REST handler + routes + DI

**Files:**
- Create: `app/server/internal/handler/message_handler.go`
- Create: `app/server/internal/router/routes/message/routes.go`
- Modify: `app/server/internal/router/router.go`
- Modify: `app/server/server/gin.go`

**Interfaces:**
- REST under protected `/api/v1/room` or `/api/v1/message` — **use nested under room** to match design:

| Method | Path | Perm |
|--------|------|------|
| GET-style POST | `/room/messages/list` | message:read |
| POST | `/room/messages/send` | message:send |
| POST | `/room/messages/edit` | message:send |
| POST | `/room/messages/delete` | message:send (auth checks delete_others inside) |
| POST | `/room/messages/react` | message:send |
| POST | `/room/messages/unreact` | message:send |

Project uses POST-heavy RPC style (`/room/list` not GET). **Follow existing POST style**, not design's GET path literally.

- [ ] **Step 1: message_handler.go**

```go
package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	msgSvc  *service.MessageService
	permSvc *service.PermissionService
}

func NewMessageHandler(msgSvc *service.MessageService, permSvc *service.PermissionService) *MessageHandler {
	return &MessageHandler{msgSvc: msgSvc, permSvc: permSvc}
}

func (h *MessageHandler) List(c *gin.Context) {
	var req struct {
		RoomUUID string `json:"room_uuid" binding:"required"`
		Before   string `json:"before"`
		Limit    int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	items, hasMore, nextBefore, err := h.msgSvc.ListHistory(req.RoomUUID, req.Before, req.Limit)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items, "has_more": hasMore, "next_before": nextBefore})
}

func (h *MessageHandler) Send(c *gin.Context) {
	var req struct {
		RoomUUID    string   `json:"room_uuid" binding:"required"`
		Content     string   `json:"content" binding:"required"`
		ReplyTo     string   `json:"reply_to"`
		Mentions    []string `json:"mentions"`
		ClientNonce string   `json:"client_nonce"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	// author = user uuid from context — project currently puts username in claims.
	// Use claims.Username as AuthorID for consistency with Hub identity (username).
	username, _ := c.Get("username")
	dto, err := h.msgSvc.Send(req.RoomUUID, username.(string), req.Content, req.ReplyTo, req.ClientNonce, req.Mentions)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, dto)
}

// Edit, Delete, React, Unreact similarly.
// Delete: canDeleteOthers := h.permSvc.HasPermission(role, permcode.PermMessageDeleteOthers)
```

- [ ] **Step 2: routes**

`app/server/internal/router/routes/message/routes.go`:

```go
package message

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"
	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.MessageHandler) {
	r.POST("/messages/list", middleware.RequirePermission(permcode.PermMessageRead), h.List)
	r.POST("/messages/send", middleware.RequirePermission(permcode.PermMessageSend), h.Send)
	r.POST("/messages/edit", middleware.RequirePermission(permcode.PermMessageSend), h.Edit)
	r.POST("/messages/delete", middleware.RequirePermission(permcode.PermMessageSend), h.Delete)
	r.POST("/messages/react", middleware.RequirePermission(permcode.PermMessageSend), h.React)
	r.POST("/messages/unreact", middleware.RequirePermission(permcode.PermMessageSend), h.Unreact)
}
```

Register on `protected.Group("/room")` **or** separate group — if room group already registered, either extend `room/routes.go` or call message routes with same prefix:

```go
messageRoutes.RegisterProtected(protected.Group("/room"), h.Message)
```

Add `Message *handler.MessageHandler` to `router.Handlers`.

- [ ] **Step 3: DI in gin.go**

```go
msgRepo := repository.NewMessageRepository(repository.DB)
msgSvc := service.NewMessageService(msgRepo, roomRepo)
if eventBus != nil { msgSvc.SetEventBus(eventBus) }
if jobQueue != nil { msgSvc.SetJobQueue(jobQueue) }
msgH := handler.NewMessageHandler(msgSvc, permSvc)
// handlers.Message = msgH
```

- [ ] **Step 4: Manual smoke (optional if server runnable)**

```bash
# with server up + JWT
curl -s -X POST localhost:8998/api/v1/room/messages/list \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"room_uuid":"...","limit":100}'
```

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/handler/message_handler.go \
  app/server/internal/router/routes/message/routes.go \
  app/server/internal/router/router.go app/server/server/gin.go
git commit -m "feat(message): REST list/send/edit/delete/react"
```

---

### Task 7: Socket events + dual-slot Hub + message bridge

**Files:**
- Modify: `app/server/internal/signal/events.go`
- Create: `app/server/internal/signal/message_bridge.go`
- Create: `app/server/internal/signal/message_bridge_test.go`
- Modify: `app/server/internal/signal/hub.go`
- Modify: `app/server/internal/signal/types.go` (RoomInfo Type)

**Interfaces:**
- Consumes: `MessageService` via Hub setter
- Produces: socket handlers for `message:send|edit|delete|react|unreact`
- Dual slot on conn context

- [ ] **Step 1: Events**

```go
EventMessageSend     = "message:send"
EventMessageEdit     = "message:edit"
EventMessageDelete   = "message:delete"
EventMessageReact    = "message:react"
EventMessageUnreact  = "message:unreact"
EventMessageCreated  = "message:created"
EventMessageUpdated  = "message:updated"
EventMessageDeleted  = "message:deleted"
EventMessageReaction = "message:reaction"
EventMessageAck      = "message:ack"
EventMessageError    = "message:error"
```

- [ ] **Step 2: Conn dual-slot state**

Define in hub or types:

```go
type connRoomSlots struct {
	TextRoom  string
	VoiceRoom string
}
```

Store on socket context alongside claims, OR map `socketID -> connRoomSlots` on Hub with mutex.

On `OnRoomJoin`:
1. Load room from DB by name → read Type.
2. If text: leave previous text slot room (socket Leave + clear), set TextRoom.
3. If voice: leave previous voice slot, set VoiceRoom.
4. Existing password/mute/limit checks remain.
5. For **text** rooms: after join, also write membership like SFU path OR keep members only after a lightweight "present" — **minimal**: for text, call same member-write path as join SFU (extract shared `addMember`) so `member:joined` works without media.

On `OnRoomJoinSFU`:
```go
if room.Type == model.RoomTypeText {
  return marshalAck(map[string]interface{}{"error": "text room has no media"})
}
```

Need room type lookup: extend `roomStore` interface:

```go
type roomStore interface {
	List(page, pageSize int, roomType string) ([]model.Room, int64, error)
	GetByName(name string) (*model.Room, error)
}
```

- [ ] **Step 3: message_bridge.go**

```go
func (h *Hub) SetMessageService(svc messageSender) { h.msgSvc = svc }

type messageSender interface {
	Send(roomUUID, authorID, content, replyTo, clientNonce string, mentions []string) (*service.MessageDTO, error)
	// Edit, Delete, React, Unreact...
}

func (h *Hub) OnMessageSend(s socketio.Conn, data string) {
	// parse {room, content, reply_to, mentions, client_nonce}
	// room is name; resolve uuid via roomStore.GetByName
	// require claimsIdentity in room (socket rooms membership)
	// require text slot
	// call msgSvc.Send
	// on error: emit message:error to s only
	// on success: message:created already broadcast by service; optionally s.Emit message:ack
}
```

Register in SetupRoutes.

**Important:** Service broadcasts by `room.Name`. Hub membership uses name. REST uses room UUID. Bridge maps name→uuid.

- [ ] **Step 4: Tests**

- Text room join SFU → error string contains `no media`
- Message send not in room → no broadcast / error
- Dual slot: join text A, join voice B, both slots set; join text C replaces A only

Use existing hub test patterns from `hub_test.go` / `bot_bridge_test.go`.

- [ ] **Step 5: Run**

```bash
cd app/server && go test ./internal/signal/ -count=1
```

Expected: PASS (fix any broken roomStore List signature tests)

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/signal/
git commit -m "feat(signal): message events and dual text/voice slots"
```

---

### Task 8: Frontend types, API, socket events

**Files:**
- Create: `app/web/src/types/message.ts`
- Create: `app/web/src/api/message.ts`
- Modify: `app/web/src/types/room.ts`
- Modify: `app/web/src/api/room.ts`
- Modify: `app/web/src/socket/events.ts`

- [ ] **Step 1: types/message.ts**

```ts
export type MessageDTO = {
  uuid: string;
  room_uuid: string;
  author_id: string;
  content: string;
  reply_to?: string;
  mentions?: string[];
  edited_at?: string | null;
  deleted: boolean;
  created_at: string;
  client_nonce?: string;
};

export type MessageListResult = {
  items: MessageDTO[];
  has_more: boolean;
  next_before: string;
};
```

- [ ] **Step 2: api/message.ts**

```ts
import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";
import type { MessageDTO, MessageListResult } from "@/types/message";

export async function listMessages(room_uuid: string, before?: string, limit = 100) {
  const res = (await apiClient.post({
    url: "/api/v1/room/messages/list",
    data: { room_uuid, before, limit },
  })) as AxiosResponse<Result<MessageListResult>>;
  if (!(res as any).data.data) throw new Error("messages missing");
  return (res as any).data.data as MessageListResult;
}

export async function sendMessage(body: {
  room_uuid: string;
  content: string;
  reply_to?: string;
  mentions?: string[];
  client_nonce?: string;
}) {
  const res = (await apiClient.post({
    url: "/api/v1/room/messages/send",
    data: body,
  })) as AxiosResponse<Result<MessageDTO>>;
  if (!(res as any).data.data) throw new Error("send failed");
  return (res as any).data.data as MessageDTO;
}
// editMessage, deleteMessage, reactMessage, unreactMessage similarly
```

- [ ] **Step 3: Room type field**

Add `type?: "text" | "voice"` to `RoomInfo`, `RoomRecord`, `CreateRoomReq`.

- [ ] **Step 4: events.ts**

```ts
MESSAGE_SEND: "message:send",
MESSAGE_EDIT: "message:edit",
MESSAGE_DELETE: "message:delete",
MESSAGE_REACT: "message:react",
MESSAGE_UNREACT: "message:unreact",
MESSAGE_CREATED: "message:created",
MESSAGE_UPDATED: "message:updated",
MESSAGE_DELETED: "message:deleted",
MESSAGE_REACTION: "message:reaction",
MESSAGE_ACK: "message:ack",
MESSAGE_ERROR: "message:error",
```

- [ ] **Step 5: Commit**

```bash
git add app/web/src/types app/web/src/api app/web/src/socket/events.ts
git commit -m "feat(web): message API types and socket events"
```

---

### Task 9: chatStore (merge, nonce, cursor)

**Files:**
- Create: `app/web/src/stores/chatStore.ts`
- Create: `app/web/src/stores/chatStore.test.ts`

**Interfaces:**
```ts
// exports
textRoom: Accessor<string | null> // room name or uuid — pick uuid for API, name for socket
joinTextRoom(room: { uuid: string; name: string }): Promise<void>
leaveTextRoom(): void
messages: Accessor<MessageDTO[]> // merged sorted ASC
loadInitial(): Promise<void>
loadMore(): Promise<void>
send(content: string, opts?: { reply_to?: string; mentions?: string[] }): void
applyCreated(dto: MessageDTO): void
applyUpdated(...): void
applyDeleted(...): void
hasMore: Accessor<boolean>
isAtBottom: Accessor<boolean> / setAtBottom
```

- [ ] **Step 1: Write merge unit test**

```ts
import { describe, expect, it } from "vitest";
import { mergeMessages } from "./chatStore";

describe("mergeMessages", () => {
  it("dedupes by uuid and sorts by created_at", () => {
    const a = { uuid: "1", created_at: "2026-01-01T00:00:01Z", content: "a" } as any;
    const b = { uuid: "2", created_at: "2026-01-01T00:00:02Z", content: "b" } as any;
    const b2 = { uuid: "2", created_at: "2026-01-01T00:00:02Z", content: "b-edit" } as any;
    const out = mergeMessages([a, b], [b2]);
    expect(out.map((m) => m.uuid)).toEqual(["1", "2"]);
    expect(out[1].content).toBe("b-edit");
  });
});
```

Export pure `mergeMessages` for testability.

- [ ] **Step 2: Run fail then implement store**

```bash
cd app/web && pnpm test -- src/stores/chatStore.test.ts
```

Implement with solid signals. Buffer cap 1000: if length > 1000 and hasMore, drop oldest until 1000.

Send path:
1. `client_nonce = crypto.randomUUID()`
2. optimistic insert pending
3. `socket.emit(MESSAGE_SEND, payload)` (wire via socketStore socket instance — inject emit fn to avoid circular imports)
4. on MESSAGE_CREATED with same nonce: replace pending

Listen to socket events: register in chatStore init or layout effect once socket ready.

- [ ] **Step 3: Pass tests + commit**

```bash
cd app/web && pnpm test -- src/stores/chatStore.test.ts
git add app/web/src/stores/chatStore.ts app/web/src/stores/chatStore.test.ts
git commit -m "feat(web): chatStore with history/realtime merge"
```

---

### Task 10: Text room UI + virtual scroll

**Files:**
- Modify: `app/web/package.json` (add `@tanstack/solid-virtual`)
- Create: `app/web/src/components/textRoom/*.tsx`

- [ ] **Step 1: Install dependency**

```bash
cd app/web && pnpm add @tanstack/solid-virtual
```

- [ ] **Step 2: MessageList with virtualizer**

Use pattern:

```tsx
import { createVirtualizer } from "@tanstack/solid-virtual";
// parentRef scroll container
// virtualizer = createVirtualizer({ count: messages().length, getScrollElement: () => parentRef, estimateSize: () => 72, overscan: 12 })
// onScroll: if scrollTop < 50 && hasMore() => loadMore() + preserve scroll anchor
```

When prepending older page: save `scrollHeight` before, restore delta after.

- [ ] **Step 3: MessageItem / Input / ReactionBar / TextRoomPanel**

- Input: Enter send, Shift+Enter newline; show reply chip if replyTo set.
- Item: show edited badge if edited_at; deleted placeholder; hover actions.
- ReactionBar: chips; click toggles react/unreact via chatStore.

Keep styles with existing Tailwind / daisyUI classes (`bg-base-200`, `btn`, etc.).

- [ ] **Step 4: Commit**

```bash
git add app/web/package.json app/web/pnpm-lock.yaml app/web/src/components/textRoom
git commit -m "feat(web): text room panel with virtualized messages"
```

---

### Task 11: Layout vertical split + dual-slot UX

**Files:**
- Modify: `app/web/src/layouts/layout.tsx`
- Modify: `app/web/src/components/room/roomDetail.tsx` (optional trim)
- Modify: room list UI to show type + join text without SFU

- [ ] **Step 1: Main area structure**

Inside Main content (right of horizontal Split), use vertical flex + drag handle:

```tsx
const [splitHeight, setSplitHeight] = createSignal(
  localStorage.getItem("splitHeight") || "40%",
);
// upper: voice RoomDetail (or compact bar if no voice)
// handle: mousedown drag → setSplitHeight, persist
// lower: TextRoomPanel
```

If only text: upper min height thin bar. If only voice: lower placeholder "选择文字房间". If both: use splitHeight.

Alternatively nest second `Split` if `cui-solid` supports vertical — check `Split` API; if only horizontal, implement simple drag div.

- [ ] **Step 2: Join wiring**

Room list click:
- if `type === "text"` → `chatStore.joinTextRoom` (socket `room:join` only, **no** `joinRoomSFU` / useVoiceSession)
- if voice → existing voice session path

Ensure voice leave does not call text leave.

- [ ] **Step 3: Manual verification checklist**

1. Create text room via API/UI with `type: "text"`.
2. Create voice room (default).
3. Join both; UI shows upper voice + lower text.
4. Send message; appears for second client in same text room.
5. Scroll up loads older messages (seed ≥ 150 messages via script/API).
6. Edit/delete/react work live.
7. Text room join SFU fails cleanly.
8. JetStream off: message still persists (sync fallback) — check DB.

- [ ] **Step 4: Commit**

```bash
git add app/web/src/layouts/layout.tsx app/web/src/components/room app/web/src/stores
git commit -m "feat(web): dual-slot layout with vertical text/voice split"
```

---

### Task 12: Integration polish + docs touch

**Files:**
- Modify: `docs/project-gaps.md` (text chat no longer 全链路空白)
- Optional: seed script under `app/server/test/` for messages
- Fix any compile/lint fallout

- [ ] **Step 1: Full backend tests**

```bash
cd app/server && go test ./internal/model/ ./internal/repository/ ./internal/service/ ./internal/signal/ ./internal/jobs/ -count=1
```

Expected: PASS

- [ ] **Step 2: Frontend lint/test**

```bash
cd app/web && pnpm test && pnpm exec biome check src/stores/chatStore.ts src/components/textRoom src/api/message.ts
```

- [ ] **Step 3: Update project-gaps.md text chat section** to "MVP shipped / remaining: attachments threads search"

- [ ] **Step 4: Final commit**

```bash
git add docs/project-gaps.md
git commit -m "docs: mark text chat MVP progress in project-gaps"
```

---

## Spec Coverage Checklist

| Spec section | Task(s) |
|--------------|---------|
| Room.Type text\|voice | 1 |
| Message / Reaction / Mention models | 2 |
| Cursor history 50–200 | 3, 4, 6 |
| Broadcast-first + JobQueue + sync fallback | 4, 5 |
| REST send/edit/delete/react | 6 |
| Socket message:* | 7 |
| Dual slot hub + text no SFU | 7 |
| Permcodes message:* | 1, 6 |
| Frontend merge + nonce | 9 |
| Virtual scroll | 10 |
| Vertical split UI | 11 |
| Soft delete / edit / reply / @ / reactions | 4, 6, 7, 10 |
| Buffer cap ~1000 | 9 |
| Tests listed in spec §9 | 3, 4, 7, 9, 12 |

## Self-Review Notes

- No TBD placeholders; REST uses project POST style (explicit deviation from design's GET path — same semantics).
- Author identity uses **username** (claims) to match Hub; not user UUID — document in MessageDTO `author_id` as identity string.
- `room` socket field = room **name**; DB messages key by **room_uuid** — bridge always maps.
- Type names consistent: `MessageDTO`, `chat.persist`, `message:created`.

---

## Execution Handoff

Plan saved to `docs/superpowers/plans/2026-07-24-text-chat-rooms.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — fresh subagent per task, review between tasks  
2. **Inline Execution** — this session with executing-plans checkpoints  

Which approach?
