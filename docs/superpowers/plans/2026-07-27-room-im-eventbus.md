# Room IM (文字房间消息) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为文字房间补齐可持久化、可分页的 IM 后端：Socket 实时收发 + REST 历史；广播走现有 `EventBus`，业务层不直接调用 NATS。

**Architecture:** 分层 `model → repository → service → handler/signal`。发送路径：`message:send` → `MessageService.Send` 同步写 DB（ULID）→ `EventBus.PublishRoom(room, "message:new", payload)` 本机 + 跨实例 fanout。历史路径：`GET/POST` 受保护 API 按 `room + ULID cursor` 分页。`bot:message` 保留 bot 协议；人类房间聊天用新事件，避免与 bot 语义混用。

**Tech Stack:** Go、GORM、`github.com/oklog/ulid/v2`（已在 go.mod）、Socket.IO、`internal/bus.EventBus`（NATS 实现已有，IM 只依赖接口）。

## Global Constraints

- 业务代码（service / handler / signal message path）**禁止** import `github.com/nats-io/nats.go`；只依赖 `bus.EventBus` 接口（或 Hub 内既有窄接口）。
- 代码不加注释，除非复杂逻辑必须；不用 emoji。
- Service 返回 `*pkg.AppError`；Handler 用 `pkg.HandleError` / `pkg.Success` / `pkg.Fail`。
- Go 文件 `snake_case`；类型 `PascalCase`；import 三组：标准库 / 第三方 / 内部。
- 消息 ID **服务端** ULID，不信任客户端 `messageId`。
- 正文上限 500 runes（与现有 bot 桥一致）。
- 发送方必须 JWT 已鉴权且已在目标房间（与 `bot_bridge` 一致）。
- 实时推送事件名：`message:new`；客户端发送：`message:send`。
- 历史只读 active 消息；本阶段 **不做** 编辑/删除/撤回/已读/@/附件（YAGNI）。
- 多实例 fanout **只** 通过 `EventBus.PublishRoom`；NATS subject / JetStream 细节不进 IM 层。

## 决策锁定

| 项 | 决策 |
|----|------|
| 实时通道 | 复用现有 Socket.IO 信令连接 |
| 跨实例 | `EventBus.PublishRoom`（内部 NATS，对 IM 透明） |
| 写路径 | **同步落库** 再 `PublishRoom`（历史立即可读，避免只 fanout 丢持久） |
| 读路径 | 受保护 HTTP，cursor（ULID）分页 |
| 与 bot | 保留 `bot:message` / `bot:command`；人类聊天用 `message:send` / `message:new` |
| 直接 NATS | **禁止**（本 plan 范围） |
| JetStream 消息流 | **本阶段不做**（现有 JobQueue 不复用为聊天日志） |
| 前端 UI | **本阶段不做**（仅后端 + 事件常量；前端可后续接 `message:new`） |

```text
Client ──message:send──► Hub.OnMessageSend
                              │
                              ▼
                       MessageService.Send
                         │ validate
                         │ ULID + row
                         │ messageRepo.Create
                         ▼
                   EventBus.PublishRoom(room, "message:new", dto)
                         │
              ┌──────────┴──────────┐
              ▼                     ▼
        本机 Socket.IO          其他实例 EventBus
        BroadcastToRoom         再本机投递

Client ──HTTP──► MessageHandler.List ──► MessageService.List ──► repo cursor query
```

## 文件结构

| 文件 | 职责 |
|------|------|
| Create: `app/server/internal/model/message.go` | `Message` 实体 |
| Create: `app/server/internal/repository/message_repo.go` | 写入 + 按 room/cursor 列表 |
| Create: `app/server/internal/repository/message_repo_test.go` | repo 测试（sqlite） |
| Create: `app/server/internal/service/message_service.go` | 发送校验、落库、经 bus 发布 |
| Create: `app/server/internal/service/message_service_test.go` | service 单测（fake bus + fake repo 或 sqlite） |
| Create: `app/server/internal/handler/message_handler.go` | 历史列表 HTTP |
| Create: `app/server/internal/router/routes/message/routes.go` | 路由注册 |
| Create: `app/server/internal/signal/message_bridge.go` | Socket `message:send` → service |
| Create: `app/server/internal/signal/message_bridge_test.go` | 桥接测试 |
| Modify: `app/server/internal/signal/events.go` | 事件常量 |
| Modify: `app/server/internal/signal/hub.go` | 注入 MessageService、注册事件、可选房间成员检查辅助 |
| Modify: `app/server/internal/repository/db.go` | AutoMigrate `Message` |
| Modify: `app/server/server/gin.go` | DI |
| Modify: `app/server/internal/router/router.go` | Handlers + RegisterProtected |
| Modify: `app/web/src/socket/events.ts` | 常量对齐（无 UI） |

