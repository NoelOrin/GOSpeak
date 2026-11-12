# 站内通知（系统公告）与定向推送能力 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 给 GOSpeak 增加一条「向指定用户推送事件」的一等能力，并用它实现系统公告中心（落库、精确已读、侧栏 badge、通知弹窗、管理后台发布页）。

**架构：** 传输复用现有 `/ws`，不新增 SSE。后端新增 `internal/notify` 把「发给某用户」翻译成现有的 `__user:{uuid}` fanout 房间投递；公告走标准 model → repository → service → handler → route 分层，发布后通过 notify 推 `announcement:new`。前端在现有 WS 事件绑定器上注册新事件，新增 `notificationStore` 管未读与列表，侧栏铃铛挂 badge，点击弹 Radix Dialog。

**技术栈：** Go 1.26 + Gin + GORM（SQLite/PostgreSQL/MySQL）、React 19 + TypeScript + zustand + TanStack Router/Query、Radix Dialog、marked + DOMPurify、vitest。

**规格来源：** `docs/superpowers/specs/2026-11-10-announcement-notification-bus-design.md`

---

## 文件结构

### 后端（新建）

| 文件 | 职责 |
|------|------|
| `app/server/internal/notify/notify.go` | 定向推送原语：`NotifyUser` / `NotifyAll`。只依赖最小 `EventBus` 接口，不 import `bus` 包 |
| `app/server/internal/notify/notify_test.go` | 单测：断言房间名、事件名、payload 不被二次 marshal |
| `app/server/internal/model/announcement.go` | `Announcement` + `AnnouncementRead` 两张表 |
| `app/server/internal/repository/announcement_repo.go` | 全部 SQL：可见性过滤、未读计数、幂等已读写入 |
| `app/server/internal/repository/announcement_repo_test.go` | 仓库单测（内存 SQLite） |
| `app/server/internal/service/announcement_service.go` | DTO、可见性、`canManage` 权限分叉、发布后投递 |
| `app/server/internal/service/announcement_service_test.go` | 服务单测：可见性、未读数、mark-read 幂等、权限分叉 |
| `app/server/internal/handler/announcement_handler.go` | 7 个 HTTP 端点，只做参数绑定 + 调 service + 响应 |
| `app/server/internal/router/routes/announcement/routes.go` | 路由注册 |
| `test/announcement/announcement.test.ts` | 集成测试：发布→可见→未读→已读、权限、域隔离、WS 推送 |

### 后端（修改）

| 文件 | 改动 |
|------|------|
| `app/server/internal/permcode/permcode.go` | 追加 `PermAnnouncementRead` / `PermAnnouncementManage` |
| `app/server/internal/model/permission.go` | 别名、`DefaultPermissions` 种子、`DefaultRolePermissions`（admin 两项、user 只加 read） |
| `app/server/internal/model/permission_test.go` | 断言新权限码 |
| `app/server/internal/repository/db_migrations.go:238` | `autoMigrate()` 列表追加两张表 |
| `app/server/internal/router/router.go:40-65` | `Handlers` 加 `Announcement` 字段；`:110-133` 注册路由 |
| `app/server/server/gin.go:89-105` | 构造 repo；`:411-422` 构造 service/handler；`:450-475` 塞进 `Handlers` 字面量 |

### 前端（新建）

| 文件 | 职责 |
|------|------|
| `packages/components/src/components/badge.tsx` | 通用 `Badge`：数字 / `99+` / dot 三态 |
| `packages/components/src/components/badge.test.tsx` | 单测 |
| `app/web/src/protocol/announcement.ts` | `AnnouncementDTO`（与后端 snake_case 对齐） |
| `app/web/src/api/announcement.ts` | 7 个 API 函数 |
| `app/web/src/stores/notificationStore.ts` | 未读数、列表、乐观已读、WS 增量 |
| `app/web/src/stores/notificationStore.test.ts` | 单测 |
| `app/web/src/components/modal/notificationModal.tsx` | 通知弹窗（列表 + Markdown 详情） |
| `app/web/src/components/modal/notificationModal.test.tsx` | 单测：渲染、点击标已读、XSS 清洗 |
| `app/web/src/pages/(app)/manage/announcements/index.tsx` | 管理页 |

### 前端（修改）

| 文件 | 改动 |
|------|------|
| `packages/components/src/index.ts` | 导出 `Badge` |
| `app/web/src/socket/events.ts` | `ANNOUNCEMENT_NEW` |
| `app/web/src/socket/socketEvents.ts` | deps 加 `handleAnnouncementNew`；绑定事件 |
| `app/web/src/stores/socketStore.ts` | 暴露 `onConnected`；注册 announcement handler 回调；传给 `bindServerEvents` |
| `app/web/src/components/common/optionSquare.tsx` | 加 `badge` prop |
| `app/web/src/layouts/common/sidebar.tsx` | 铃铛入口 + badge + 打开弹窗回调 |
| `app/web/src/layouts/layout.tsx` | 弹窗单例状态 |
| `app/web/src/components/manage/manageNav.tsx` | `ManagePath` + `MANAGE_TABS` 追加 |
| `app/web/src/utils/permissions.ts` | 兜底表 + `MANAGE_ENTRY_PERMISSIONS` |
| `app/web/src/components/room/components/roomItem.tsx:256-263` | 迁移到 `Badge` |
| `app/web/src/components/chat/conversationList.tsx:65-69` | 迁移到 `Badge` |

---

## 任务 1：定向推送原语 `internal/notify`

**文件：**
- 创建：`app/server/internal/notify/notify.go`
- 测试：`app/server/internal/notify/notify_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
package notify

import (
	"context"
	"encoding/json"
	"testing"
)

type recordedPublish struct {
	room  string
	event string
	data  interface{}
}

type fakeBus struct {
	namespace []recordedPublish
	room      []recordedPublish
	err       error
}

func (b *fakeBus) PublishNamespace(_ context.Context, event string, payload interface{}) error {
	if b.err != nil {
		return b.err
	}
	b.namespace = append(b.namespace, recordedPublish{event: event, data: payload})
	return nil
}

func (b *fakeBus) PublishRoom(_ context.Context, room, event string, payload interface{}) error {
	if b.err != nil {
		return b.err
	}
	b.room = append(b.room, recordedPublish{room: room, event: event, data: payload})
	return nil
}

type samplePayload struct {
	UUID  string `json:"uuid"`
	Title string `json:"title"`
}

func TestNotifyUser_TargetsPersonalRoom(t *testing.T) {
	bus := &fakeBus{}
	payload := samplePayload{UUID: "a-1", Title: "维护公告"}

	if err := NotifyUser(context.Background(), bus, "user-uuid-1", "announcement:new", payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bus.room) != 1 {
		t.Fatalf("expected 1 room publish, got %d", len(bus.room))
	}
	if bus.room[0].room != "__user:user-uuid-1" {
		t.Errorf("room = %q, want %q", bus.room[0].room, "__user:user-uuid-1")
	}
	if bus.room[0].event != "announcement:new" {
		t.Errorf("event = %q", bus.room[0].event)
	}
	if len(bus.namespace) != 0 {
		t.Errorf("NotifyUser must not broadcast to namespace")
	}
}

// payload 必须是 struct，不能被序列化成 []byte（否则 fanout 二次 marshal 会变 base64 字符串）。
func TestNotifyUser_PayloadStaysObject(t *testing.T) {
	bus := &fakeBus{}
	if err := NotifyUser(context.Background(), bus, "u1", "evt", samplePayload{UUID: "x", Title: "t"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, err := json.Marshal(map[string]interface{}{"event": bus.room[0].event, "data": bus.room[0].data})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	var decoded struct {
		Data struct {
			UUID  string `json:"uuid"`
			Title string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if decoded.Data.Title != "t" {
		t.Fatalf("payload was not encoded as an object; got %s", raw)
	}
	if _, isString := bus.room[0].data.([]byte); isString {
		t.Fatal("payload must not be []byte")
	}
}

func TestNotifyUser_RejectsEmptyUser(t *testing.T) {
	bus := &fakeBus{}
	if err := NotifyUser(context.Background(), bus, "", "evt", samplePayload{}); err == nil {
		t.Fatal("expected error for empty user uuid")
	}
	if len(bus.room) != 0 {
		t.Fatal("must not publish for empty user uuid")
	}
}

func TestNotifyAll_UsesNamespace(t *testing.T) {
	bus := &fakeBus{}
	if err := NotifyAll(context.Background(), bus, "announcement:new", samplePayload{UUID: "g"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bus.namespace) != 1 {
		t.Fatalf("expected 1 namespace publish, got %d", len(bus.namespace))
	}
	if len(bus.room) != 0 {
		t.Fatal("NotifyAll must not publish to a room")
	}
}

func TestNotify_NilBusIsSafe(t *testing.T) {
	if err := NotifyUser(context.Background(), nil, "u1", "evt", samplePayload{}); err == nil {
		t.Fatal("expected error for nil bus")
	}
	if err := NotifyAll(context.Background(), nil, "evt", samplePayload{}); err == nil {
		t.Fatal("expected error for nil bus")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
cd app/server && go test ./internal/notify/ -v
```
预期：FAIL，`undefined: NotifyUser`。

- [ ] **步骤 3：编写实现**

```go
// Package notify 提供服务端到客户端的定向推送原语。
//
// 传输复用现有 WebSocket：NotifyUser 投递到 fanout 伪房间 "__user:{uuid}"，
// 该房间的成员关系由 signal.Hub.OnConnect 建立、断连时清理。
//
// 能力边界：仅对在线连接可达。总线没有 user subject、没有 ack/offset，
// WAL 只覆盖「发布时 NATS 断线」且默认关闭。任何必须送达的通知
// 必须自带持久化 inbox（公告中心即如此），不能只依赖本包。
package notify

import (
	"context"
	"fmt"

	"GOSpeak/internal/pkg"
)

// UserRoomPrefix 是 fanout 中按用户定向投递的伪房间前缀。
const UserRoomPrefix = "__user:"

// EventBus 是本包依赖的最小发布面，由 bus.EventBus 满足。
type EventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
}

// NotifyUser 向单个用户的全部在线连接推送。
// payload 必须传 struct / map / json.RawMessage；禁止传 []byte，
// 否则 Fanout 二次 marshal 后前端会收到 base64 字符串而非对象。
func NotifyUser(ctx context.Context, bus EventBus, userUUID, event string, payload interface{}) error {
	if bus == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "notify: nil event bus")
	}
	if userUUID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "notify: empty user uuid")
	}
	if event == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "notify: empty event")
	}
	if _, isBytes := payload.([]byte); isBytes {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "notify: payload must not be []byte")
	}
	if err := bus.PublishRoom(ctx, UserRoomPrefix+userUUID, event, payload); err != nil {
		return fmt.Errorf("notify user %s: %w", userUUID, err)
	}
	return nil
}

// NotifyAll 向所有在线连接推送。
func NotifyAll(ctx context.Context, bus EventBus, event string, payload interface{}) error {
	if bus == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "notify: nil event bus")
	}
	if event == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "notify: empty event")
	}
	if _, isBytes := payload.([]byte); isBytes {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "notify: payload must not be []byte")
	}
	if err := bus.PublishNamespace(ctx, event, payload); err != nil {
		return fmt.Errorf("notify all: %w", err)
	}
	return nil
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
cd app/server && go test ./internal/notify/ -v
```
预期：PASS（5 个用例）。

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/notify/
git commit -m "feat(notify): 新增定向推送原语 NotifyUser/NotifyAll"
```

---

## 任务 2：公告数据模型与仓库

**文件：**
- 创建：`app/server/internal/model/announcement.go`
- 创建：`app/server/internal/repository/announcement_repo.go`
- 创建：`app/server/internal/repository/announcement_repo_test.go`
- 修改：`app/server/internal/repository/db_migrations.go:238-265`

- [ ] **步骤 1：编写模型**

```go
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	AnnouncementScopePlatform = "platform"
	AnnouncementScopeDomain   = "domain"

	AnnouncementLevelInfo     = "info"
	AnnouncementLevelWarning  = "warning"
	AnnouncementLevelCritical = "critical"
)

// Announcement 是站内系统公告。PublishedAt 为 nil 表示草稿，不进入任何用户可见集合。
type Announcement struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UUID          string         `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	ScopeType     string         `gorm:"size:16;not null;index:idx_announcement_visible,priority:1" json:"scope_type"`
	ScopeID       string         `gorm:"size:32;index:idx_announcement_visible,priority:2" json:"scope_id"`
	Title         string         `gorm:"size:200;not null" json:"title"`
	Content       string         `gorm:"type:text;not null" json:"content"`
	Level         string         `gorm:"size:16;default:info;index" json:"level"`
	Pinned        bool           `gorm:"default:false" json:"pinned"`
	PublishedAt   *time.Time     `gorm:"index:idx_announcement_visible,priority:3" json:"published_at,omitempty"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
	CreatedByUUID string         `gorm:"size:36;index" json:"created_by_uuid"`
	CreatedByName string         `gorm:"size:64" json:"created_by_name"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func (a *Announcement) BeforeCreate(_ *gorm.DB) error {
	if a.UUID == "" {
		a.UUID = uuid.New().String()
	}
	if a.Level == "" {
		a.Level = AnnouncementLevelInfo
	}
	if a.ScopeType == "" {
		a.ScopeType = AnnouncementScopePlatform
	}
	return nil
}

func (a *Announcement) TableName() string { return "announcements" }

// AnnouncementRead 记录「某用户已读某公告」，联合唯一保证幂等。
type AnnouncementRead struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	AnnouncementUUID string    `gorm:"size:36;uniqueIndex:idx_announcement_user;not null" json:"announcement_uuid"`
	UserUUID         string    `gorm:"size:36;uniqueIndex:idx_announcement_user;not null" json:"user_uuid"`
	ReadAt           time.Time `json:"read_at"`
}

func (AnnouncementRead) TableName() string { return "announcement_reads" }
```

- [ ] **步骤 2：注册迁移**

在 `app/server/internal/repository/db_migrations.go` 的 `autoMigrate()` 中，把 `&model.DomainGuestBan{},` 之后追加：

```go
		&model.Announcement{},
		&model.AnnouncementRead{},
```

- [ ] **步骤 3：编写失败的仓库测试**

```go
package repository

import (
	"testing"
	"time"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAnnouncementDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:announcement_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func publishedAnnouncement(scope, scopeID, title string) *model.Announcement {
	now := time.Now()
	return &model.Announcement{
		ScopeType:   scope,
		ScopeID:     scopeID,
		Title:       title,
		Content:     "body",
		Level:       model.AnnouncementLevelInfo,
		PublishedAt: &now,
	}
}

func TestAnnouncementRepo_ListVisibleFiltersScope(t *testing.T) {
	db := setupAnnouncementDB(t)
	repo := NewAnnouncementRepository(db)

	platform := publishedAnnouncement(model.AnnouncementScopePlatform, "", "platform notice")
	domainMine := publishedAnnouncement(model.AnnouncementScopeDomain, "dom-1", "mine")
	domainOther := publishedAnnouncement(model.AnnouncementScopeDomain, "dom-2", "other")
	for _, a := range []*model.Announcement{platform, domainMine, domainOther} {
		if err := repo.Create(a); err != nil {
			t.Fatal(err)
		}
	}

	items, total, err := repo.ListVisible("user-1", []string{"dom-1"}, false, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	for _, item := range items {
		if item.Title == "other" {
			t.Fatal("announcement from a foreign domain must not be visible")
		}
	}
}

func TestAnnouncementRepo_ListVisibleExcludesDraftAndExpired(t *testing.T) {
	db := setupAnnouncementDB(t)
	repo := NewAnnouncementRepository(db)

	draft := publishedAnnouncement(model.AnnouncementScopePlatform, "", "draft")
	draft.PublishedAt = nil
	past := time.Now().Add(-time.Hour)
	expired := publishedAnnouncement(model.AnnouncementScopePlatform, "", "expired")
	expired.ExpiresAt = &past
	future := time.Now().Add(time.Hour)
	alive := publishedAnnouncement(model.AnnouncementScopePlatform, "", "alive")
	alive.ExpiresAt = &future

	for _, a := range []*model.Announcement{draft, expired, alive} {
		if err := repo.Create(a); err != nil {
			t.Fatal(err)
		}
	}

	items, total, err := repo.ListVisible("user-1", nil, false, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].Title != "alive" {
		t.Fatalf("got total=%d items=%v", total, items)
	}
}

func TestAnnouncementRepo_UnreadCountAndMarkReadIdempotent(t *testing.T) {
	db := setupAnnouncementDB(t)
	repo := NewAnnouncementRepository(db)

	a1 := publishedAnnouncement(model.AnnouncementScopePlatform, "", "one")
	a2 := publishedAnnouncement(model.AnnouncementScopePlatform, "", "two")
	for _, a := range []*model.Announcement{a1, a2} {
		if err := repo.Create(a); err != nil {
			t.Fatal(err)
		}
	}

	count, err := repo.CountUnread("user-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("unread = %d, want 2", count)
	}

	for i := 0; i < 2; i++ {
		if err := repo.MarkRead(a1.UUID, "user-1"); err != nil {
			t.Fatalf("mark read attempt %d: %v", i, err)
		}
	}

	count, err = repo.CountUnread("user-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("unread after duplicate mark = %d, want 1", count)
	}
}

func TestAnnouncementRepo_OnlyUnreadFilter(t *testing.T) {
	db := setupAnnouncementDB(t)
	repo := NewAnnouncementRepository(db)

	a1 := publishedAnnouncement(model.AnnouncementScopePlatform, "", "read me")
	a2 := publishedAnnouncement(model.AnnouncementScopePlatform, "", "leave me")
	for _, a := range []*model.Announcement{a1, a2} {
		if err := repo.Create(a); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.MarkRead(a1.UUID, "user-1"); err != nil {
		t.Fatal(err)
	}

	items, total, err := repo.ListVisible("user-1", nil, true, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || items[0].Title != "leave me" {
		t.Fatalf("only_unread filter broken: total=%d", total)
	}
}
```

- [ ] **步骤 4：运行测试验证失败**

```bash
cd app/server && go test ./internal/repository/ -run TestAnnouncementRepo -v
```
预期：FAIL，`undefined: NewAnnouncementRepository`。

- [ ] **步骤 5：编写仓库实现**

```go
package repository

import (
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AnnouncementRepository struct {
	db *gorm.DB
}

func NewAnnouncementRepository(db *gorm.DB) *AnnouncementRepository {
	return &AnnouncementRepository{db: db}
}

func (r *AnnouncementRepository) Create(a *model.Announcement) error {
	return r.db.Create(a).Error
}

func (r *AnnouncementRepository) Save(a *model.Announcement) error {
	return r.db.Save(a).Error
}

func (r *AnnouncementRepository) GetByUUID(uuid string) (*model.Announcement, error) {
	var a model.Announcement
	err := r.db.Where("uuid = ?", uuid).First(&a).Error
	return &a, err
}

func (r *AnnouncementRepository) Delete(uuid string) error {
	return r.db.Where("uuid = ?", uuid).Delete(&model.Announcement{}).Error
}

// visibleQuery 是「某用户可见公告」的唯一事实来源：已发布 + 未过期 + scope 命中。
func (r *AnnouncementRepository) visibleQuery(domainUUIDs []string) *gorm.DB {
	now := time.Now()
	q := r.db.Model(&model.Announcement{}).
		Where("published_at IS NOT NULL AND published_at <= ?", now).
		Where("(expires_at IS NULL OR expires_at > ?)", now)

	if len(domainUUIDs) == 0 {
		return q.Where("scope_type = ?", model.AnnouncementScopePlatform)
	}
	return q.Where(
		"((scope_type = ? AND (scope_id = '' OR scope_id IS NULL)) OR (scope_type = ? AND scope_id IN ?))",
		model.AnnouncementScopePlatform, model.AnnouncementScopeDomain, domainUUIDs,
	)
}

func (r *AnnouncementRepository) ListVisible(userUUID string, domainUUIDs []string, onlyUnread bool, page, pageSize int) ([]model.Announcement, int64, error) {
	q := r.visibleQuery(domainUUIDs)
	if onlyUnread {
		q = q.Where(
			"NOT EXISTS (SELECT 1 FROM announcement_reads ar WHERE ar.announcement_uuid = announcements.uuid AND ar.user_uuid = ?)",
			userUUID,
		)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Announcement
	err := q.Order("pinned DESC, published_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	return items, total, err
}

func (r *AnnouncementRepository) CountUnread(userUUID string, domainUUIDs []string) (int64, error) {
	var total int64
	err := r.visibleQuery(domainUUIDs).
		Where(
			"NOT EXISTS (SELECT 1 FROM announcement_reads ar WHERE ar.announcement_uuid = announcements.uuid AND ar.user_uuid = ?)",
			userUUID,
		).
		Count(&total).Error
	return total, err
}

// ReadUUIDs 返回给定公告集合中该用户已读的 uuid 集合。
func (r *AnnouncementRepository) ReadUUIDs(userUUID string, announcementUUIDs []string) (map[string]struct{}, error) {
	if len(announcementUUIDs) == 0 {
		return map[string]struct{}{}, nil
	}
	var uuids []string
	err := r.db.Model(&model.AnnouncementRead{}).
		Where("user_uuid = ? AND announcement_uuid IN ?", userUUID, announcementUUIDs).
		Pluck("announcement_uuid", &uuids).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(uuids))
	for _, u := range uuids {
		out[u] = struct{}{}
	}
	return out, nil
}

// MarkRead 幂等写入已读记录；重复调用不产生错误也不更新 read_at。
func (r *AnnouncementRepository) MarkRead(announcementUUID, userUUID string) error {
	row := model.AnnouncementRead{
		AnnouncementUUID: announcementUUID,
		UserUUID:         userUUID,
		ReadAt:           time.Now(),
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

// IsVisible 判断单条公告是否对该用户可见（已发布 + 未过期 + scope 命中）。
// MarkRead 用它做权限校验，避免翻页扫描。
func (r *AnnouncementRepository) IsVisible(announcementUUID string, domainUUIDs []string) (bool, error) {
	var count int64
	err := r.visibleQuery(domainUUIDs).
		Where("uuid = ?", announcementUUID).
		Count(&count).Error
	return count > 0, err
}

// ListAdmin 返回管理端列表（含草稿与已过期）。
// platformAdmin 为 true 时看全部；否则只看自己可管理的域公告 + 自己创建的草稿。
func (r *AnnouncementRepository) ListAdmin(userUUID string, domainUUIDs []string, platformAdmin bool, page, pageSize int) ([]model.Announcement, int64, error) {
	q := r.db.Model(&model.Announcement{})
	if !platformAdmin {
		if len(domainUUIDs) == 0 {
			q = q.Where("scope_type = ? AND created_by_uuid = ?", model.AnnouncementScopePlatform, userUUID)
		} else {
			q = q.Where(
				"(scope_type = ? AND scope_id IN ?) OR (scope_type = ? AND created_by_uuid = ?)",
				model.AnnouncementScopeDomain, domainUUIDs, model.AnnouncementScopePlatform, userUUID,
			)
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Announcement
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	return items, total, err
}
```