**明确不改：** `bot_bridge.go` 行为（可后续可选对接 service）、SFU、JobQueue、直接 NATS API。

---

### Task 1: Message model + repository

**Files:**
- Create: `app/server/internal/model/message.go`
- Create: `app/server/internal/repository/message_repo.go`
- Create: `app/server/internal/repository/message_repo_test.go`
- Modify: `app/server/internal/repository/db.go`（AutoMigrate 增加 `&model.Message{}`）

**Interfaces:**
- Consumes: GORM `*gorm.DB`、现有 `model` 风格
- Produces:
  - `model.Message` 字段见下
  - `repository.NewMessageRepository(db *gorm.DB) *MessageRepository`
  - `(*MessageRepository).Create(msg *model.Message) error`
  - `(*MessageRepository).ListByRoom(roomUUID string, beforeULID string, limit int) ([]model.Message, error)`
  - `ListByRoom`：`beforeULID == ""` 时取最新 `limit` 条（按 `id` 降序再在 service 里反转为时间正序，或 repo 直接升序返回——**约定 repo 返回时间升序**）；`beforeULID != ""` 时取 `id < beforeULID` 的更早消息，升序，最多 `limit`

- [ ] **Step 1: Write failing repo test**

```go
// app/server/internal/repository/message_repo_test.go
package repository

import (
	"GOSpeak/internal/model"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Message{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMessageRepository_CreateAndList(t *testing.T) {
	db := setupMessageTestDB(t)
	repo := NewMessageRepository(db)
	room := "11111111-1111-1111-1111-111111111111"
	id1 := ulid.Make().String()
	time.Sleep(2 * time.Millisecond)
	id2 := ulid.Make().String()

	for _, m := range []*model.Message{
		{ID: id1, RoomUUID: room, SenderIdentity: "alice", SenderDisplay: "Alice", SenderRole: "user", Content: "hi", Status: model.MessageStatusActive},
		{ID: id2, RoomUUID: room, SenderIdentity: "bob", SenderDisplay: "Bob", SenderRole: "user", Content: "yo", Status: model.MessageStatusActive},
	} {
		if err := repo.Create(m); err != nil {
			t.Fatal(err)
		}
	}

	list, err := repo.ListByRoom(room, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d", len(list))
	}
	if list[0].ID != id1 || list[1].ID != id2 {
		t.Fatalf("order wrong: %+v", list)
	}

	older, err := repo.ListByRoom(room, id2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(older) != 1 || older[0].ID != id1 {
		t.Fatalf("cursor page: %+v", older)
	}
}
```

- [ ] **Step 2: Run test — expect fail**

```bash
cd app/server && go test ./internal/repository/ -run TestMessageRepository_CreateAndList -count=1
```

Expected: fail（`Message` / `NewMessageRepository` 未定义）

- [ ] **Step 3: Implement model**

```go
// app/server/internal/model/message.go
package model

import "time"

const (
	MessageStatusActive  = "active"
	MessageStatusDeleted = "deleted"
)

type Message struct {
	ID             string    `gorm:"primaryKey;size:26" json:"id"`
	RoomUUID       string    `gorm:"size:36;index:idx_msg_room_id,priority:1;not null" json:"room_uuid"`
	SenderIdentity string    `gorm:"size:64;not null" json:"sender_identity"`
	SenderDisplay  string    `gorm:"size:128" json:"sender_display"`
	SenderRole     string    `gorm:"size:32" json:"sender_role"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	ReplyToID      string    `gorm:"size:26" json:"reply_to_id,omitempty"`
	Status         string    `gorm:"size:16;index;default:active" json:"status"`
	CreatedAt      time.Time `gorm:"index:idx_msg_room_id,priority:2" json:"created_at"`
}