- [ ] **步骤 6：运行测试验证通过**

```bash
cd app/server && go test ./internal/repository/ -run TestAnnouncementRepo -v
```
预期：PASS（4 个用例）。

- [ ] **步骤 8：Commit**

```bash
git add app/server/internal/model/announcement.go app/server/internal/repository/announcement_repo.go app/server/internal/repository/announcement_repo_test.go app/server/internal/repository/db_migrations.go
git commit -m "feat(announcement): 新增公告与已读数据模型及仓库"
```

---

## 任务 3：权限码全链路

**文件：**
- 修改：`app/server/internal/permcode/permcode.go`
- 修改：`app/server/internal/model/permission.go`
- 修改：`app/server/internal/model/permission_test.go`
- 修改：`app/web/src/utils/permissions.ts`

- [ ] **步骤 1：后端权限码常量**

`app/server/internal/permcode/permcode.go` 末尾追加：

```go
	PermAnnouncementRead   = "announcement:read"
	PermAnnouncementManage = "announcement:manage"
```

- [ ] **步骤 2：model 层别名与种子**

`app/server/internal/model/permission.go`：

1. 常量区追加：

```go
	PermAnnouncementRead   = permcode.PermAnnouncementRead
	PermAnnouncementManage = permcode.PermAnnouncementManage
```

2. `DefaultPermissions` 末尾追加：

```go
	{Code: PermAnnouncementRead, Name: "查看公告", Description: "查看系统公告列表与未读数"},
	{Code: PermAnnouncementManage, Name: "管理公告", Description: "创建、编辑、删除系统公告"},
```

3. `DefaultRolePermissions["admin"]` 追加 `PermAnnouncementRead, PermAnnouncementManage`；`["user"]` **只追加** `PermAnnouncementRead`。

- [ ] **步骤 3：补权限测试**

`app/server/internal/model/permission_test.go` 末尾追加：

```go
func TestDefaultRolePermissionsContainAnnouncementCodes(t *testing.T) {
	admin := make(map[string]struct{}, len(DefaultRolePermissions["admin"]))
	for _, code := range DefaultRolePermissions["admin"] {
		admin[code] = struct{}{}
	}
	for _, code := range []string{permcode.PermAnnouncementRead, permcode.PermAnnouncementManage} {
		if _, ok := admin[code]; !ok {
			t.Errorf("admin role missing %q", code)
		}
	}

	user := make(map[string]struct{}, len(DefaultRolePermissions["user"]))
	for _, code := range DefaultRolePermissions["user"] {
		user[code] = struct{}{}
	}
	if _, ok := user[permcode.PermAnnouncementRead]; !ok {
		t.Error("user role must be able to read announcements")
	}
	if _, ok := user[permcode.PermAnnouncementManage]; ok {
		t.Error("user role must not manage announcements")
	}
}
```

- [ ] **步骤 4：运行后端测试**

```bash
cd app/server && go test ./internal/model/ -run TestDefaultRolePermissions -v
```
预期：PASS。

- [ ] **步骤 5：前端兜底权限表**

`app/web/src/utils/permissions.ts`：

1. `rolePermissions.admin` 数组追加 `"announcement:read"`, `"announcement:manage"`；
2. `rolePermissions.user` 数组追加 `"announcement:read"`；
3. `MANAGE_ENTRY_PERMISSIONS` 追加 `"announcement:manage"`。

- [ ] **步骤 6：Commit**

```bash
git add app/server/internal/permcode/permcode.go app/server/internal/model/permission.go app/server/internal/model/permission_test.go app/web/src/utils/permissions.ts
git commit -m "feat(permission): 新增公告查看与管理权限码"
```

---

## 任务 4：公告服务层

**文件：**
- 创建：`app/server/internal/service/announcement_service.go`
- 创建：`app/server/internal/service/announcement_service_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
package service

import (
	"context"
	"testing"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type announcementTestBus struct {
	namespace []string
	rooms     []string
}

func (b *announcementTestBus) PublishNamespace(_ context.Context, event string, _ interface{}) error {
	b.namespace = append(b.namespace, event)
	return nil
}

func (b *announcementTestBus) PublishRoom(_ context.Context, room, event string, _ interface{}) error {
	b.rooms = append(b.rooms, room+":"+event)
	return nil
}

func setupAnnouncementService(t *testing.T) (*AnnouncementService, *gorm.DB, *announcementTestBus) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:announcement_svc_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}, &model.DomainMember{}); err != nil {
		t.Fatal(err)
	}
	bus := &announcementTestBus{}
	svc := NewAnnouncementService(repository.NewAnnouncementRepository(db), repository.NewDomainRepository(db), db)
	svc.SetEventBus(bus)
	// 测试里用角色名近似 domain:manage：owner/admin 视为有管理权。
	svc.SetDomainPermissionChecker(func(domainUUID, userUUID, _ string) bool {
		member, err := repository.NewDomainRepository(db).GetMember(domainUUID, userUUID)
		if err != nil || member == nil {
			return false
		}
		return member.RoleName == model.DomainRoleOwner || member.RoleName == model.DomainRoleAdmin
	})
	return svc, db, bus
}

func TestAnnouncementService_PlatformPublishBroadcasts(t *testing.T) {
	svc, _, bus := setupAnnouncementService(t)
	publish := true

	dto, err := svc.Create(AnnouncementActor{UserUUID: "admin-uuid", Name: "admin", IsPlatformAdmin: true}, AnnouncementInput{
		ScopeType: model.AnnouncementScopePlatform,
		Title:     "维护公告",
		Content:   "# 维护\n今晚 22:00",
		Level:     model.AnnouncementLevelWarning,
		Publish:   &publish,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if dto.UUID == "" {
		t.Fatal("uuid missing")
	}
	if len(bus.namespace) != 1 || bus.namespace[0] != EventAnnouncementNew {
		t.Fatalf("expected namespace broadcast, got %v", bus.namespace)
	}
}

func TestAnnouncementService_DomainPublishTargetsMembersOnly(t *testing.T) {
	svc, db, bus := setupAnnouncementService(t)
	if err := db.Create(&model.DomainMember{DomainUUID: "dom-1", UserUUID: "u-1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: "dom-1", UserUUID: "u-2"}).Error; err != nil {
		t.Fatal(err)
	}
	publish := true

	_, err := svc.Create(AnnouncementActor{UserUUID: "owner-uuid", Name: "owner"}, AnnouncementInput{
		ScopeType: model.AnnouncementScopeDomain,
		ScopeID:   "dom-1",
		Title:     "域公告",
		Content:   "body",
		Publish:   &publish,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(bus.namespace) != 0 {
		t.Fatalf("domain announcement must not use namespace broadcast, got %v", bus.namespace)
	}
	if len(bus.rooms) != 2 {
		t.Fatalf("expected 2 per-member deliveries, got %v", bus.rooms)
	}
}

func TestAnnouncementService_DraftIsNotDelivered(t *testing.T) {
	svc, _, bus := setupAnnouncementService(t)
	no := false

	if _, err := svc.Create(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, AnnouncementInput{
		ScopeType: model.AnnouncementScopePlatform,
		Title:     "草稿",
		Content:   "body",
		Publish:   &no,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(bus.namespace) != 0 || len(bus.rooms) != 0 {
		t.Fatal("draft must not be delivered")
	}
}

func TestAnnouncementService_UnreadAndMarkRead(t *testing.T) {
	svc, _, _ := setupAnnouncementService(t)
	publish := true

	dto, err := svc.Create(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, AnnouncementInput{
		ScopeType: model.AnnouncementScopePlatform,
		Title:     "公告一",
		Content:   "body",
		Publish:   &publish,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	count, err := svc.UnreadCount("u-1")
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if count != 1 {
		t.Fatalf("unread = %d, want 1", count)
	}

	if err := svc.MarkRead("u-1", dto.UUID); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	count, err = svc.UnreadCount("u-1")
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if count != 0 {
		t.Fatalf("unread after read = %d, want 0", count)
	}
}

func TestAnnouncementService_MarkReadRejectsInvisible(t *testing.T) {
	svc, db, _ := setupAnnouncementService(t)
	now := time.Now()
	foreign := &model.Announcement{
		ScopeType:   model.AnnouncementScopeDomain,
		ScopeID:     "dom-x",
		Title:       "foreign",
		Content:     "body",
		PublishedAt: &now,
	}
	if err := db.Create(foreign).Error; err != nil {
		t.Fatal(err)
	}

	err := svc.MarkRead("u-1", foreign.UUID)
	if err == nil {
		t.Fatal("expected NOT_FOUND for invisible announcement")
	}
	appErr, ok := err.(*pkg.AppError)
	if !ok || appErr.Code != pkg.NOT_FOUND {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}

func TestAnnouncementService_CanManage(t *testing.T) {
	svc, db, _ := setupAnnouncementService(t)
	if err := db.Create(&model.DomainMember{DomainUUID: "dom-1", UserUUID: "u-1", RoleName: model.DomainRoleAdmin}).Error; err != nil {
		t.Fatal(err)
	}

	if !svc.CanManage(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, model.AnnouncementScopePlatform, "") {
		t.Error("platform admin should manage platform announcements")
	}
	if svc.CanManage(AnnouncementActor{UserUUID: "u-1"}, model.AnnouncementScopePlatform, "") {
		t.Error("regular user must not manage platform announcements")
	}
	if !svc.CanManage(AnnouncementActor{UserUUID: "u-1"}, model.AnnouncementScopeDomain, "dom-1") {
		t.Error("domain admin should manage its own domain announcements")
	}
	if svc.CanManage(AnnouncementActor{UserUUID: "u-1"}, model.AnnouncementScopeDomain, "dom-2") {
		t.Error("domain admin must not manage another domain")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
cd app/server && go test ./internal/service/ -run TestAnnouncementService -v
```
预期：FAIL，`undefined: NewAnnouncementService`。

- [ ] **步骤 3：编写服务实现**

```go
package service

import (
	"context"
	"log"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/notify"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

const (
	EventAnnouncementNew = "announcement:new"

	announcementDefaultPageSize = 20
	announcementMaxPageSize     = 100
	announcementMaxTitleRunes   = 200
	announcementMaxContentRunes = 20000
)

type AnnouncementDTO struct {
	UUID          string `json:"uuid"`
	ScopeType     string `json:"scope_type"`
	ScopeID       string `json:"scope_id,omitempty"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	Level         string `json:"level"`
	Pinned        bool   `json:"pinned"`
	PublishedAt   *int64 `json:"published_at,omitempty"`
	ExpiresAt     *int64 `json:"expires_at,omitempty"`
	CreatedByUUID string `json:"created_by_uuid,omitempty"`
	CreatedByName string `json:"created_by_name"`
	IsRead        bool   `json:"is_read"`
	CreatedAt     int64  `json:"created_at"`
}

type AnnouncementActor struct {
	UserUUID        string
	Name            string
	IsPlatformAdmin bool
}

type AnnouncementInput struct {
	ScopeType string
	ScopeID   string
	Title     string
	Content   string
	Level     string
	Pinned    bool
	Publish   *bool
	ExpiresAt *int64
}

type AnnouncementEventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
}

type AnnouncementService struct {
	repo       *repository.AnnouncementRepository
	domainRepo *repository.DomainRepository
	db         *gorm.DB
	bus        AnnouncementEventBus
	// domainPerm 判定域内权限码，由 DomainService.HasDomainPermission 注入。
	// 不用「角色名 == owner/admin」硬编码，否则自定义域角色拿到 domain:manage 也进不来。
	domainPerm func(domainUUID, userUUID, permCode string) bool
}

func NewAnnouncementService(repo *repository.AnnouncementRepository, domainRepo *repository.DomainRepository, db *gorm.DB) *AnnouncementService {
	return &AnnouncementService{repo: repo, domainRepo: domainRepo, db: db}
}

func (s *AnnouncementService) SetEventBus(b AnnouncementEventBus) { s.bus = b }

func (s *AnnouncementService) SetDomainPermissionChecker(fn func(domainUUID, userUUID, permCode string) bool) {
	s.domainPerm = fn
}

func toAnnouncementDTO(a *model.Announcement, isRead bool) AnnouncementDTO {
	dto := AnnouncementDTO{
		UUID:          a.UUID,
		ScopeType:     a.ScopeType,
		ScopeID:       a.ScopeID,
		Title:         a.Title,
		Content:       a.Content,
		Level:         a.Level,
		Pinned:        a.Pinned,
		CreatedByUUID: a.CreatedByUUID,
		CreatedByName: a.CreatedByName,
		IsRead:        isRead,
		CreatedAt:     a.CreatedAt.UnixMilli(),
	}
	if a.PublishedAt != nil {
		ts := a.PublishedAt.UnixMilli()
		dto.PublishedAt = &ts
	}
	if a.ExpiresAt != nil {
		ts := a.ExpiresAt.UnixMilli()
		dto.ExpiresAt = &ts
	}
	return dto
}

// CanManage 按操作对象的 scope 分叉判定，避免平台权限与域权限互相放大。
func (s *AnnouncementService) CanManage(actor AnnouncementActor, scopeType, scopeID string) bool {
	if actor.UserUUID == "" {
		return false
	}
	if scopeType == model.AnnouncementScopePlatform {
		return actor.IsPlatformAdmin
	}
	if scopeID == "" {
		return false
	}
	if actor.IsPlatformAdmin {
		return true
	}
	if s.domainPerm != nil {
		return s.domainPerm(scopeID, actor.UserUUID, permcode.PermDomainManage)
	}
	member, err := s.domainRepo.GetMember(scopeID, actor.UserUUID)
	if err != nil || member == nil {
		return false
	}
	return member.RoleName == model.DomainRoleOwner || member.RoleName == model.DomainRoleAdmin
}

func (s *AnnouncementService) userDomainUUIDs(userUUID string) ([]string, error) {
	if userUUID == "" {
		return nil, nil
	}
	return s.domainRepo.ListUserDomains(userUUID)
}

func (s *AnnouncementService) Create(actor AnnouncementActor, in AnnouncementInput) (*AnnouncementDTO, error) {
	if !s.CanManage(actor, in.ScopeType, in.ScopeID) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "insufficient announcement permission")
	}
	if in.Title == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "title is required")
	}
	if len([]rune(in.Title)) > announcementMaxTitleRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "title too long")
	}
	if in.Content == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content is required")
	}
	if len([]rune(in.Content)) > announcementMaxContentRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content too long")
	}
	if in.Level == "" {
		in.Level = model.AnnouncementLevelInfo
	}

	a := &model.Announcement{
		ScopeType:     in.ScopeType,
		ScopeID:       in.ScopeID,
		Title:         in.Title,
		Content:       in.Content,
		Level:         in.Level,
		Pinned:        in.Pinned,
		CreatedByUUID: actor.UserUUID,
		CreatedByName: actor.Name,
	}
	if in.Publish != nil && *in.Publish {
		now := time.Now()
		a.PublishedAt = &now
	}
	if in.ExpiresAt != nil && *in.ExpiresAt > 0 {
		t := time.UnixMilli(*in.ExpiresAt)
		a.ExpiresAt = &t
	}

	if err := s.repo.Create(a); err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "create announcement failed")
	}

	dto := toAnnouncementDTO(a, true)
	if a.PublishedAt != nil {
		s.deliver(a, dto)
	}
	return &dto, nil
}

func (s *AnnouncementService) Update(actor AnnouncementActor, uuid string, in AnnouncementInput) (*AnnouncementDTO, error) {
	a, err := s.repo.GetByUUID(uuid)
	if err != nil || a == nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "announcement not found")
	}
	if !s.CanManage(actor, a.ScopeType, a.ScopeID) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "insufficient announcement permission")
	}
	if in.Title != "" {
		a.Title = in.Title
	}
	if in.Content != "" {
		a.Content = in.Content
	}
	if in.Level != "" {
		a.Level = in.Level
	}
	a.Pinned = in.Pinned
	if in.ExpiresAt != nil {
		if *in.ExpiresAt > 0 {
			t := time.UnixMilli(*in.ExpiresAt)
			a.ExpiresAt = &t
		} else {
			a.ExpiresAt = nil
		}
	}

	wasPublished := a.PublishedAt != nil
	if in.Publish != nil && *in.Publish && !wasPublished {
		now := time.Now()
		a.PublishedAt = &now
	}

	if err := s.repo.Save(a); err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "update announcement failed")
	}

	dto := toAnnouncementDTO(a, false)
	if !wasPublished && a.PublishedAt != nil {
		s.deliver(a, dto)
	}
	return &dto, nil
}

func (s *AnnouncementService) Delete(actor AnnouncementActor, uuid string) error {
	a, err := s.repo.GetByUUID(uuid)
	if err != nil || a == nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "announcement not found")
	}
	if !s.CanManage(actor, a.ScopeType, a.ScopeID) {
		return pkg.NewAppError(pkg.FORBIDDEN, "insufficient announcement permission")
	}
	return s.repo.Delete(uuid)
}

func (s *AnnouncementService) List(userUUID string, onlyUnread bool, page, pageSize int) ([]AnnouncementDTO, int64, error) {
	page, pageSize = normalizeAnnouncementPaging(page, pageSize)
	domains, err := s.userDomainUUIDs(userUUID)
	if err != nil {
		return nil, 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "load user domains failed")
	}
	items, total, err := s.repo.ListVisible(userUUID, domains, onlyUnread, page, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "list announcements failed")
	}

	uuids := make([]string, 0, len(items))
	for i := range items {
		uuids = append(uuids, items[i].UUID)
	}
	readSet, err := s.repo.ReadUUIDs(userUUID, uuids)
	if err != nil {
		return nil, 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "load read states failed")
	}

	out := make([]AnnouncementDTO, 0, len(items))
	for i := range items {
		_, read := readSet[items[i].UUID]
		out = append(out, toAnnouncementDTO(&items[i], read))
	}
	return out, total, nil
}

func (s *AnnouncementService) UnreadCount(userUUID string) (int64, error) {
	domains, err := s.userDomainUUIDs(userUUID)
	if err != nil {
		return 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "load user domains failed")
	}
	count, err := s.repo.CountUnread(userUUID, domains)
	if err != nil {
		return 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "count unread failed")
	}
	return count, nil
}

func (s *AnnouncementService) MarkRead(userUUID, announcementUUID string) error {
	if announcementUUID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "uuid is required")
	}
	domains, err := s.userDomainUUIDs(userUUID)
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "load user domains failed")
	}
	visible, err := s.repo.IsVisible(announcementUUID, domains)
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "check visibility failed")
	}
	if !visible {
		return pkg.NewAppError(pkg.NOT_FOUND, "announcement not found")
	}
	if err := s.repo.MarkRead(announcementUUID, userUUID); err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "mark read failed")
	}
	return nil
}

func (s *AnnouncementService) ListAdmin(actor AnnouncementActor, page, pageSize int) ([]AnnouncementDTO, int64, error) {
	page, pageSize = normalizeAnnouncementPaging(page, pageSize)
	var domains []string
	if !actor.IsPlatformAdmin {
		owned, err := s.userDomainUUIDs(actor.UserUUID)
		if err != nil {
			return nil, 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "load user domains failed")
		}
		managed := make([]string, 0, len(owned))
		for _, d := range owned {
			if s.CanManage(actor, model.AnnouncementScopeDomain, d) {
				managed = append(managed, d)
			}
		}
		domains = managed
	}

	items, total, err := s.repo.ListAdmin(actor.UserUUID, domains, actor.IsPlatformAdmin, page, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "list announcements failed")
	}
	out := make([]AnnouncementDTO, 0, len(items))
	for i := range items {
		out = append(out, toAnnouncementDTO(&items[i], true))
	}
	return out, total, nil
}

func normalizeAnnouncementPaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = announcementDefaultPageSize
	}
	if pageSize > announcementMaxPageSize {
		pageSize = announcementMaxPageSize
	}
	return page, pageSize
}