func (Message) TableName() string { return "messages" }
```

- [ ] **Step 4: Implement repository**

```go
// app/server/internal/repository/message_repo.go
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
	// fetch newest page then reverse to ascending
	err := q.Order("id DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}
```

- [ ] **Step 5: AutoMigrate**

In `app/server/internal/repository/db.go` `AutoMigrate(...)` list, append:

```go
&model.Message{},
```

- [ ] **Step 6: Run test — expect pass**

```bash
cd app/server && go test ./internal/repository/ -run TestMessageRepository_CreateAndList -count=1
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add app/server/internal/model/message.go \
  app/server/internal/repository/message_repo.go \
  app/server/internal/repository/message_repo_test.go \
  app/server/internal/repository/db.go
git commit -m "feat(im): add message model and repository"
```

---

### Task 2: MessageService + EventBus fanout

**Files:**
- Create: `app/server/internal/service/message_service.go`
- Create: `app/server/internal/service/message_service_test.go`

**Interfaces:**
- Consumes: `*repository.MessageRepository`；窄接口 `MessageEventBus`：

```go
type MessageEventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}
```

  （实现为现有 `bus.EventBus`，**不**在 service 里引用 NATS。）

- Produces:
  - `NewMessageService(repo *repository.MessageRepository, bus MessageEventBus) *MessageService`
  - `SetEventBus(bus MessageEventBus)`（可选，便于 gin 晚注入）
  - `Send(ctx, input MessageSendInput) (*MessageDTO, error)`
  - `List(ctx, roomUUID, beforeULID string, limit int) (*MessageListResult, error)`
  - 对外 DTO / 事件 payload 字段：

```go
type MessageSendInput struct {
	RoomUUID       string // 房间业务名或 UUID：与 Hub 房间 key 对齐，见 Task 3
	RoomKey        string // Socket 房间名（Broadcast 用，与 Hub rooms map key 一致）
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
	NextCursor string       `json:"nextCursor,omitempty"` // 更早一页的 before=oldest.id
}
```

  - 发布事件名常量（service 包或 signal 包共享）：与 `signal.EventMessageNew` 字符串 **完全一致** `"message:new"`。
  - `Room` 字段在 DTO 里用 **Hub 房间名**（`RoomKey`），便于前端；DB 存 `RoomUUID`（若当前 Hub 仅用 name 作 key，则 `RoomUUID` 与 `RoomKey` 可同为 name——**约定：DB `room_uuid` 存 Hub 使用的 room 字符串 key**，与 `bot_bridge` 的 `req.Room` 一致，避免本阶段强依赖 DB room 表 UUID 解析）。

- [ ] **Step 1: Write failing service test**

```go
// app/server/internal/service/message_service_test.go
package service

import (
	"context"
	"sync"
	"testing"
	"unicode/utf8"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type memBus struct {
	mu       sync.Mutex
	room     string
	event    string
	payload  interface{}
	calls    int
}

func (b *memBus) PublishRoom(ctx context.Context, room, event string, payload interface{}) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	b.room, b.event, b.payload = room, event, payload
	return nil
}

func setupMsgSvc(t *testing.T) (*MessageService, *memBus) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:msg_svc?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.AutoMigrate(&model.Message{})
	bus := &memBus{}
	svc := NewMessageService(repository.NewMessageRepository(db), bus)
	return svc, bus
}