// deliver 只做在线推送；失败只记日志，公告已落库，前端重连后靠 REST 兜底。
func (s *AnnouncementService) deliver(a *model.Announcement, dto AnnouncementDTO) {
	if s.bus == nil {
		return
	}
	ctx := context.Background()
	if a.ScopeType == model.AnnouncementScopePlatform {
		if err := notify.NotifyAll(ctx, s.bus, EventAnnouncementNew, dto); err != nil {
			log.Printf("[Announcement] broadcast failed: %v", err)
		}
		return
	}
	members, err := s.domainRepo.ListMembers(a.ScopeID)
	if err != nil {
		log.Printf("[Announcement] list domain members failed scope=%s: %v", a.ScopeID, err)
		return
	}
	for i := range members {
		if err := notify.NotifyUser(ctx, s.bus, members[i].UserUUID, EventAnnouncementNew, dto); err != nil {
			log.Printf("[Announcement] notify user %s failed: %v", members[i].UserUUID, err)
		}
	}
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
cd app/server && go test ./internal/service/ -run TestAnnouncementService -v
```
预期：PASS（6 个用例）。

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/service/announcement_service.go app/server/internal/service/announcement_service_test.go
git commit -m "feat(announcement): 公告服务层（可见性/未读/权限分叉/投递）"
```

---

## 任务 5：HTTP 处理器、路由与装配

**文件：**
- 创建：`app/server/internal/handler/announcement_handler.go`
- 创建：`app/server/internal/router/routes/announcement/routes.go`
- 修改：`app/server/internal/router/router.go`
- 修改：`app/server/server/gin.go`

- [ ] **步骤 1：编写 handler**

```go
package handler

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type AnnouncementHandler struct {
	svc     *service.AnnouncementService
	permSvc *service.PermissionService
}

func NewAnnouncementHandler(svc *service.AnnouncementService, permSvc *service.PermissionService) *AnnouncementHandler {
	return &AnnouncementHandler{svc: svc, permSvc: permSvc}
}

func (h *AnnouncementHandler) actor(c *gin.Context) (service.AnnouncementActor, bool) {
	userUUID := currentUserUUID(c)
	if userUUID == "" {
		return service.AnnouncementActor{}, false
	}
	name, _ := c.Get("username")
	nameStr, _ := name.(string)
	return service.AnnouncementActor{
		UserUUID:        userUUID,
		Name:            nameStr,
		IsPlatformAdmin: h.permSvc != nil && h.permSvc.HasPermission(roleFromContext(c), permcode.PermAnnouncementManage),
	}, true
}

// List
// @Summary      公告列表
// @Description  返回当前用户可见的系统公告，含已读状态
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{only_unread=bool,page=int,page_size=int}  false  "查询条件"
// @Success      200      {object}  pkg.Response
// @Router       /announcement/list [post]
func (h *AnnouncementHandler) List(c *gin.Context) {
	var req struct {
		OnlyUnread bool `json:"only_unread"`
		Page       int  `json:"page"`
		PageSize   int  `json:"page_size"`
	}
	_ = c.ShouldBindJSON(&req)

	userUUID := currentUserUUID(c)
	items, total, err := h.svc.List(userUUID, req.OnlyUnread, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items, "total": total})
}

// UnreadCount
// @Summary      公告未读数
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /announcement/unread-count [post]
func (h *AnnouncementHandler) UnreadCount(c *gin.Context) {
	count, err := h.svc.UnreadCount(currentUserUUID(c))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"count": count})
}

// MarkRead
// @Summary      标记公告已读
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{uuid=string}  true  "公告 UUID"
// @Success      200  {object}  pkg.Response
// @Router       /announcement/mark-read [post]
func (h *AnnouncementHandler) MarkRead(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.MarkRead(currentUserUUID(c), req.UUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"ok": true})
}

// AdminList
// @Summary      管理端公告列表
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{page=int,page_size=int}  false  "分页"
// @Success      200  {object}  pkg.Response
// @Router       /announcement/admin/list [post]
func (h *AnnouncementHandler) AdminList(c *gin.Context) {
	var req struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	_ = c.ShouldBindJSON(&req)

	actor, ok := h.actor(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	if !actor.IsPlatformAdmin {
		if !h.hasAnyDomainManage(c) {
			pkg.Fail(c, pkg.FORBIDDEN)
			return
		}
	}
	items, total, err := h.svc.ListAdmin(actor, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items, "total": total})
}

// hasAnyDomainManage 判断用户是否至少管理一个域（域级公告的发布入口条件）。
func (h *AnnouncementHandler) hasAnyDomainManage(c *gin.Context) bool {
	if h.permSvc == nil {
		return false
	}
	return h.permSvc.HasPermission(roleFromContext(c), permcode.PermDomainManage)
}

type announcementInputBody struct {
	ScopeType string `json:"scope_type"`
	ScopeID   string `json:"scope_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Level     string `json:"level"`
	Pinned    bool   `json:"pinned"`
	Publish   *bool  `json:"publish"`
	ExpiresAt *int64 `json:"expires_at"`
}

func (b announcementInputBody) toInput() service.AnnouncementInput {
	return service.AnnouncementInput{
		ScopeType: b.ScopeType,
		ScopeID:   b.ScopeID,
		Title:     b.Title,
		Content:   b.Content,
		Level:     b.Level,
		Pinned:    b.Pinned,
		Publish:   b.Publish,
		ExpiresAt: b.ExpiresAt,
	}
}

// AdminCreate
// @Summary      创建公告
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{scope_type=string,scope_id=string,title=string,content=string,level=string,pinned=bool,publish=bool,expires_at=int64}  true  "公告内容"
// @Success      200  {object}  pkg.Response
// @Router       /announcement/admin/create [post]
func (h *AnnouncementHandler) AdminCreate(c *gin.Context) {
	var req announcementInputBody
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.ScopeType == "" {
		req.ScopeType = model.AnnouncementScopePlatform
	}
	actor, ok := h.actor(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	dto, err := h.svc.Create(actor, req.toInput())
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, dto)
}