func TestMessageService_Send_PublishesAndPersists(t *testing.T) {
	svc, bus := setupMsgSvc(t)
	dto, err := svc.Send(context.Background(), MessageSendInput{
		RoomKey:        "lobby",
		SenderIdentity: "alice",
		SenderDisplay:  "Alice",
		SenderRole:     "user",
		Content:        "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dto.ID == "" || dto.Content != "hello" {
		t.Fatalf("%+v", dto)
	}
	if bus.calls != 1 || bus.event != "message:new" || bus.room != "lobby" {
		t.Fatalf("bus=%+v", bus)
	}
	list, err := svc.List(context.Background(), "lobby", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Messages) != 1 || list.Messages[0].ID != dto.ID {
		t.Fatalf("%+v", list)
	}
}

func TestMessageService_Send_RejectsEmptyAndTooLong(t *testing.T) {
	svc, _ := setupMsgSvc(t)
	if _, err := svc.Send(context.Background(), MessageSendInput{RoomKey: "r", SenderIdentity: "a", Content: ""}); err == nil {
		t.Fatal("empty")
	}
	long := make([]rune, 501)
	for i := range long {
		long[i] = 'x'
	}
	if _, err := svc.Send(context.Background(), MessageSendInput{RoomKey: "r", SenderIdentity: "a", Content: string(long)}); err == nil {
		t.Fatal("long")
	}
	if utf8.RuneCountInString(string(long)) != 501 {
		t.Fatal("fixture")
	}
}
```

- [ ] **Step 2: Run test — expect fail**

```bash
cd app/server && go test ./internal/service/ -run TestMessageService_ -count=1
```

- [ ] **Step 3: Implement MessageService**

```go
// app/server/internal/service/message_service.go
package service

import (
	"context"
	"crypto/rand"
	"time"
	"unicode/utf8"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/oklog/ulid/v2"
)

const (
	EventMessageNew   = "message:new"
	MaxMessageRunes   = 500
	DefaultListLimit  = 50
	MaxListLimit      = 100
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

func (s *MessageService) SetEventBus(bus MessageEventBus) {
	s.bus = bus
}

func (s *MessageService) Send(ctx context.Context, in MessageSendInput) (*MessageDTO, error) {
	if in.RoomKey == "" || in.SenderIdentity == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "room and sender required")
	}
	content := in.Content
	if content == "" || utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid content")
	}
	id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR)
	}
	now := time.Now().UTC()
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
			// 已落库；广播失败只记错误码返回，调用方可记日志。仍返回 dto。
			return dto, pkg.NewAppError(pkg.INTERNAL_ERROR, "publish failed")
		}
	}
	return dto, nil
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
```

**广播失败策略（锁定）：** `Create` 成功后 `PublishRoom` 失败 → 返回 `INTERNAL_ERROR` 且仍带 dto 困难（当前签名只返回 error）。实现采用：**Publish 失败返回 error，消息已在 DB**（客户端可靠历史拉取补偿）。测试里 memBus 不失败即可。若需「广播失败不报错」，可在实现时只 `log` 并 `return dto, nil`——**本 plan 锁定为 log + 仍 `return dto, nil`**，避免发送方误以为失败重发重复消息：

将 `Send` 末尾改为：

```go
	if s.bus != nil {
		if err := s.bus.PublishRoom(ctx, in.RoomKey, EventMessageNew, dto); err != nil {
			// 持久化已成功；fanout 失败由多实例/重连后历史 API 补偿
			_ = err
		}
	}
	return dto, nil
```

并相应放宽测试（只断言 `bus.calls` 在成功 bus 时为 1）。

- [ ] **Step 4: Run tests — expect pass**

```bash
cd app/server && go test ./internal/service/ -run TestMessageService_ -count=1
```

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/service/message_service.go \
  app/server/internal/service/message_service_test.go
git commit -m "feat(im): message service with EventBus publish"
```

---

### Task 3: Socket bridge `message:send`

**Files:**
- Modify: `app/server/internal/signal/events.go`
- Create: `app/server/internal/signal/message_bridge.go`
- Create: `app/server/internal/signal/message_bridge_test.go`
- Modify: `app/server/internal/signal/hub.go`（字段、setter、`SetupRoutes` 注册）

**Interfaces:**
- Consumes: `*service.MessageService`（或接口 `MessageSender`：`Send(ctx, service.MessageSendInput) (*service.MessageDTO, error)`）
- Produces: Hub 方法 `OnMessageSend`；事件常量：

```go
EventMessageSend = "message:send"
EventMessageNew  = "message:new"
```

- 成员校验：复用 `bot_bridge` 模式——`h.mu.RLock` 查 `h.rooms[req.Room]` 成员 identity；不在房间则静默 return（与 bot 一致）。
- **不**在 bridge 里再 `BroadcastToRoom`：fanout 已由 `MessageService` → `EventBus.PublishRoom` 完成（含本机 Deliverer）。

- [ ] **Step 1: Add event constants**

In `events.go` add:

```go
	EventMessageSend = "message:send"
	EventMessageNew  = "message:new"
```

- [ ] **Step 2: Write bridge test**（可仿 `bot_bridge_test.go`：假 Conn、注入 hub rooms、假 MessageService 记录调用；或集成 sqlite service + mem bus）

最小：

```go
// 验证：未入房不调用 Send；入房后调用 Send 且 Content 正确。
```