// AdminUpdate
// @Summary      更新公告
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{uuid=string,title=string,content=string,level=string,pinned=bool,publish=bool,expires_at=int64}  true  "公告内容"
// @Success      200  {object}  pkg.Response
// @Router       /announcement/admin/update [post]
func (h *AnnouncementHandler) AdminUpdate(c *gin.Context) {
	var req struct {
		announcementInputBody
		UUID string `json:"uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	dto, err := h.svc.Update(actor, req.UUID, req.toInput())
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, dto)
}

// AdminDelete
// @Summary      删除公告
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{uuid=string}  true  "公告 UUID"
// @Success      200  {object}  pkg.Response
// @Router       /announcement/admin/delete [post]
func (h *AnnouncementHandler) AdminDelete(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	if err := h.svc.Delete(actor, req.UUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"ok": true})
}
```

- [ ] **步骤 2：路由注册**

`app/server/internal/router/routes/announcement/routes.go`：

```go
package announcement

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.AnnouncementHandler) {
	r.POST("/list", h.List)
	r.POST("/unread-count", h.UnreadCount)
	r.POST("/mark-read", h.MarkRead)

	admin := r.Group("/admin")
	admin.POST("/list", h.AdminList)
	admin.POST("/create", h.AdminCreate)
	admin.POST("/update", h.AdminUpdate)
	admin.POST("/delete", h.AdminDelete)
}
```

- [ ] **步骤 3：接进 router**

`app/server/internal/router/router.go`：

1. import 区追加 `announcementRoutes "GOSpeak/internal/router/routes/announcement"`；
2. `Handlers` 结构体末尾追加 `Announcement *handler.AnnouncementHandler`；
3. `protected` 注册区（`auditRoutes.RegisterProtected(...)` 之后）追加：

```go
	if h.Announcement != nil {
		announcementRoutes.RegisterProtected(protected.Group("/announcement"), h.Announcement)
	}
```

- [ ] **步骤 4：接进依赖注入**

`app/server/server/gin.go`：

1. repo 构造区（`domainRoleRepo := ...` 之后）追加：

```go
	announcementRepo := repository.NewAnnouncementRepository(db)
```

2. handler 构造区（`auditH := handler.NewAuditHandler(auditSvc)` 之后）追加：

```go
	announcementSvc := service.NewAnnouncementService(announcementRepo, domainRepo, db)
	announcementSvc.SetEventBus(eventBus)
	announcementSvc.SetDomainPermissionChecker(domainSvc.HasDomainPermission)
	announcementH := handler.NewAnnouncementHandler(announcementSvc, permSvc)
```

3. `router.SetupRoutes` 的 `Handlers` 字面量追加 `Announcement: announcementH,`

- [ ] **步骤 5：编译并跑服务端全量单测**

```bash
cd app/server && go build ./... && go test ./internal/... 2>&1 | tail -30
```
预期：build 通过，测试全 PASS。

- [ ] **步骤 6：Commit**

```bash
git add app/server/internal/handler/announcement_handler.go app/server/internal/router/routes/announcement/ app/server/internal/router/router.go app/server/server/gin.go
git commit -m "feat(announcement): HTTP 端点、路由与依赖装配"
```

---

## 任务 6：后端集成测试

**文件：**
- 创建：`test/announcement/announcement.test.ts`

- [ ] **步骤 1：编写集成测试**

```ts
import { describe, expect, it } from "vitest";
import {
  api,
  connectWS,
  createDomain,
  getAdminToken,
  joinDomain,
  registerUser,
} from "../helpers";

interface AnnouncementDTO {
  uuid: string;
  title: string;
  scope_type: string;
  scope_id?: string;
  is_read: boolean;
  level: string;
}

async function publishPlatform(token: string, title: string, publish = true) {
  const res = await api<AnnouncementDTO>("/api/v1/announcement/admin/create", {
    token,
    body: {
      scope_type: "platform",
      title,
      content: "# 标题\n正文 **加粗**",
      level: "info",
      publish,
    },
  });
  expect(res.code).toBe(0);
  return res.data!;
}

describe("announcement module", () => {
  it("publishes, lists, counts and marks read", async () => {
    const adminToken = await getAdminToken();
    const user = await registerUser("ann");
    const title = `集成公告_${Date.now()}`;
    const created = await publishPlatform(adminToken, title);
    expect(created.uuid).toBeTruthy();

    const list = await api<{ items: AnnouncementDTO[]; total: number }>(
      "/api/v1/announcement/list",
      { token: user.access_token, body: {} },
    );
    expect(list.code).toBe(0);
    const found = list.data?.items.find((item) => item.uuid === created.uuid);
    expect(found?.title).toBe(title);
    expect(found?.is_read).toBe(false);

    const before = await api<{ count: number }>(
      "/api/v1/announcement/unread-count",
      { token: user.access_token, body: {} },
    );
    expect(before.data!.count).toBeGreaterThanOrEqual(1);

    const marked = await api("/api/v1/announcement/mark-read", {
      token: user.access_token,
      body: { uuid: created.uuid },
    });
    expect(marked.code).toBe(0);

    const after = await api<{ count: number }>(
      "/api/v1/announcement/unread-count",
      { token: user.access_token, body: {} },
    );
    expect(after.data!.count).toBe(before.data!.count - 1);

    // 幂等：重复标记不报错、不再减
    const again = await api("/api/v1/announcement/mark-read", {
      token: user.access_token,
      body: { uuid: created.uuid },
    });
    expect(again.code).toBe(0);
    const afterAgain = await api<{ count: number }>(
      "/api/v1/announcement/unread-count",
      { token: user.access_token, body: {} },
    );
    expect(afterAgain.data!.count).toBe(after.data!.count);
  });

  it("rejects publishing from a regular user", async () => {
    const user = await registerUser("ann");
    const res = await api("/api/v1/announcement/admin/create", {
      token: user.access_token,
      body: { scope_type: "platform", title: "hack", content: "x", publish: true },
    });
    expect(res.code).toBe(1013);
  });

  it("keeps drafts invisible and domain announcements scoped", async () => {
    const adminToken = await getAdminToken();
    const owner = await registerUser("ann");
    const outsider = await registerUser("ann");
    const domain = await createDomain(owner.access_token, `ann_dom_${Date.now()}`);

    const draftTitle = `草稿_${Date.now()}`;
    await publishPlatform(adminToken, draftTitle, false);
    const ownerList = await api<{ items: AnnouncementDTO[] }>(
      "/api/v1/announcement/list",
      { token: owner.access_token, body: {} },
    );
    expect(ownerList.data?.items.some((i) => i.title === draftTitle)).toBe(false);

    const domainTitle = `域公告_${Date.now()}`;
    const created = await api<AnnouncementDTO>("/api/v1/announcement/admin/create", {
      token: owner.access_token,
      body: {
        scope_type: "domain",
        scope_id: domain.uuid,
        title: domainTitle,
        content: "域内可见",
        publish: true,
      },
    });
    expect(created.code).toBe(0);

    const inDomain = await api<{ items: AnnouncementDTO[] }>(
      "/api/v1/announcement/list",
      { token: owner.access_token, body: {} },
    );
    expect(inDomain.data?.items.some((i) => i.uuid === created.data!.uuid)).toBe(true);

    const outDomain = await api<{ items: AnnouncementDTO[] }>(
      "/api/v1/announcement/list",
      { token: outsider.access_token, body: {} },
    );
    expect(outDomain.data?.items.some((i) => i.uuid === created.data!.uuid)).toBe(false);

    const deniedMark = await api("/api/v1/announcement/mark-read", {
      token: outsider.access_token,
      body: { uuid: created.data!.uuid },
    });
    expect(deniedMark.code).toBe(3001);
  });

  it("pushes announcement:new over websocket with an object payload", async () => {
    const adminToken = await getAdminToken();
    const user = await registerUser("ann");
    const ws = await connectWS(user.access_token);
    try {
      const title = `推送_${Date.now()}`;
      const created = await publishPlatform(adminToken, title);
      const payload = (await ws.waitForEvent("announcement:new")) as AnnouncementDTO;
      expect(payload.uuid).toBe(created.uuid);
      expect(payload.title).toBe(title);
      expect(typeof payload).toBe("object");
    } finally {
      ws.close();
    }
  });
});
```

- [ ] **步骤 2：运行集成测试**

```bash
pnpm test:server -- test/announcement/announcement.test.ts
```
预期：4 个用例全 PASS。若 `connectWS` 签名不符，查看 `test/helpers/ws.ts` 的实际导出并调整。

- [ ] **步骤 3：Commit**

```bash
git add test/announcement/
git commit -m "test(announcement): 集成测试覆盖发布/可见性/已读/WS 推送"
```

---

## 任务 7：前端 Badge 组件

**文件：**
- 创建：`packages/components/src/components/badge.tsx`
- 创建：`packages/components/src/components/badge.test.tsx`
- 修改：`packages/components/src/index.ts`
- 修改：`app/web/src/components/room/components/roomItem.tsx:256-263`
- 修改：`app/web/src/components/chat/conversationList.tsx:65-69`

- [ ] **步骤 1：编写失败的测试**

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Badge } from "./badge";

describe("Badge", () => {
	it("renders nothing when count is 0", () => {
		const { container } = render(<Badge count={0} />);
		expect(container.firstChild).toBeNull();
	});

	it("renders the count", () => {
		render(<Badge count={7} />);
		expect(screen.getByText("7")).toBeTruthy();
	});

	it("caps at 99+", () => {
		render(<Badge count={150} />);
		expect(screen.getByText("99+")).toBeTruthy();
	});

	it("renders a dot when dot is set", () => {
		const { container } = render(<Badge dot />);
		expect(container.querySelector("[data-slot='badge-dot']")).toBeTruthy();
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

```bash
pnpm --filter @gospeak/components test
```
预期：FAIL，找不到 `./badge`。

- [ ] **步骤 3：编写组件**

```tsx
import { cn } from "../lib/utils";

export interface BadgeProps {
	/** 未读数量；<= 0 时不渲染。 */
	count?: number;
	/** 超过该值显示 `N+`，默认 99。 */
	max?: number;
	/** 只渲染小圆点，不显示数字。 */
	dot?: boolean;
	className?: string;
	title?: string;
}

/**
 * 通用未读徽章。数字态 / `99+` / dot 三态。
 *
 * 定位由调用方决定（通常绝对定位到图标右上角），组件本身只管外观。
 */
export function Badge({ count = 0, max = 99, dot, className, title }: BadgeProps) {
	if (dot) {
		return (
			<span
				data-slot="badge-dot"
				aria-hidden="true"
				className={cn(
					"bg-destructive inline-block size-2 rounded-full",
					className,
				)}
			/>
		);
	}

	if (count <= 0) return null;
	const text = count > max ? `${max}+` : String(count);

	return (
		<span
			data-slot="badge"
			title={title}
			className={cn(
				"bg-primary text-primary-foreground inline-flex min-w-4 items-center justify-center rounded-full px-1 text-2xs font-semibold leading-4",
				className,
			)}
		>
			{text}
		</span>
	);
}
```

- [ ] **步骤 4：导出**

`packages/components/src/index.ts` 追加：

```ts
export { Badge, type BadgeProps } from "./components/badge";
```

- [ ] **步骤 5：运行测试验证通过**

```bash
pnpm --filter @gospeak/components test
```
预期：PASS。

- [ ] **步骤 6：迁移两处重复 badge**

`app/web/src/components/room/components/roomItem.tsx` 第 256-263 行：

```tsx
							{room.type === "text" && unread > 0 && (
								<Badge count={unread} title={`${unread} 条未读消息`} />
							)}
```
并在文件顶部 import 区加 `import { Badge } from "@gospeak/components";`（若已有 `@gospeak/components` import 则合并）。

`app/web/src/components/chat/conversationList.tsx` 第 65-69 行：

```tsx
									{conv.unread_count > 0 && (
										<Badge count={conv.unread_count} className="ml-1 shrink-0" />
									)}
```
同样补 import。

- [ ] **步骤 7：类型检查**

```bash
pnpm --filter @gospeak/web typecheck
```
预期：无错误。

- [ ] **步骤 8：Commit**

```bash
git add packages/components/src/components/badge.tsx packages/components/src/components/badge.test.tsx packages/components/src/index.ts app/web/src/components/room/components/roomItem.tsx app/web/src/components/chat/conversationList.tsx
git commit -m "feat(components): 新增 Badge 组件并替换两处重复内联徽章"
```

---

## 任务 8：前端通知 store、API 与 WS 接入

**文件：**
- 创建：`app/web/src/protocol/announcement.ts`
- 创建：`app/web/src/api/announcement.ts`
- 创建：`app/web/src/stores/notificationStore.ts`
- 创建：`app/web/src/stores/notificationStore.test.ts`
- 修改：`app/web/src/socket/events.ts`
- 修改：`app/web/src/socket/socketEvents.ts`
- 修改：`app/web/src/stores/socketStore.ts`

- [ ] **步骤 1：DTO 与 API**

`app/web/src/protocol/announcement.ts`：

```ts
/** 系统公告 DTO — 与后端 service.AnnouncementDTO 对齐 */
export interface AnnouncementDTO {
	uuid: string;
	scope_type: "platform" | "domain";
	scope_id?: string;
	title: string;
	content: string;
	level: "info" | "warning" | "critical";
	pinned: boolean;
	published_at?: number;
	expires_at?: number;
	created_by_uuid?: string;
	created_by_name: string;
	is_read: boolean;
	created_at: number;
}

export interface AnnouncementListResult {
	items: AnnouncementDTO[];
	total: number;
}

export interface AnnouncementInput {
	uuid?: string;
	scope_type: "platform" | "domain";
	scope_id?: string;
	title: string;
	content: string;
	level: "info" | "warning" | "critical";
	pinned?: boolean;
	publish?: boolean;
	expires_at?: number | null;
}
```

`app/web/src/api/announcement.ts`：

```ts
import type {
	AnnouncementDTO,
	AnnouncementInput,
	AnnouncementListResult,
} from "@/protocol/announcement";
import apiClient from "./apiClient";

export async function listAnnouncements(
	onlyUnread = false,
	page = 1,
	pageSize = 20,
): Promise<AnnouncementListResult> {
	const data = await apiClient.post<AnnouncementListResult>({
		url: "/api/v1/announcement/list",
		data: { only_unread: onlyUnread, page, page_size: pageSize },
		skipErrorToast: true,
	});
	return data ?? { items: [], total: 0 };
}

export async function getUnreadCount(): Promise<number> {
	const data = await apiClient.post<{ count: number }>({
		url: "/api/v1/announcement/unread-count",
		data: {},
		skipErrorToast: true,
	});
	return data?.count ?? 0;
}

export async function markAnnouncementRead(uuid: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/announcement/mark-read",
		data: { uuid },
	});
}

export async function adminListAnnouncements(
	page = 1,
	pageSize = 50,
): Promise<AnnouncementListResult> {
	const data = await apiClient.post<AnnouncementListResult>({
		url: "/api/v1/announcement/admin/list",
		data: { page, page_size: pageSize },
	});
	return data ?? { items: [], total: 0 };
}

export async function createAnnouncement(
	input: AnnouncementInput,
): Promise<AnnouncementDTO> {
	const data = await apiClient.post<AnnouncementDTO>({
		url: "/api/v1/announcement/admin/create",
		data: input,
	});
	if (!data) throw new Error("announcement missing");
	return data;
}

export async function updateAnnouncement(
	input: AnnouncementInput,
): Promise<AnnouncementDTO> {
	const data = await apiClient.post<AnnouncementDTO>({
		url: "/api/v1/announcement/admin/update",
		data: input,
	});
	if (!data) throw new Error("announcement missing");
	return data;
}

export async function deleteAnnouncement(uuid: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/announcement/admin/delete",
		data: { uuid },
	});
}
```

- [ ] **步骤 2：编写失败的 store 测试**

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/announcement", () => ({
	listAnnouncements: vi.fn(async () => ({ items: [], total: 0 })),
	getUnreadCount: vi.fn(async () => 3),
	markAnnouncementRead: vi.fn(async () => {}),
}));

import { getUnreadCount, markAnnouncementRead } from "@/api/announcement";
import { notificationStore, useNotificationStore } from "./notificationStore";

const dto = (uuid: string, isRead = false) => ({
	uuid,
	scope_type: "platform" as const,
	title: `t-${uuid}`,
	content: "body",
	level: "info" as const,
	pinned: false,
	created_by_name: "admin",
	is_read: isRead,
	created_at: Date.now(),
});

describe("notificationStore", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		useNotificationStore.setState({ unreadCount: 0, items: [], loading: false });
	});

	it("loads unread count", async () => {
		await notificationStore.loadUnreadCount();
		expect(useNotificationStore.getState().unreadCount).toBe(3);
		expect(getUnreadCount).toHaveBeenCalled();
	});

	it("increments on incoming and prepends", () => {
		notificationStore.handleIncoming(dto("a"));
		const state = useNotificationStore.getState();
		expect(state.unreadCount).toBe(1);
		expect(state.items[0].uuid).toBe("a");
	});

	it("does not double count the same uuid", () => {
		notificationStore.handleIncoming(dto("a"));
		notificationStore.handleIncoming(dto("a"));
		expect(useNotificationStore.getState().unreadCount).toBe(1);
	});

	it("optimistically marks read and calls the API", async () => {
		useNotificationStore.setState({
			unreadCount: 1,
			items: [dto("a")],
			loading: false,
		});
		await notificationStore.markRead("a");
		const state = useNotificationStore.getState();
		expect(state.unreadCount).toBe(0);
		expect(state.items[0].is_read).toBe(true);
		expect(markAnnouncementRead).toHaveBeenCalledWith("a");
	});

	it("rolls back when the API fails", async () => {
		vi.mocked(markAnnouncementRead).mockRejectedValueOnce(new Error("boom"));
		useNotificationStore.setState({
			unreadCount: 1,
			items: [dto("a")],
			loading: false,
		});
		await notificationStore.markRead("a");
		const state = useNotificationStore.getState();
		expect(state.unreadCount).toBe(1);
		expect(state.items[0].is_read).toBe(false);
	});
});
```

- [ ] **步骤 3：运行测试验证失败**

```bash
pnpm --filter @gospeak/web test -- notificationStore
```
预期：FAIL，找不到 `./notificationStore`。

- [ ] **步骤 4：编写 store**

```ts
import { toast } from "@gospeak/components";
import { create } from "zustand";
import {
	getUnreadCount,
	listAnnouncements,
	markAnnouncementRead,
} from "@/api/announcement";
import type { AnnouncementDTO } from "@/protocol/announcement";