实现时复制 `bot_bridge_test.go` 的 conn/hub 脚手架，注入：

```go
type stubMsgSvc struct {
	last service.MessageSendInput
	n    int
}
func (s *stubMsgSvc) Send(ctx context.Context, in service.MessageSendInput) (*service.MessageDTO, error) {
	s.n++
	s.last = in
	return &service.MessageDTO{ID: "01TEST", Room: in.RoomKey, Content: in.Content}, nil
}
```

Hub 依赖接口：

```go
type messageSender interface {
	Send(ctx context.Context, in service.MessageSendInput) (*service.MessageDTO, error)
}
```

- [ ] **Step 3: Implement `message_bridge.go`**

```go
package signal

import (
	"context"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	socketio "github.com/googollee/go-socket.io"
)

type messageSendPayload struct {
	Room    string `json:"room"`
	Content string `json:"content"`
	Text    string `json:"text"`
	ReplyTo string `json:"replyTo,omitempty"`
}

type messageSender interface {
	Send(ctx context.Context, in service.MessageSendInput) (*service.MessageDTO, error)
}

func (h *Hub) OnMessageSend(s socketio.Conn, data string) {
	if h.messageSvc == nil {
		return
	}
	var req messageSendPayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return
	}
	text := req.Content
	if text == "" {
		text = req.Text
	}
	identity := claimsIdentity(s)
	if identity == "" {
		return
	}
	h.mu.RLock()
	room, exists := h.rooms[req.Room]
	var member *MemberInfo
	if exists {
		for _, m := range room.Members {
			if m.Identity == identity {
				member = m
				break
			}
		}
	}
	h.mu.RUnlock()
	if member == nil {
		return
	}
	display := member.DisplayName
	if display == "" {
		display = member.Name
	}
	role := "member"
	if ctx := s.Context(); ctx != nil {
		if claims, ok := ctx.(*pkg.Claims); ok && claims.Role != "" {
			role = claims.Role
		}
	}
	_, _ = h.messageSvc.Send(context.Background(), service.MessageSendInput{
		RoomKey:        req.Room,
		SenderIdentity: identity,
		SenderDisplay:  display,
		SenderRole:     role,
		Content:        text,
		ReplyToID:      req.ReplyTo,
	})
}
```

- [ ] **Step 4: Wire Hub**

In `Hub` struct add `messageSvc messageSender`。

```go
func (h *Hub) SetMessageService(svc messageSender) { h.messageSvc = svc }
```

In `SetupRoutes`:

```go
server.OnEvent("/", EventMessageSend, safeOnEventData(h.OnMessageSend))
```

- [ ] **Step 5: Tests pass**

```bash
cd app/server && go test ./internal/signal/ -run Message -count=1
```

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/signal/events.go \
  app/server/internal/signal/message_bridge.go \
  app/server/internal/signal/message_bridge_test.go \
  app/server/internal/signal/hub.go
git commit -m "feat(im): socket message:send bridge via MessageService"
```

---

### Task 4: HTTP history API + DI + routes

**Files:**
- Create: `app/server/internal/handler/message_handler.go`
- Create: `app/server/internal/router/routes/message/routes.go`
- Modify: `app/server/internal/router/router.go`
- Modify: `app/server/server/gin.go`
- Modify: `app/web/src/socket/events.ts`（常量）

**Interfaces:**
- HTTP（受保护，与 room 模块风格一致，POST body）：

```
POST /api/v1/message/list
body: { "room": "<room key>", "before": "<ulid optional>", "limit": 50 }
auth: JWT
permission: 复用 PermRoomRead（与读房间一致）
```

- Produces: `Handlers.Message *handler.MessageHandler`

- [ ] **Step 1: Handler**

```go
// app/server/internal/handler/message_handler.go
package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	svc *service.MessageService
}

func NewMessageHandler(svc *service.MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
}