interface NotificationState {
	unreadCount: number;
	items: AnnouncementDTO[];
	loading: boolean;
}

export const useNotificationStore = create<NotificationState>(() => ({
	unreadCount: 0,
	items: [],
	loading: false,
}));

/** 重连后必须重新拉一次：WS 不重放断线期间的事件。 */
async function loadUnreadCount(): Promise<void> {
	try {
		const count = await getUnreadCount();
		useNotificationStore.setState({ unreadCount: count });
	} catch {
		// 未登录或网络失败：保持当前值，等下次连接成功再拉
	}
}

async function loadList(onlyUnread = false): Promise<void> {
	useNotificationStore.setState({ loading: true });
	try {
		const result = await listAnnouncements(onlyUnread);
		useNotificationStore.setState({ items: result.items, loading: false });
	} catch {
		useNotificationStore.setState({ loading: false });
	}
}

function handleIncoming(dto: AnnouncementDTO): void {
	const state = useNotificationStore.getState();
	if (state.items.some((item) => item.uuid === dto.uuid)) return;
	useNotificationStore.setState({
		items: [dto, ...state.items],
		unreadCount: state.unreadCount + 1,
	});
	toast.info(dto.title);
}

async function markRead(uuid: string): Promise<void> {
	const before = useNotificationStore.getState();
	const target = before.items.find((item) => item.uuid === uuid);
	if (!target || target.is_read) return;

	useNotificationStore.setState({
		unreadCount: Math.max(0, before.unreadCount - 1),
		items: before.items.map((item) =>
			item.uuid === uuid ? { ...item, is_read: true } : item,
		),
	});

	try {
		await markAnnouncementRead(uuid);
	} catch {
		useNotificationStore.setState({
			unreadCount: before.unreadCount,
			items: before.items,
		});
		toast.error("标记已读失败");
	}
}

function reset(): void {
	useNotificationStore.setState({ unreadCount: 0, items: [], loading: false });
}

export const notificationStore = {
	unreadCount: () => useNotificationStore.getState().unreadCount,
	items: () => useNotificationStore.getState().items,
	loading: () => useNotificationStore.getState().loading,
	loadUnreadCount,
	loadList,
	handleIncoming,
	markRead,
	reset,
};

export default notificationStore;
```

- [ ] **步骤 5：运行测试验证通过**

```bash
pnpm --filter @gospeak/web test -- notificationStore
```
预期：PASS（5 个用例）。

- [ ] **步骤 6：事件常量**

`app/web/src/socket/events.ts` 在 `TYPING` 之后追加：

```ts
	// 系统公告（服务端推送，平台公告广播 / 域公告定向）
	ANNOUNCEMENT_NEW: "announcement:new",
```

- [ ] **步骤 7：绑定事件**

`app/web/src/socket/socketEvents.ts`：

1. `SocketEventDeps` 接口追加：

```ts
	handleAnnouncementNew: (dto: AnnouncementDTO) => void;
```

2. 顶部 import 追加 `import type { AnnouncementDTO } from "@/protocol/announcement";`

3. `bindServerEvents` 末尾（`TYPING` 绑定之后）追加：

```ts
	// 系统公告：平台公告走 namespace 广播，域公告定向投递到 __user 房间
	adapter.onServerEvent(EVENTS.ANNOUNCEMENT_NEW, (dto: AnnouncementDTO) => {
		deps.handleAnnouncementNew(dto);
	});
```

- [ ] **步骤 8：接进 socketStore（含 onConnected 兜底）**

`app/web/src/stores/socketStore.ts`：

1. 顶部 import 追加：

```ts
import { notificationStore } from "@/stores/notificationStore";
```

> `notificationStore` 不反向 import `socketStore`，因此这里可以直接依赖，不需要像 chatStore 那样走回调注册。

2. `bindServerEvents(adapter, {...})` 的 deps 里追加：

```ts
				handleAnnouncementNew: (dto) => notificationStore.handleIncoming(dto),
```

3. 在既有 `adapter.onConnected(() => { ... })` 回调体末尾追加：

```ts
	// WS 不重放断线期间的事件：连接（含重连）成功后必须拉一次未读数兜底。
	void notificationStore.loadUnreadCount();
```

4. 门面 `socketStore` 对象追加：

```ts
	onConnected: (cb: () => void) => adapter.onConnected(cb),
```

5. `disconnect()` 内追加 `notificationStore.reset();`

- [ ] **步骤 9：类型检查与测试**

```bash
pnpm --filter @gospeak/web typecheck && pnpm --filter @gospeak/web test
```
预期：类型无误，测试全 PASS。

- [ ] **步骤 10：Commit**

```bash
git add app/web/src/protocol/announcement.ts app/web/src/api/announcement.ts app/web/src/stores/notificationStore.ts app/web/src/stores/notificationStore.test.ts app/web/src/socket/events.ts app/web/src/socket/socketEvents.ts app/web/src/stores/socketStore.ts
git commit -m "feat(web): 通知 store、API 与 WS 事件接入（含重连兜底）"
```

---

## 任务 9：侧栏铃铛与通知弹窗

**文件：**
- 创建：`app/web/src/components/modal/notificationModal.tsx`
- 创建：`app/web/src/components/modal/notificationModal.test.tsx`
- 修改：`app/web/src/components/common/optionSquare.tsx`
- 修改：`app/web/src/layouts/common/sidebar.tsx`
- 修改：`app/web/src/layouts/layout.tsx`

- [ ] **步骤 1：OptionSquare 加 badge 插槽**

`app/web/src/components/common/optionSquare.tsx`：

1. `OptionSquareProps` 追加：

```ts
	/** 右上角未读徽章数量；0 或不传则不渲染 */
	badge?: number;
```

2. 解构参数追加 `badge`
3. 在 `{children}` 之后、`{active && ...}` 之前插入：

```tsx
				{badge && badge > 0 ? (
					<span className="absolute -top-1 -right-1">
						<Badge count={badge} />
					</span>
				) : null}
```
4. 顶部 import 追加 `import { Badge } from "@gospeak/components";`

- [ ] **步骤 2：通知弹窗组件**

`app/web/src/components/modal/notificationModal.tsx`：

```tsx
import { Dialog, DialogContent, DialogTitle } from "@gospeak/components";
import { Bell, X } from "lucide-react";
import { useEffect, useState } from "react";
import MarkdownText from "@/components/textRoom/MarkdownText";
import type { AnnouncementDTO } from "@/protocol/announcement";
import { notificationStore, useNotificationStore } from "@/stores/notificationStore";

const LEVEL_STYLES: Record<AnnouncementDTO["level"], string> = {
	info: "text-info",
	warning: "text-warning",
	critical: "text-destructive",
};

function formatTime(ms: number): string {
	const date = new Date(ms);
	return `${date.getMonth() + 1}/${date.getDate()} ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

interface NotificationModalProps {
	open: boolean;
	onClose: () => void;
}

export default function NotificationModal({ open, onClose }: NotificationModalProps) {
	const items = useNotificationStore((s) => s.items);
	const loading = useNotificationStore((s) => s.loading);
	const [selected, setSelected] = useState<string | null>(null);

	useEffect(() => {
		if (!open) return;
		void notificationStore.loadList();
	}, [open]);

	const current = items.find((item) => item.uuid === selected) ?? items[0];

	if (!open) return null;

	const select = (item: AnnouncementDTO) => {
		setSelected(item.uuid);
		void notificationStore.markRead(item.uuid);
	};

	return (
		<Dialog open onOpenChange={(v) => !v && onClose()}>
			<DialogContent
				showClose={false}
				className="flex h-[min(80vh,640px)] max-w-4xl flex-col gap-0 overflow-hidden p-0"
			>
				<div className="border-border flex items-center justify-between border-b px-4 py-3">
					<div className="flex items-center gap-2">
						<Bell size={18} />
						<DialogTitle className="text-base font-semibold">通知</DialogTitle>
					</div>
					<button
						type="button"
						onClick={onClose}
						className="hover:bg-muted flex size-7 items-center justify-center rounded-sm"
						aria-label="关闭"
					>
						<X size={16} />
					</button>
				</div>

				<div className="flex min-h-0 flex-1">
					<div className="border-border w-72 shrink-0 overflow-y-auto border-r">
						{loading && items.length === 0 ? (
							<div className="text-faint-foreground p-4 text-sm">加载中...</div>
						) : items.length === 0 ? (
							<div className="text-faint-foreground p-8 text-center text-sm">
								暂无通知
							</div>
						) : (
							items.map((item) => (
								<button
									key={item.uuid}
									type="button"
									onClick={() => select(item)}
									className={`border-border hover:bg-muted flex w-full flex-col items-start gap-1 border-b px-3 py-2.5 text-left ${
										current?.uuid === item.uuid ? "bg-muted" : ""
									}`}
								>
									<div className="flex w-full items-center gap-1.5">
										{!item.is_read && (
											<span className="bg-primary size-1.5 shrink-0 rounded-full" />
										)}
										<span className="truncate text-sm font-medium">{item.title}</span>
									</div>
									<div className="text-faint-foreground flex items-center gap-2 text-2xs">
										<span className={LEVEL_STYLES[item.level]}>
											{item.scope_type === "platform" ? "全站" : "域内"}
										</span>
										<span>{formatTime(item.published_at ?? item.created_at)}</span>
									</div>
								</button>
							))
						)}
					</div>

					<div className="min-w-0 flex-1 overflow-y-auto p-5">
						{current ? (
							<>
								<h2 className="mb-1 text-lg font-semibold">{current.title}</h2>
								<div className="text-faint-foreground mb-4 text-xs">
									{current.created_by_name} · {formatTime(current.published_at ?? current.created_at)}
								</div>
								<MarkdownText text={current.content} />
							</>
						) : (
							<div className="text-faint-foreground flex h-full items-center justify-center text-sm">
								选择一条通知查看详情
							</div>
						)}
					</div>
				</div>
			</DialogContent>
		</Dialog>
	);
}
```

- [ ] **步骤 3：弹窗测试**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/stores/notificationStore", () => {
	const state = { items: [], loading: false, unreadCount: 0 };
	const markRead = vi.fn();
	return {
		useNotificationStore: (selector: (s: typeof state) => unknown) =>
			selector(state),
		notificationStore: { loadList: vi.fn(), markRead },
	};
});

import NotificationModal from "./notificationModal";

describe("NotificationModal", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("shows the empty state when there are no notifications", () => {
		render(<NotificationModal open onClose={() => {}} />);
		expect(screen.getByText("暂无通知")).toBeTruthy();
	});

	it("closes when the close button is clicked", async () => {
		const onClose = vi.fn();
		render(<NotificationModal open onClose={onClose} />);
		await userEvent.click(screen.getByLabelText("关闭"));
		expect(onClose).toHaveBeenCalled();
	});
});
```

- [ ] **步骤 4：侧栏铃铛**

`app/web/src/layouts/common/sidebar.tsx`：

1. `SidebarProps` 追加 `onOpenNotifications?: () => void;`
2. 解构追加该 prop
3. import 追加 `Bell` 到 lucide-react 列表，追加 `import { useNotificationStore } from "@/stores/notificationStore";`
4. 组件体顶部读取未读数：

```ts
	const unreadCount = useNotificationStore((s) => s.unreadCount);
```

5. 在「分享链接」`OptionSquare` 之后插入：

```tsx
				<OptionSquare
					label="通知"
					badge={unreadCount}
					onClick={() => onOpenNotifications?.()}
				>
					<Bell {...iconProps} />
				</OptionSquare>
```

- [ ] **步骤 5：布局挂弹窗**

`app/web/src/layouts/layout.tsx`：

1. import 追加：

```ts
import NotificationModal from "@/components/modal/notificationModal";
```

2. 状态区（`const [settingsOpen, setSettingsOpen] = useState(false);` 附近）追加：

```ts
	const [notificationsOpen, setNotificationsOpen] = useState(false);
```

3. `<Sidebar onOpenSettings={onOpenSettings} />` 改为：

```tsx
						<Sidebar
							onOpenSettings={onOpenSettings}
							onOpenNotifications={() => setNotificationsOpen(true)}
						/>
```

4. `<SettingModal ... />` 之后追加：

```tsx
				<NotificationModal
					open={notificationsOpen}
					onClose={() => setNotificationsOpen(false)}
				/>
```

- [ ] **步骤 6：运行前端测试与类型检查**

```bash
pnpm --filter @gospeak/web typecheck && pnpm --filter @gospeak/web test
```
预期：全 PASS。

- [ ] **步骤 7：Commit**

```bash
git add app/web/src/components/modal/notificationModal.tsx app/web/src/components/modal/notificationModal.test.tsx app/web/src/components/common/optionSquare.tsx app/web/src/layouts/common/sidebar.tsx app/web/src/layouts/layout.tsx
git commit -m "feat(web): 侧栏通知铃铛与通知弹窗"
```

---

## 任务 10：管理后台公告页

**文件：**
- 创建：`app/web/src/pages/(app)/manage/announcements/index.tsx`
- 修改：`app/web/src/components/manage/manageNav.tsx`

- [ ] **步骤 1：注册管理页路由**

`app/web/src/components/manage/manageNav.tsx`：

1. `ManagePath` 联合类型追加 `| "announcements"`；
2. lucide import 追加 `Megaphone`；
3. `MANAGE_TABS` 在 `monitor` 之前插入：

```ts
	{
		path: "announcements",
		to: "/manage/announcements",
		label: "公告",
		icon: Megaphone,
		permissions: ["announcement:manage", "domain:manage"],
	},
```

- [ ] **步骤 2：页面实现**

```tsx
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Button, Input, Switch, toast } from "@gospeak/components";
import { Megaphone, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import {
	adminListAnnouncements,
	createAnnouncement,
	deleteAnnouncement,
	updateAnnouncement,
} from "@/api/announcement";
import { getMyDomainsDetailed } from "@/api/domain";
import {
	ManageHeader,
	ManageLoading,
	ManagePage,
	ManageSection,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";
import type { AnnouncementDTO } from "@/protocol/announcement";
import { denyAccess } from "@/utils/accessDenied";
import { hasPermission } from "@/utils/permissions";
import { toastActionError } from "@/utils/toast";

export const Route = createFileRoute("/(app)/manage/announcements/")({
	beforeLoad: () => {
		if (!hasPermission("announcement:manage") && !hasPermission("domain:manage")) {
			denyAccess();
		}
	},
	component: AnnouncementsPage,
	staticData: { title: "公告" },
});

interface FormState {
	uuid?: string;
	scopeType: "platform" | "domain";
	scopeId: string;
	title: string;
	content: string;
	level: "info" | "warning" | "critical";
	pinned: boolean;
	publish: boolean;
}

const EMPTY_FORM: FormState = {
	scopeType: "platform",
	scopeId: "",
	title: "",
	content: "",
	level: "info",
	pinned: false,
	publish: true,
};

export default function AnnouncementsPage() {
	const queryClient = useQueryClient();
	const isPlatformAdmin = hasPermission("announcement:manage");
	const [form, setForm] = useState<FormState>(EMPTY_FORM);
	const [saving, setSaving] = useState(false);

	const { data, isLoading } = useQuery({
		queryKey: ["manage-announcements"],
		queryFn: () => adminListAnnouncements(),
	});

	const { data: domains = [] } = useQuery({
		queryKey: ["manage-announcement-domains"],
		queryFn: getMyDomainsDetailed,
		enabled: !isPlatformAdmin,
	});

	const items = data?.items ?? [];

	const reset = () => setForm(EMPTY_FORM);

	const submit = async () => {
		if (!form.title.trim() || !form.content.trim()) {
			toast.error("标题与正文不能为空");
			return;
		}
		setSaving(true);
		try {
			const payload = {
				scope_type: form.scopeType,
				scope_id: form.scopeType === "domain" ? form.scopeId : undefined,
				title: form.title.trim(),
				content: form.content,
				level: form.level,
				pinned: form.pinned,
				publish: form.publish,
			};
			if (form.uuid) {
				await updateAnnouncement({ ...payload, uuid: form.uuid });
				toast.success("公告已更新");
			} else {
				await createAnnouncement(payload);
				toast.success(form.publish ? "公告已发布" : "草稿已保存");
			}
			reset();
			await queryClient.invalidateQueries({ queryKey: ["manage-announcements"] });
		} catch (error) {
			toastActionError(error, "保存失败");
		} finally {
			setSaving(false);
		}
	};

	const remove = async (item: AnnouncementDTO) => {
		try {
			await deleteAnnouncement(item.uuid);
			toast.success("公告已删除");
			await queryClient.invalidateQueries({ queryKey: ["manage-announcements"] });
		} catch (error) {
			toastActionError(error, "删除失败");
		}
	};

	const edit = (item: AnnouncementDTO) => {
		setForm({
			uuid: item.uuid,
			scopeType: item.scope_type,
			scopeId: item.scope_id ?? "",
			title: item.title,
			content: item.content,
			level: item.level,
			pinned: item.pinned,
			publish: !!item.published_at,
		});
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<Megaphone size={20} />}
				title="公告"
				description="发布全站或域内系统公告"
			/>

			<ManageSection title={form.uuid ? "编辑公告" : "新建公告"}>
				<div className="flex flex-col gap-3">
					{isPlatformAdmin && (
						<div className="flex items-center gap-2 text-sm">
							<span>范围</span>
							<select
								className="border-border bg-card rounded-sm border px-2 py-1 text-sm"
								value={form.scopeType}
								onChange={(e) =>
									setForm((prev) => ({
										...prev,
										scopeType: e.target.value as "platform" | "domain",
									}))
								}
							>
								<option value="platform">全站</option>
								<option value="domain">指定域</option>
							</select>
							{form.scopeType === "domain" && (
								<select
									className="border-border bg-card rounded-sm border px-2 py-1 text-sm"
									value={form.scopeId}
									onChange={(e) =>
										setForm((prev) => ({ ...prev, scopeId: e.target.value }))
									}
								>
									<option value="">选择域</option>
									{(domains as Array<{ uuid: string; name: string }>).map((d) => (
										<option key={d.uuid} value={d.uuid}>
											{d.name}
										</option>
									))}
								</select>
							)}
						</div>
					)}

					<Input
						placeholder="标题"
						value={form.title}
						onChange={(e) => setForm((prev) => ({ ...prev, title: e.target.value }))}
					/>

					<textarea
						className="border-border bg-card min-h-40 rounded-sm border p-3 font-mono text-sm"
						placeholder="正文（支持 Markdown）"
						value={form.content}
						onChange={(e) => setForm((prev) => ({ ...prev, content: e.target.value }))}
					/>

					<div className="flex flex-wrap items-center gap-4 text-sm">
						<label className="flex items-center gap-2">
							级别
							<select
								className="border-border bg-card rounded-sm border px-2 py-1"
								value={form.level}
								onChange={(e) =>
									setForm((prev) => ({
										...prev,
										level: e.target.value as FormState["level"],
									}))
								}
							>
								<option value="info">普通</option>
								<option value="warning">提醒</option>
								<option value="critical">重要</option>
							</select>
						</label>
						<label className="flex items-center gap-2">
							<Switch
								checked={form.pinned}
								onCheckedChange={(v) => setForm((prev) => ({ ...prev, pinned: v }))}
							/>
							置顶
						</label>
						<label className="flex items-center gap-2">
							<Switch
								checked={form.publish}
								onCheckedChange={(v) => setForm((prev) => ({ ...prev, publish: v }))}
							/>
							立即发布
						</label>
					</div>

					<div className="flex items-center gap-2">
						<Button onClick={submit} disabled={saving}>
							{form.uuid ? "保存" : "创建"}
						</Button>
						{form.uuid && (
							<Button variant="ghost" onClick={reset}>
								取消编辑
							</Button>
						)}
					</div>
				</div>
			</ManageSection>

			<ManageSection title="公告列表">
				{isLoading ? (
					<ManageLoading />
				) : items.length === 0 ? (
					<div className="text-faint-foreground py-8 text-center text-sm">
						暂无公告
					</div>
				) : (
					<table className="w-full text-sm">
						<thead className={manageTableHeadClass}>
							<tr>
								<th className="px-2 py-2 text-left">标题</th>
								<th className="px-2 py-2 text-left">范围</th>
								<th className="px-2 py-2 text-left">级别</th>
								<th className="px-2 py-2 text-left">状态</th>
								<th className="px-2 py-2 text-right">操作</th>
							</tr>
						</thead>
						<tbody>
							{items.map((item) => (
								<tr key={item.uuid} className={manageTableRowClass}>
									<td className="px-2 py-2">{item.title}</td>
									<td className="px-2 py-2">
										{item.scope_type === "platform" ? "全站" : "域内"}
									</td>
									<td className="px-2 py-2">{item.level}</td>
									<td className="px-2 py-2">
										{item.published_at ? "已发布" : "草稿"}
									</td>
									<td className="px-2 py-2 text-right">
										<Button variant="ghost" onClick={() => edit(item)}>
											编辑
										</Button>
										<Button variant="ghost" onClick={() => void remove(item)}>
											<Trash2 size={14} />
										</Button>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
			</ManageSection>
		</ManagePage>
	);
}
```

> 若 `ManageSection` 的 props 与上方不一致，读 `app/web/src/components/manage/ManageShell.tsx:80` 后按其实际签名调整。

- [ ] **步骤 3：生成路由树并类型检查**

```bash
pnpm --filter @gospeak/web typecheck
```
预期：`tsr generate` 生成 `routeTree.gen.ts` 后 tsc 无错误。

- [ ] **步骤 4：全量验证**

```bash
pnpm lint && pnpm typecheck && pnpm test
```
预期：全部通过。

- [ ] **步骤 5：Commit**

```bash
git add "app/web/src/pages/(app)/manage/announcements/index.tsx" app/web/src/components/manage/manageNav.tsx app/web/src/routeTree.gen.ts
git commit -m "feat(web): 管理后台公告发布页"
```

---

## 验收标准

- [ ] `cd app/server && go build ./... && go test ./internal/...` 全通过
- [ ] `pnpm test:server -- test/announcement/` 4 个用例通过
- [ ] `pnpm --filter @gospeak/web typecheck && pnpm --filter @gospeak/web test` 通过
- [ ] `pnpm lint` 通过
- [ ] 手动验证：admin 发布平台公告 → 另一个浏览器登录普通用户 → 侧栏铃铛出现 badge → 点击弹窗 → 点单条 → badge 减 1 → 刷新页面 badge 与已读状态保持
- [ ] 手动验证：域管理员发布域公告 → 域内成员看到、域外成员看不到