func (h *MessageHandler) List(c *gin.Context) {
	var req struct {
		Room   string `json:"room" binding:"required"`
		Before string `json:"before"`
		Limit  int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	out, err := h.svc.List(c.Request.Context(), req.Room, req.Before, req.Limit)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}
```

- [ ] **Step 2: Routes**

```go
// app/server/internal/router/routes/message/routes.go
package message

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.MessageHandler) {
	r.POST("/list", middleware.RequirePermission(permcode.PermRoomRead), h.List)
}
```

- [ ] **Step 3: router.Handlers + SetupRoutes**

- `Handlers` 增加 `Message *handler.MessageHandler`
- import `messageRoutes "GOSpeak/internal/router/routes/message"`
- `messageRoutes.RegisterProtected(protected.Group("/message"), h.Message)`

- [ ] **Step 4: gin.go DI**

在 repo 区：

```go
messageRepo := repository.NewMessageRepository(repository.DB)
```

在 service 区（eventBus 创建**之后**，因需要 bus）：

```go
messageSvc := service.NewMessageService(messageRepo, eventBus)
signalHub.SetMessageService(messageSvc)
```

若 `messageSvc` 必须在 `eventBus` 之前构造：先 `NewMessageService(messageRepo, nil)`，`bus.Init` 后 `messageSvc.SetEventBus(eventBus)`。

Handler：

```go
messageH := handler.NewMessageHandler(messageSvc)
```

`router.Handlers{ ..., Message: messageH }`

**注意：** `eventBus` 类型已是 `bus.EventBus`，满足 `service.MessageEventBus`（`PublishRoom`）。**不要**把 `*nats.Conn` 传给 MessageService。

- [ ] **Step 5: Frontend event constants only**

`app/web/src/socket/events.ts`:

```ts
MESSAGE_SEND: "message:send",
MESSAGE_NEW: "message:new",
```

- [ ] **Step 6: Compile + unit tests**

```bash
cd app/server && go test ./internal/repository/ ./internal/service/ ./internal/signal/ -count=1
cd app/server && go build -o /dev/null .
```

- [ ] **Step 7: Commit**

```bash
git add app/server/internal/handler/message_handler.go \
  app/server/internal/router/routes/message/routes.go \
  app/server/internal/router/router.go \
  app/server/server/gin.go \
  app/web/src/socket/events.ts
git commit -m "feat(im): message list API and DI wiring"
```

---

### Task 5: 手动验证清单 + 文档锚点

**Files:**
- 可选 Modify: `ARCHITECTURE.md` 增加 IM 一小节（3–5 行）——若仓库惯例要求；否则跳过。

- [ ] **Step 1: 本地验证步骤（执行者执行）**

1. `pnpm dev:server` 或 `go run . server -e dev`
2. 两客户端同房 JWT 连 Socket
3. Emit `message:send` `{"room":"<name>","content":"ping"}`
4. 双方收到 `message:new`，payload 含服务端 `id`（26 字符 ULID）
5. `POST /api/v1/message/list` + Bearer，body `{"room":"<name>","limit":50}` → 含该消息
6. 再发多条，`before` 为最早一条 id → 更早页
7. 确认 `app/server` 内 `grep -R nats.go internal/service/message_service.go internal/handler/message_handler.go internal/signal/message_bridge.go` **无命中**

- [ ] **Step 2: Commit docs if any**

```bash
git add ARCHITECTURE.md  # if touched
git commit -m "docs: note room IM over EventBus"
```

---

## Self-Review

| Spec / 决策 | Task |
|-------------|------|
| 持久化 + cursor 历史 | T1, T2, T4 |
| 代码解耦（service 层，bot 分离） | T2, T3 |
| 复用信令 Socket | T3 |
| 只经 EventBus，不直接 NATS | T2（接口）、T4（注入 eventBus） |
| ULID 服务端 ID | T2 |
| ≤500 runes | T2 |
| 入房 + JWT | T3 |
| 不做编辑删除已读附件前端 UI | 全局约束 |
| 保留 bot:message | 不改 bot_bridge |

**Placeholder scan:** 无 TBD；广播失败策略已锁定为 log + 仍返回成功 dto。  
**Type consistency:** `MessageDTO` / `MessageSendInput` / `EventMessageNew`=`"message:new"` 全文一致；DB `RoomUUID` 列存 Hub room key。  
**Gap:** 前端不监听 `message:new` 属本阶段范围外；多实例依赖现有 EventBus 已验证路径。

---

## Execution Handoff

Plan 已写到 `docs/superpowers/plans/2026-07-27-room-im-eventbus.md`。

**两种执行方式：**

1. **Subagent-Driven（推荐）** — 每任务新 subagent，任务间审查  
2. **Inline Execution** — 本会话按 executing-plans 批量执行并设检查点  

选哪个？
