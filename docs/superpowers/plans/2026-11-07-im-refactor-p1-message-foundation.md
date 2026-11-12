# 文字 IM 重构 P1（消息地基）实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 用统一会话抽象（conversation 一等实体 + 参与者集合 + 统一 read_states）重构消息地基：1:1 与群聊 DM、消息操作覆盖 DM、频道/DM/提及统一未读，旧 API 响应格式兼容（现有前端零改动可继续工作）。

**架构：** 方案 A（见规格 `docs/superpowers/specs/2026-11-07-im-refactor-p1-message-foundation-design.md`）。新建 `conversations` / `conversation_members` / `read_states` 三表与配套仓储；`ConversationService` 内部换引擎、对外四个旧端点保持原响应格式；新增 conversation 端点族与 `read-state` 端点族；WS 新增 `conversation:*` 事件族（复用 `__user:{identity}` 个人房间定向投递）。旧 `conversation_participants` 模型/代码删除，旧表留在库中不动（不迁移已拍板）。

**技术栈：** Go + Gin + GORM（SQLite/glebarez 内存库测试）+ 内嵌 NATS job 队列（chat.persist / chat.mutate / chat.private.persist）。

---

## ⚠️ 工作区安全约定（每个任务都必须遵守）

当前工作区含**与本计划无关的未提交改动**（bot/plugin/agora 砍除 WIP，已暂存 + 未暂存混合）。因此：

- **禁止** `git add -A` / `git add .` / `git commit -a`
- 每次 commit 一律：`git add <本任务明确列出的文件>` 然后 `git commit -m "<msg>" -- <同样的文件列表>`
- commit 前用 `git status --short` 确认没有把无关文件带进暂存区

## 规格澄清（实现前确认过的三个落地细节）

1. **列表端点拆分**：`/conversation/list` 全程保持旧响应格式（兼容旧前端）；新格式会话列表走新端点 `POST /conversation/summaries`（同一路径无法承载两种契约）。
2. **DM 消息操作的 API 面**：规格 §3 要求 edit/delete/react 覆盖 conversation scope，对应实现为 WS `conversation:edit|delete|react|unreact`（C→S）+ REST `POST /conversation/messages/{edit,delete,react,unreact}`。
3. **MESSAGE_ACK**：仅删除后端死常量 `signal.EventMessageAck`；前端 `socket/events.ts` 的同名常量留给 P2 清理（P1 前端零改动）。
4. **mark-read 游标语义**：`last_read_message_uuid` 按「传入值优先、空值保留现有」落库（UUID 字符串之间无大小可比性，按消息位置比较需额外查询，无必要）。

## 文件结构

**新建：**

| 文件 | 职责 |
|------|------|
| `app/server/internal/model/conversation.go` | `Conversation` / `ConversationMember` 模型 + `MaxConversationMembers` |
| `app/server/internal/model/read_state.go` | `ReadState` 模型 + scope 常量 |
| `app/server/internal/repository/conversation_store.go` | 新模型仓储（会话/成员读写、direct 查重、last_message 维护） |
| `app/server/internal/repository/read_state_repo.go` | 未读仓储（ApplyRoomMessage / ApplyConversationMessage / MarkRead / ListByUser / EnsureScopeRow） |
| `app/server/internal/service/read_state_service.go` | Mark / Overview 业务逻辑（含 conversation 成员校验） |
| `app/server/internal/handler/read_state_handler.go` | `/read-state/*` HTTP 处理 |
| `app/server/internal/router/routes/readstate/routes.go` | read-state 路由组 |
| `app/server/internal/signal/conversation_bridge.go` | WS `conversation:send|edit|delete|react|unreact` 处理 |
| `app/server/internal/service/message_conversation.go` | MessageService 的 conversation-scope 四操作（Edit/Delete/React/Unreact） |

**修改：**

| 文件 | 变更 |
|------|------|
| `app/server/internal/model/message.go` | `ConversationID` size 32→36（语义变为会话 UUID） |
| `app/server/internal/repository/db_migrations.go` | AutoMigrate 增 3 模型；删除 `migrateConversationQueryIndexes` |
| `app/server/internal/repository/user_repo.go` | 增 `GetByUUIDs` |
| `app/server/internal/repository/domain_repo.go` | 增 `HasCommonDomain` |
| `app/server/internal/service/conversation_service.go` | 整体重写（新模型引擎 + 兼容投影 + 新方法族） |
| `app/server/internal/service/message_service.go` / `message_jobs.go` | 落库路径接 read-state 累加；增 store/readStates 依赖 |
| `app/server/internal/handler/conversation_handler.go` | 保留 4 兼容端点 + 新增 create/get/invite/leave/summaries/messages-* |
| `app/server/internal/router/routes/conversation/routes.go` | 路由扩展 |
| `app/server/internal/router/router.go` | 注册 readstate 组 |
| `app/server/internal/signal/events.go` | 增 `conversation:*` 常量；删 `EventMessageAck` |
| `app/server/internal/signal/hub.go` | `clientUserUUID` helper；事件注册 |
| `app/server/internal/signal/private_bridge.go` | `conversationSender` 接口扩展 |
| `app/server/server/gin.go` | 装配新依赖 |
| `test/conversation/conversation.test.ts` | 新端点全链路 + 群聊流集成测试 |

**删除（任务 8 内）：** `app/server/internal/model/conversation_participant.go`、`app/server/internal/repository/conversation_repo.go`

---

### 任务 1：新数据模型 + 迁移注册

**文件：**
- 创建：`app/server/internal/model/conversation.go`、`app/server/internal/model/read_state.go`
- 修改：`app/server/internal/model/message.go:27`、`app/server/internal/repository/db_migrations.go:237-280`
- 测试：`app/server/internal/repository/conversation_store_test.go`（新建，本任务先放模型级用例）

- [ ] **步骤 1：编写失败的测试**

```go
package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newConvTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Conversation{}, &model.ConversationMember{}, &model.ReadState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestConversationMemberUniqueConstraint(t *testing.T) {
	db := newConvTestDB(t)
	conv := &model.Conversation{Type: model.ConversationTypeDirect}
	if err := db.Create(conv).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	first := &model.ConversationMember{ConversationUUID: conv.UUID, UserUUID: "u-1"}
	if err := db.Create(first).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	dup := &model.ConversationMember{ConversationUUID: conv.UUID, UserUUID: "u-1"}
	if err := db.Create(dup).Error; err == nil {
		t.Fatal("expected duplicate member insert to fail")
	}
}

func TestReadStateUniqueConstraint(t *testing.T) {
	db := newConvTestDB(t)
	first := &model.ReadState{UserUUID: "u-1", ScopeType: model.ReadScopeRoom, ScopeID: "room-1"}
	if err := db.Create(first).Error; err != nil {
		t.Fatalf("create read state: %v", err)
	}
	dup := &model.ReadState{UserUUID: "u-1", ScopeType: model.ReadScopeRoom, ScopeID: "room-1"}
	if err := db.Create(dup).Error; err == nil {
		t.Fatal("expected duplicate read state insert to fail")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/repository/ -run 'TestConversationMemberUnique|TestReadStateUnique' -v`
预期：编译失败（`model.Conversation` 等未定义）

- [ ] **步骤 3：编写模型**

`app/server/internal/model/conversation.go`：

```go
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ConversationTypeDirect = "direct"
	ConversationTypeGroup  = "group"
)

// MaxConversationMembers 群聊人数上限（发起者 + 被邀请者合计）。
const MaxConversationMembers = 50

type Conversation struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	UUID            string     `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Type            string     `gorm:"size:10;not null;default:'direct'" json:"type"`
	LastMessageUUID string     `gorm:"size:36;index" json:"last_message_uuid,omitempty"`
	LastContent     string     `gorm:"size:200" json:"last_content"`
	LastSenderUUID  string     `gorm:"size:36" json:"last_sender_uuid"`
	LastMessageAt   *time.Time `json:"last_message_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (c *Conversation) BeforeCreate(_ *gorm.DB) error {
	if c.UUID == "" {
		c.UUID = uuid.New().String()
	}
	return nil
}

func (Conversation) TableName() string { return "conversations" }

type ConversationMember struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	ConversationUUID string    `gorm:"size:36;uniqueIndex:uniq_conv_member,priority:1" json:"conversation_uuid"`
	UserUUID         string    `gorm:"size:36;uniqueIndex:uniq_conv_member,priority:2" json:"user_uuid"`
	InvitedBy        string    `gorm:"size:36" json:"invited_by,omitempty"`
	JoinedAt         time.Time `json:"joined_at"`
}

func (ConversationMember) TableName() string { return "conversation_members" }
```

`app/server/internal/model/read_state.go`：

```go
package model

import "time"

const (
	ReadScopeRoom         = "room"
	ReadScopeConversation = "conversation"
)

type ReadState struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	UserUUID            string    `gorm:"size:36;uniqueIndex:uniq_read_state,priority:1" json:"user_uuid"`
	ScopeType           string    `gorm:"size:12;uniqueIndex:uniq_read_state,priority:2" json:"scope_type"`
	ScopeID             string    `gorm:"size:36;uniqueIndex:uniq_read_state,priority:3" json:"scope_id"`
	LastReadMessageUUID string    `gorm:"size:36" json:"last_read_message_uuid"`
	UnreadCount         int       `gorm:"default:0" json:"unread_count"`
	MentionCount        int       `gorm:"default:0" json:"mention_count"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (ReadState) TableName() string { return "read_states" }
```

`app/server/internal/model/message.go` 第 27 行改为：

```go
	ConversationID   *string        `gorm:"size:36;index:idx_msg_conversation,priority:2" json:"conversation_id,omitempty"`
```

`app/server/internal/repository/db_migrations.go`：AutoMigrate 列表（第 238-264 行）在 `&model.DomainGuestBan{},` 前追加：

```go
		&model.Conversation{},
		&model.ConversationMember{},
		&model.ReadState{},
```

并整体删除 `migrateConversationQueryIndexes` 函数（第 268-280 行）及 `autoMigrate` 末尾对它的调用（第 265 行改为 `return nil` 的等价收敛：`}); err != nil { return err }\n\treturn nil`）。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/repository/ -run 'TestConversationMemberUnique|TestReadStateUnique' -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/model/conversation.go app/server/internal/model/read_state.go app/server/internal/model/message.go app/server/internal/repository/db_migrations.go app/server/internal/repository/conversation_store_test.go
git commit -m "feat(im): add conversation/conversation-member/read-state models" -- app/server/internal/model/conversation.go app/server/internal/model/read_state.go app/server/internal/model/message.go app/server/internal/repository/db_migrations.go app/server/internal/repository/conversation_store_test.go
```

---

### 任务 2：支撑查询 GetByUUIDs + HasCommonDomain

**文件：**
- 修改：`app/server/internal/repository/user_repo.go`、`app/server/internal/repository/domain_repo.go`
- 测试：`app/server/internal/repository/user_repo_test.go`、`app/server/internal/repository/domain_repo_common_test.go`（若已存在同名测试文件则追加用例）

- [ ] **步骤 1：编写失败的测试**

```go
func TestUserGetByUUIDs(t *testing.T) {
	db := newConvTestDB(t)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := []model.User{
		{Name: "alice", UUID: "uuid-a"},
		{Name: "bob", UUID: "uuid-b"},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	repo := NewUserRepository(db)
	got, err := repo.GetByUUIDs([]string{"uuid-a", "uuid-b", "uuid-missing"})
	if err != nil {
		t.Fatalf("GetByUUIDs: %v", err)
	}
	if len(got) != 2 || got["uuid-a"].Name != "alice" || got["uuid-b"].Name != "bob" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestDomainHasCommonDomain(t *testing.T) {
	db := newConvTestDB(t)
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewDomainRepository(db)
	dom := &model.Domain{UUID: "dom-1", Name: "d"}
	if err := db.Create(dom).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	for _, m := range []model.DomainMember{
		{DomainUUID: "dom-1", UserUUID: "u-1", RoleName: "member"},
		{DomainUUID: "dom-1", UserUUID: "u-2", RoleName: "member"},
		{DomainUUID: "dom-1", UserUUID: "u-3", RoleName: "member"},
	} {
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("seed member: %v", err)
		}
	}
	yes, err := repo.HasCommonDomain("u-1", "u-2")
	if err != nil || !yes {
		t.Fatalf("expected common domain, got %v err=%v", yes, err)
	}
	no, err := repo.HasCommonDomain("u-1", "u-4")
	if err != nil || no {
		t.Fatalf("expected no common domain, got %v err=%v", no, err)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/repository/ -run 'TestUserGetByUUIDs|TestDomainHasCommonDomain' -v`
预期：编译失败（方法不存在）

- [ ] **步骤 3：实现**

`user_repo.go` 追加（放在 `GetByNames` 之后）：

```go
// GetByUUIDs 批量按 UUID 取用户，返回 map[uuid]*User；缺失的 UUID 不出现在结果中。
func (r *UserRepository) GetByUUIDs(uuids []string) (map[string]*model.User, error) {
	out := make(map[string]*model.User, len(uuids))
	if len(uuids) == 0 {
		return out, nil
	}
	var rows []model.User
	if err := r.db.Where("uuid IN ?", uuids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		out[rows[i].UUID] = &rows[i]
	}
	return out, nil
}
```

`domain_repo.go` 追加：

```go
// HasCommonDomain 判断两个用户是否同为任一 Domain 的成员（DM 拉人资格校验）。
func (r *DomainRepository) HasCommonDomain(userA, userB string) (bool, error) {
	if userA == "" || userB == "" {
		return false, nil
	}
	var count int64
	err := r.db.Model(&model.DomainMember{}).
		Where("user_uuid = ? AND domain_uuid IN (?)", userB,
			r.db.Model(&model.DomainMember{}).Select("domain_uuid").Where("user_uuid = ?", userA)).
		Count(&count).Error
	return count > 0, err
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/repository/ -run 'TestUserGetByUUIDs|TestDomainHasCommonDomain' -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/repository/user_repo.go app/server/internal/repository/domain_repo.go app/server/internal/repository/user_repo_test.go app/server/internal/repository/domain_repo_common_test.go
git commit -m "feat(im): add user GetByUUIDs and domain HasCommonDomain" -- app/server/internal/repository/user_repo.go app/server/internal/repository/domain_repo.go app/server/internal/repository/user_repo_test.go app/server/internal/repository/domain_repo_common_test.go
```

---

### 任务 3：ReadStateRepository

**文件：**
- 创建：`app/server/internal/repository/read_state_repo.go`
- 测试：`app/server/internal/repository/read_state_repo_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

func seedReadRows(t *testing.T, db *gorm.DB, rows ...model.ReadState) {
	t.Helper()
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed read state: %v", err)
		}
	}
}

func TestApplyRoomMessageIncrementAndMention(t *testing.T) {
	db := newConvTestDB(t)
	repo := NewReadStateRepository(db)
	seedReadRows(t, db,
		model.ReadState{UserUUID: "u-1", ScopeType: model.ReadScopeRoom, ScopeID: "r-1"},
		model.ReadState{UserUUID: "u-2", ScopeType: model.ReadScopeRoom, ScopeID: "r-1"},
		model.ReadState{UserUUID: "u-1", ScopeType: model.ReadScopeRoom, ScopeID: "r-other"},
	)

	// u-1 在场且被提及：既有行 +1 未读 +1 提及；u-2 未读 +1；无行者不建行
	if err := repo.ApplyRoomMessage("r-1", "u-sender", []string{"u-1"}); err != nil {
		t.Fatalf("ApplyRoomMessage: %v", err)
	}
	var u1, u2 model.ReadState
	if err := db.First(&u1, "user_uuid = ? AND scope_type = ? AND scope_id = ?", "u-1", model.ReadScopeRoom, "r-1").Error; err != nil {
		t.Fatalf("load u-1: %v", err)
	}
	if u1.UnreadCount != 1 || u1.MentionCount != 1 {
		t.Fatalf("u-1 got unread=%d mention=%d, want 1/1", u1.UnreadCount, u1.MentionCount)
	}
	if err := db.First(&u2, "user_uuid = ? AND scope_type = ? AND scope_id = ?", "u-2", model.ReadScopeRoom, "r-1").Error; err != nil {
		t.Fatalf("load u-2: %v", err)
	}
	if u2.UnreadCount != 1 || u2.MentionCount != 0 {
		t.Fatalf("u-2 got unread=%d mention=%d, want 1/0", u2.UnreadCount, u2.MentionCount)
	}
	var other model.ReadState
	if err := db.First(&other, "user_uuid = ? AND scope_id = ?", "u-1", "r-other").Error; err != nil {
		t.Fatalf("load other: %v", err)
	}
	if other.UnreadCount != 0 {
		t.Fatalf("other scope must be untouched, got %d", other.UnreadCount)
	}
}

func TestApplyRoomMessageMentionCreatesRow(t *testing.T) {
	db := newConvTestDB(t)
	repo := NewReadStateRepository(db)
	if err := repo.ApplyRoomMessage("r-1", "u-sender", []string{"u-new"}); err != nil {
		t.Fatalf("ApplyRoomMessage: %v", err)
	}
	var row model.ReadState
	if err := db.First(&row, "user_uuid = ? AND scope_type = ? AND scope_id = ?", "u-new", model.ReadScopeRoom, "r-1").Error; err != nil {
		t.Fatalf("mention must force-create row: %v", err)
	}
	if row.UnreadCount != 1 || row.MentionCount != 1 {
		t.Fatalf("created row unread=%d mention=%d, want 1/1", row.UnreadCount, row.MentionCount)
	}
}

func TestApplyConversationMessage(t *testing.T) {
	db := newConvTestDB(t)
	repo := NewReadStateRepository(db)
	seedReadRows(t, db,
		model.ReadState{UserUUID: "u-1", ScopeType: model.ReadScopeConversation, ScopeID: "c-1"},
		model.ReadState{UserUUID: "u-2", ScopeType: model.ReadScopeConversation, ScopeID: "c-1"},
	)
	if err := repo.ApplyConversationMessage("c-1", "u-1", []string{"u-2"}); err != nil {
		t.Fatalf("ApplyConversationMessage: %v", err)
	}
	var u1, u2 model.ReadState
	_ = db.First(&u1, "user_uuid = ? AND scope_id = ?", "u-1", "c-1").Error
	_ = db.First(&u2, "user_uuid = ? AND scope_id = ?", "u-2", "c-1").Error
	if u1.UnreadCount != 0 || u1.MentionCount != 0 {
		t.Fatalf("sender unread/mention must stay 0, got %d/%d", u1.UnreadCount, u1.MentionCount)
	}
	if u2.UnreadCount != 1 || u2.MentionCount != 1 {
		t.Fatalf("receiver unread=%d mention=%d, want 1/1", u2.UnreadCount, u2.MentionCount)
	}
}

func TestMarkReadResetsAndKeepsCursor(t *testing.T) {
	db := newConvTestDB(t)
	repo := NewReadStateRepository(db)
	seedReadRows(t, db,
		model.ReadState{UserUUID: "u-1", ScopeType: model.ReadScopeConversation, ScopeID: "c-1",
			LastReadMessageUUID: "m-old", UnreadCount: 5, MentionCount: 2},
	)
	if err := repo.MarkRead(model.ReadScopeConversation, "c-1", "u-1", "m-new"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	var row model.ReadState
	if err := db.First(&row, "user_uuid = ? AND scope_id = ?", "u-1", "c-1").Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if row.UnreadCount != 0 || row.MentionCount != 0 || row.LastReadMessageUUID != "m-new" {
		t.Fatalf("after mark-read: %+v", row)
	}
	if err := repo.MarkRead(model.ReadScopeConversation, "c-1", "u-1", ""); err != nil {
		t.Fatalf("MarkRead empty cursor: %v", err)
	}
	if err := db.First(&row, "user_uuid = ? AND scope_id = ?", "u-1", "c-1").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.LastReadMessageUUID != "m-new" {
		t.Fatalf("empty cursor must keep existing value, got %q", row.LastReadMessageUUID)
	}
}

func TestEnsureScopeRowAndListByUser(t *testing.T) {
	db := newConvTestDB(t)
	repo := NewReadStateRepository(db)
	if err := repo.EnsureScopeRow(model.ReadScopeConversation, "c-1", "u-1"); err != nil {
		t.Fatalf("EnsureScopeRow: %v", err)
	}
	if err := repo.EnsureScopeRow(model.ReadScopeConversation, "c-1", "u-1"); err != nil {
		t.Fatalf("EnsureScopeRow idempotent: %v", err)
	}
	var count int64
	db.Model(&model.ReadState{}).Count(&count)
	if count != 1 {
		t.Fatalf("ensure must be idempotent, rows=%d", count)
	}
	_ = repo.EnsureScopeRow(model.ReadScopeConversation, "c-2", "u-1")
	rows, err := repo.ListByUser("u-1")
	if err != nil || len(rows) != 2 {
		t.Fatalf("ListByUser: %v rows=%d", err, len(rows))
	}
}
```

（测试文件顶部 import 需含 `"gorm.io/gorm"`。）

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/repository/ -run 'TestApplyRoom|TestApplyConversation|TestMarkRead|TestEnsureScope' -v`
预期：编译失败（`NewReadStateRepository` 不存在）

- [ ] **步骤 3：实现 `read_state_repo.go`**

```go
package repository

import (
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReadStateRepository struct {
	db *gorm.DB
}

func NewReadStateRepository(db *gorm.DB) *ReadStateRepository {
	return &ReadStateRepository{db: db}
}

// EnsureScopeRow 幂等创建（scope, user）的 read_state 行（unread=0）。
func (r *ReadStateRepository) EnsureScopeRow(scopeType, scopeID, userUUID string) error {
	row := model.ReadState{UserUUID: userUUID, ScopeType: scopeType, ScopeID: scopeID, UpdatedAt: time.Now().UTC()}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_uuid"}, {Name: "scope_type"}, {Name: "scope_id"}},
		DoNothing: true,
	}).Create(&row).Error
}

// MarkRead 归零计数并推进游标；lastReadMessageUUID 为空时保留现有游标。
func (r *ReadStateRepository) MarkRead(scopeType, scopeID, userUUID, lastReadMessageUUID string) error {
	now := time.Now().UTC()
	row := model.ReadState{UserUUID: userUUID, ScopeType: scopeType, ScopeID: scopeID, LastReadMessageUUID: lastReadMessageUUID, UpdatedAt: now}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_uuid"}, {Name: "scope_type"}, {Name: "scope_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"unread_count":  0,
			"mention_count": 0,
			"last_read_message_uuid": gorm.Expr(
				"CASE WHEN ? = '' THEN last_read_message_uuid ELSE ? END", lastReadMessageUUID, lastReadMessageUUID),
			"updated_at": now,
		}),
	}).Create(&row).Error
}

// ApplyRoomMessage 房间消息落库后的未读累加：既有行 +1（排除发送者与被提及者），
// 被提及者强制建行（unread+1、mention+1）。
func (r *ReadStateRepository) ApplyRoomMessage(roomUUID, senderUUID string, mentionUUIDs []string) error {
	if senderUUID == "" {
		return nil
	}
	q := r.db.Model(&model.ReadState{}).
		Where("scope_type = ? AND scope_id = ? AND user_uuid <> ?", model.ReadScopeRoom, roomUUID, senderUUID)
	if len(mentionUUIDs) > 0 {
		q = q.Where("user_uuid NOT IN ?", mentionUUIDs)
	}
	if err := q.Updates(map[string]interface{}{
		"unread_count": gorm.Expr("unread_count + 1"),
		"updated_at":   time.Now().UTC(),
	}).Error; err != nil {
		return err
	}
	return r.bumpMentionRows(model.ReadScopeRoom, roomUUID, senderUUID, mentionUUIDs)
}

// ApplyConversationMessage 会话消息落库后的未读累加：成员行 +1（排除发送者），被提及者 mention+1（行必然已存在）。
func (r *ReadStateRepository) ApplyConversationMessage(convUUID, senderUUID string, mentionUUIDs []string) error {
	if senderUUID == "" {
		return nil
	}
	if err := r.db.Model(&model.ReadState{}).
		Where("scope_type = ? AND scope_id = ? AND user_uuid <> ?", model.ReadScopeConversation, convUUID, senderUUID).
		Updates(map[string]interface{}{
			"unread_count": gorm.Expr("unread_count + 1"),
			"updated_at":   time.Now().UTC(),
		}).Error; err != nil {
		return err
	}
	if len(mentionUUIDs) == 0 {
		return nil
	}
	return r.db.Model(&model.ReadState{}).
		Where("scope_type = ? AND scope_id = ? AND user_uuid <> ? AND user_uuid IN ?",
			model.ReadScopeConversation, convUUID, senderUUID, mentionUUIDs).
		Update("mention_count", gorm.Expr("mention_count + 1")).Error
}

// bumpMentionRows 被提及者强制建行：新行 unread=1/mention=1，已有行各 +1。
func (r *ReadStateRepository) bumpMentionRows(scopeType, scopeID, senderUUID string, mentionUUIDs []string) error {
	if len(mentionUUIDs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]model.ReadState, 0, len(mentionUUIDs))
	for _, u := range mentionUUIDs {
		if u == "" || u == senderUUID {
			continue
		}
		rows = append(rows, model.ReadState{UserUUID: u, ScopeType: scopeType, ScopeID: scopeID, UnreadCount: 1, MentionCount: 1, UpdatedAt: now})
	}
	if len(rows) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_uuid"}, {Name: "scope_type"}, {Name: "scope_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"unread_count":   gorm.Expr("unread_count + 1"),
			"mention_count":  gorm.Expr("mention_count + 1"),
			"updated_at":     now,
		}),
	}).Create(&rows).Error
}

// Get 返回单行；gorm.ErrRecordNotFound 表示无行（未读视为 0）。
func (r *ReadStateRepository) Get(scopeType, scopeID, userUUID string) (*model.ReadState, error) {
	var row model.ReadState
	err := r.db.First(&row, "user_uuid = ? AND scope_type = ? AND scope_id = ?", userUUID, scopeType, scopeID).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListByUser 返回该用户全部 read_state 行（有行才有未读语义）。
func (r *ReadStateRepository) ListByUser(userUUID string) ([]model.ReadState, error) {
	var rows []model.ReadState
	err := r.db.Where("user_uuid = ?", userUUID).Find(&rows).Error
	return rows, err
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/repository/ -run 'TestApplyRoom|TestApplyConversation|TestMarkRead|TestEnsureScope' -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/repository/read_state_repo.go app/server/internal/repository/read_state_repo_test.go
git commit -m "feat(im): add read-state repository with unified unread pipeline" -- app/server/internal/repository/read_state_repo.go app/server/internal/repository/read_state_repo_test.go
```

---

### 任务 4：ConversationStore（新模型仓储）

**文件：**
- 创建：`app/server/internal/repository/conversation_store.go`
- 测试：追加到 `app/server/internal/repository/conversation_store_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestConversationStoreCreateAndGet(t *testing.T) {
	db := newConvTestDB(t)
	store := NewConversationStore(db)
	conv := &model.Conversation{Type: model.ConversationTypeGroup}
	members := []model.ConversationMember{
		{ConversationUUID: conv.UUID, UserUUID: "u-1"},
		{ConversationUUID: conv.UUID, UserUUID: "u-2"},
		{ConversationUUID: conv.UUID, UserUUID: "u-3"},
	}
	if err := store.CreateConversation(conv, members); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	got, err := store.GetByUUID(conv.UUID)
	if err != nil || got.Type != model.ConversationTypeGroup {
		t.Fatalf("GetByUUID: %v %+v", err, got)
	}
	uuids, err := store.GetMemberUUIDs(conv.UUID)
	if err != nil || len(uuids) != 3 {
		t.Fatalf("GetMemberUUIDs: %v %v", err, uuids)
	}
	if ok, _ := store.IsMember(conv.UUID, "u-2"); !ok {
		t.Fatal("u-2 must be member")
	}
	if ok, _ := store.IsMember(conv.UUID, "u-9"); ok {
		t.Fatal("u-9 must not be member")
	}
}

func TestConversationStoreFindDirectByMembers(t *testing.T) {
	db := newConvTestDB(t)
	store := NewConversationStore(db)
	a, b := &model.Conversation{Type: model.ConversationTypeDirect}, &model.Conversation{Type: model.ConversationTypeDirect}
	_ = store.CreateConversation(a, []model.ConversationMember{{ConversationUUID: a.UUID, UserUUID: "u-1"}, {ConversationUUID: a.UUID, UserUUID: "u-2"}})
	_ = store.CreateConversation(b, []model.ConversationMember{{ConversationUUID: b.UUID, UserUUID: "u-1"}, {ConversationUUID: b.UUID, UserUUID: "u-3"}})

	got, err := store.FindDirectByMembers("u-2", "u-1")
	if err != nil || got.UUID != a.UUID {
		t.Fatalf("FindDirectByMembers: %v %+v", err, got)
	}
	if missing, err := store.FindDirectByMembers("u-2", "u-3"); err == nil {
		t.Fatalf("expected not-found for unshared pair, got %+v err=%v", missing, err)
	}
}

func TestConversationStoreLastMessageAndLeave(t *testing.T) {
	db := newConvTestDB(t)
	store := NewConversationStore(db)
	conv := &model.Conversation{Type: model.ConversationTypeGroup}
	_ = store.CreateConversation(conv, []model.ConversationMember{
		{ConversationUUID: conv.UUID, UserUUID: "u-1"},
		{ConversationUUID: conv.UUID, UserUUID: "u-2"},
	})
	now := time.Now().UTC()
	if err := store.UpdateLastMessage(conv.UUID, "m-1", "hello", "u-1", now); err != nil {
		t.Fatalf("UpdateLastMessage: %v", err)
	}
	got, _ := store.GetByUUID(conv.UUID)
	if got.LastContent != "hello" || got.LastMessageAt == nil {
		t.Fatalf("last message not updated: %+v", got)
	}
	if err := store.RemoveMember(conv.UUID, "u-1"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if ok, _ := store.IsMember(conv.UUID, "u-1"); ok {
		t.Fatal("u-1 should have left")
	}
	if err := store.DeleteByUUID(conv.UUID); err != nil {
		t.Fatalf("DeleteByUUID: %v", err)
	}
	if _, err := store.GetByUUID(conv.UUID); err == nil {
		t.Fatal("conversation should be gone")
	}
}

func TestConversationStoreListByUser(t *testing.T) {
	db := newConvTestDB(t)
	store := NewConversationStore(db)
	a := &model.Conversation{Type: model.ConversationTypeDirect}
	b := &model.Conversation{Type: model.ConversationTypeGroup}
	_ = store.CreateConversation(a, []model.ConversationMember{{ConversationUUID: a.UUID, UserUUID: "u-1"}, {ConversationUUID: a.UUID, UserUUID: "u-2"}})
	_ = store.CreateConversation(b, []model.ConversationMember{{ConversationUUID: b.UUID, UserUUID: "u-1"}, {ConversationUUID: b.UUID, UserUUID: "u-2"}})
	rows, err := store.ListByUser("u-1", 10)
	if err != nil || len(rows) != 2 {
		t.Fatalf("ListByUser: %v rows=%d", err, len(rows))
	}
}
```

（import 增 `"time"`。）

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/repository/ -run TestConversationStore -v`
预期：编译失败（`NewConversationStore` 不存在）

- [ ] **步骤 3：实现 `conversation_store.go`**

```go
package repository

import (
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type ConversationStore struct {
	db *gorm.DB
}

func NewConversationStore(db *gorm.DB) *ConversationStore {
	return &ConversationStore{db: db}
}

// CreateConversation 在同一事务内创建会话与成员行。
func (r *ConversationStore) CreateConversation(conv *model.Conversation, members []model.ConversationMember) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(conv).Error; err != nil {
			return err
		}
		for i := range members {
			members[i].ConversationUUID = conv.UUID
		}
		return tx.Create(&members).Error
	})
}

func (r *ConversationStore) GetByUUID(uuid string) (*model.Conversation, error) {
	var conv model.Conversation
	if err := r.db.First(&conv, "uuid = ?", uuid).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *ConversationStore) GetMemberUUIDs(convUUID string) ([]string, error) {
	var uuids []string
	err := r.db.Model(&model.ConversationMember{}).
		Where("conversation_uuid = ?", convUUID).Order("user_uuid").Pluck("user_uuid", &uuids).Error
	return uuids, err
}

func (r *ConversationStore) IsMember(convUUID, userUUID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.ConversationMember{}).
		Where("conversation_uuid = ? AND user_uuid = ?", convUUID, userUUID).Count(&count).Error
	return count > 0, err
}

func (r *ConversationStore) AddMember(member *model.ConversationMember) error {
	return r.db.Create(member).Error
}

func (r *ConversationStore) RemoveMember(convUUID, userUUID string) error {
	return r.db.Where("conversation_uuid = ? AND user_uuid = ?", convUUID, userUUID).
		Delete(&model.ConversationMember{}).Error
}

func (r *ConversationStore) CountMembers(convUUID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.ConversationMember{}).
		Where("conversation_uuid = ?", convUUID).Count(&count).Error
	return count, err
}

// FindDirectByMembers 返回同时包含两个成员的 direct 会话；无则返回 gorm.ErrRecordNotFound。
func (r *ConversationStore) FindDirectByMembers(userA, userB string) (*model.Conversation, error) {
	var conv model.Conversation
	sub := r.db.Model(&model.ConversationMember{}).
		Select("conversation_uuid").
		Where("user_uuid IN ?", []string{userA, userB}).
		Group("conversation_uuid").
		Having("COUNT(DISTINCT user_uuid) = 2")
	err := r.db.Where("type = ? AND uuid IN (?)", model.ConversationTypeDirect, sub).
		First(&conv).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

// ListByUser 返回该用户参与的会话，按最近活跃倒序。
func (r *ConversationStore) ListByUser(userUUID string, limit int) ([]model.Conversation, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []model.Conversation
	err := r.db.
		Where("uuid IN (?)", r.db.Model(&model.ConversationMember{}).
			Select("conversation_uuid").Where("user_uuid = ?", userUUID)).
		Order("COALESCE(last_message_at, created_at) DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *ConversationStore) UpdateLastMessage(convUUID, msgUUID, content, senderUUID string, t time.Time) error {
	if len(content) > 200 {
		content = content[:200]
	}
	return r.db.Model(&model.Conversation{}).
		Where("uuid = ?", convUUID).
		Updates(map[string]interface{}{
			"last_message_uuid": msgUUID,
			"last_content":      content,
			"last_sender_uuid":  senderUUID,
			"last_message_at":   t,
		}).Error
}

// DeleteByUUID 删除会话行与成员行（最后一名成员离开群聊时调用）。
func (r *ConversationStore) DeleteByUUID(convUUID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_uuid = ?", convUUID).Delete(&model.ConversationMember{}).Error; err != nil {
			return err
		}
		return tx.Where("uuid = ?", convUUID).Delete(&model.Conversation{}).Error
	})
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/repository/ -run TestConversationStore -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/repository/conversation_store.go app/server/internal/repository/conversation_store_test.go
git commit -m "feat(im): add conversation store repository" -- app/server/internal/repository/conversation_store.go app/server/internal/repository/conversation_store_test.go
```

---

### 任务 5：房间消息落库路径接 read-state

**文件：**
- 修改：`app/server/internal/service/message_service.go`（结构体 + setter）、`app/server/internal/service/message_jobs.go`（PersistFromJob）
- 测试：`app/server/internal/service/message_service_test.go` 追加用例（沿用该文件既有 fakeBus/fakeQueue/sqlite 脚手架）

- [ ] **步骤 1：编写失败的测试**（追加到 `message_service_test.go`，按既有脚手架风格构造 service；`newSvcWithReadStates` 若无现成构造器则按文件内既有方式组装）

```go
// 独立命名的 fixture，避免与 message_service_test.go 既有助手重名冲突。
func newMsgReadStateFixture(t *testing.T) (*gorm.DB, *MessageService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Message{}, &model.MessageMention{}, &model.MessageReaction{},
		&model.User{}, &model.Room{}, &model.ReadState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewMessageService(repository.NewMessageRepository(db), repository.NewRoomRepository(db), nil)
	svc.SetUserRepo(repository.NewUserRepository(db))
	svc.SetReadStates(repository.NewReadStateRepository(db))
	return db, svc
}

func TestPersistFromJobBumpsRoomReadStates(t *testing.T) {
	db, s := newMsgReadStateFixture(t)
	// 房间内既有读者 uuid-reader，发送者 uuid-author
	if err := db.Create(&model.ReadState{UserUUID: "uuid-reader", ScopeType: model.ReadScopeRoom, ScopeID: "room-u"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"uuid": "m-rs-1", "room_uuid": "room-u",
		"author_id": "author", "author_uuid": "uuid-author",
		"content": "hi", "created_at": time.Now().UTC(),
	})
	if err := s.PersistFromJob(payload); err != nil {
		t.Fatalf("PersistFromJob: %v", err)
	}
	var row model.ReadState
	if err := db.First(&row, "user_uuid = ? AND scope_id = ?", "uuid-reader", "room-u").Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if row.UnreadCount != 1 {
		t.Fatalf("reader unread = %d, want 1", row.UnreadCount)
	}
}

func TestPersistFromJobMentionForcesReadStateRow(t *testing.T) {
	db, s := newMsgReadStateFixture(t)
	// mentions 传 identity（用户名），仓库按 name→uuid 解析
	_ = db.Create(&model.User{Name: "mentioned", UUID: "uuid-mentioned"}).Error
	payload, _ := json.Marshal(map[string]interface{}{
		"uuid": "m-rs-2", "room_uuid": "room-u",
		"author_id": "author", "author_uuid": "uuid-author",
		"content": "hi @mentioned", "mentions": []string{"mentioned"},
		"created_at": time.Now().UTC(),
	})
	if err := s.PersistFromJob(payload); err != nil {
		t.Fatalf("PersistFromJob: %v", err)
	}
	var row model.ReadState
	if err := db.First(&row, "user_uuid = ?", "uuid-mentioned").Error; err != nil {
		t.Fatalf("mention row must be created: %v", err)
	}
	if row.UnreadCount != 1 || row.MentionCount != 1 {
		t.Fatalf("mention row unread=%d mention=%d, want 1/1", row.UnreadCount, row.MentionCount)
	}
}
```

（消息落库走 fixture 内同一 `*gorm.DB`；`repository.NewRoomRepository(db)` 若构造器名有出入，以 repository 包实际名为准。）

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run 'TestPersistFromJobBumpsRoom|TestPersistFromJobMentionForces' -v`
预期：编译失败（`SetReadStates` 不存在）

- [ ] **步骤 3：实现**

`message_service.go`：`MessageService` 结构体（第 73-81 行）增两个字段：

```go
	readStates       *repository.ReadStateRepository
	convStore        *repository.ConversationStore
```

并新增 setter（放在 `SetJobQueue` 之后）：

```go
// SetReadStates 注入未读仓储，消息落库路径据此累加频道未读。
func (s *MessageService) SetReadStates(repo *repository.ReadStateRepository) {
	s.readStates = repo
}

// SetConversationStore 注入会话仓储，供 conversation-scope 消息操作做成员校验。
func (s *MessageService) SetConversationStore(store *repository.ConversationStore) {
	s.convStore = store
}
```

同文件追加 helper（放在 `persistMessageSync` 之后）：

```go
// resolveMentionUUIDs 将提及的 identity（用户名）解析为 UserUUID，忽略不存在者并排除作者本人。
func (s *MessageService) resolveMentionUUIDs(mentions []string, authorUUID string) []string {
	if len(mentions) == 0 || s.userRepo == nil {
		return nil
	}
	users, err := s.userRepo.GetByNames(mentions)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(users))
	for _, u := range users {
		if u.UUID == "" || u.UUID == authorUUID {
			continue
		}
		out = append(out, u.UUID)
	}
	return out
}
```

`message_jobs.go` 的 `PersistFromJob`：在 `return s.ensureJobMentions(data.UUID, data.Mentions)`（第 53 行）前插入：

```go
	if s.readStates != nil {
		mentionUUIDs := s.resolveMentionUUIDs(data.Mentions, data.AuthorUUID)
		if err := s.readStates.ApplyRoomMessage(data.RoomUUID, data.AuthorUUID, mentionUUIDs); err != nil {
			// 未读累加失败不阻塞消息持久化
		}
	}
```

（`_ = err` 或 log 均可，保持既有「未读失败仅日志」风格——conversation_service.go:339 的先例。）

`persistMessageSync`（message_service.go:295-315）在 `EnsureMentions` 成功后追加同样的 read-states 调用。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run 'TestPersistFromJob' -v`
预期：全部 PASS（含既有用例）

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/service/message_service.go app/server/internal/service/message_jobs.go app/server/internal/service/message_service_test.go
git commit -m "feat(im): bump room read-states on message persistence" -- app/server/internal/service/message_service.go app/server/internal/service/message_jobs.go app/server/internal/service/message_service_test.go
```

---

### 任务 6：ConversationService v2 — 会话管理方法族

**文件：**
- 修改：`app/server/internal/service/conversation_service.go`（新增方法族与依赖注入；旧方法暂不动）
- 测试：`app/server/internal/service/conversation_service_v2_test.go`（新建）

本任务把新依赖与新方法加入 `ConversationService`，旧四个方法（List/GetMessages/MarkRead/SendDirect）暂保持 md5 引擎不动（任务 8 切换），保证每个 commit 可编译可测。

- [ ] **步骤 1：编写失败的测试**

```go
package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
)

func newConvV2Fixture(t *testing.T) (*ConversationService, *repository.ConversationStore, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Conversation{}, &model.ConversationMember{}, &model.ReadState{},
		&model.User{}, &model.Domain{}, &model.DomainMember{}, &model.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := repository.NewConversationStore(db)
	// 任务 6 阶段构造器仍为三参（旧 ConversationRepository 尚未删除，任务 8 才收敛为两参）
	svc := NewConversationService(repository.NewConversationRepository(db), repository.NewMessageRepository(db), store)
	svc.SetUserRepo(repository.NewUserRepository(db))
	svc.SetReadStates(repository.NewReadStateRepository(db))
	svc.SetDomainChecker(domainCheckerFixture(db))
	return svc, store, db
}

type domainCheckerFixtureType struct{ db *gorm.DB }

func (d *domainCheckerFixtureType) HasCommonDomain(a, b string) (bool, error) {
	return repository.NewDomainRepository(d.db).HasCommonDomain(a, b)
}

func domainCheckerFixture(db *gorm.DB) *domainCheckerFixtureType { return &domainCheckerFixtureType{db: db} }

func seedDomainPair(t *testing.T, db *gorm.DB) {
	t.Helper()
	users := []model.User{
		{Name: "alice", UUID: "uuid-a", DisplayName: "Alice"},
		{Name: "bob", UUID: "uuid-b", DisplayName: "Bob"},
		{Name: "carol", UUID: "uuid-c", DisplayName: "Carol"},
		{Name: "stranger", UUID: "uuid-x", DisplayName: "X"},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	if err := db.Create(&model.Domain{UUID: "dom-1", Name: "d1"}).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	for _, m := range []model.DomainMember{
		{DomainUUID: "dom-1", UserUUID: "uuid-a", RoleName: "member"},
		{DomainUUID: "dom-1", UserUUID: "uuid-b", RoleName: "member"},
		{DomainUUID: "dom-1", UserUUID: "uuid-c", RoleName: "member"},
	} {
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("seed member: %v", err)
		}
	}
}

func TestCreateDirectIdempotent(t *testing.T) {
	svc, store, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	actor := MessageActor{Identity: "alice", UserUUID: "uuid-a"}

	first, err := svc.Create(actor, model.ConversationTypeDirect, []string{"uuid-b"})
	if err != nil {
		t.Fatalf("create direct: %v", err)
	}
	second, err := svc.Create(actor, model.ConversationTypeDirect, []string{"uuid-b"})
	if err != nil {
		t.Fatalf("create direct again: %v", err)
	}
	if first.ConversationID != second.ConversationID {
		t.Fatalf("direct create must be idempotent: %s vs %s", first.ConversationID, second.ConversationID)
	}
	if len(first.Members) != 2 {
		t.Fatalf("direct members = %d, want 2", len(first.Members))
	}
	if _, err := store.GetByUUID(first.ConversationID); err != nil {
		t.Fatalf("conversation persisted: %v", err)
	}
	_ = db
}

func TestCreateDirectRequiresCommonDomain(t *testing.T) {
	svc, _, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	actor := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	if _, err := svc.Create(actor, model.ConversationTypeDirect, []string{"uuid-x"}); err == nil {
		t.Fatal("stranger without common domain must be rejected")
	}
}

func TestCreateDirectRejectsSelfBotGuest(t *testing.T) {
	svc, _, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	_ = db.Create(&model.User{Name: "gg", UUID: "uuid-g", IsGuest: true}).Error
	_ = db.Create(&model.DomainMember{DomainUUID: "dom-1", UserUUID: "uuid-g", RoleName: "guest"}).Error
	actor := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	if _, err := svc.Create(actor, model.ConversationTypeDirect, []string{"uuid-g"}); err == nil {
		t.Fatal("guest must not be DM-able")
	}
	if _, err := svc.Create(actor, model.ConversationTypeDirect, []string{"uuid-a"}); err == nil {
		t.Fatal("self DM must be rejected")
	}
}

func TestCreateGroupAndInviteAndLeave(t *testing.T) {
	svc, store, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	actor := MessageActor{Identity: "alice", UserUUID: "uuid-a"}

	conv, err := svc.Create(actor, model.ConversationTypeGroup, []string{"uuid-b", "uuid-c"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if conv.Type != model.ConversationTypeGroup || len(conv.Members) != 3 {
		t.Fatalf("group summary wrong: %+v", conv)
	}

	// bob 邀请 stranger 失败（无共同域）； alice 不能 invite 到 direct 会话
	stranger := MessageActor{Identity: "bob", UserUUID: "uuid-b"}
	if _, err := svc.Invite(stranger, conv.ConversationID, []string{"uuid-x"}); err == nil {
		t.Fatal("cross-domain invite must fail")
	}

	// carol 离开
	if err := svc.Leave(MessageActor{Identity: "carol", UserUUID: "uuid-c"}, conv.ConversationID); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if ok, _ := store.IsMember(conv.ConversationID, "uuid-c"); ok {
		t.Fatal("carol should have left")
	}
}

func TestCreateGroupCapFifty(t *testing.T) {
	svc, _, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	actor := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	tooMany := make([]string, 0, model.MaxConversationMembers)
	for i := 0; i < model.MaxConversationMembers; i++ {
		u := &model.User{Name: "u" + string(rune('a'+i%26)) + fmt.Sprint(i), UUID: fmt.Sprintf("uuid-n%d", i)}
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
		_ = db.Create(&model.DomainMember{DomainUUID: "dom-1", UserUUID: u.UUID, RoleName: "member"}).Error
		tooMany = append(tooMany, u.UUID)
	}
	if _, err := svc.Create(actor, model.ConversationTypeGroup, tooMany); err == nil {
		t.Fatal("group over cap must be rejected")
	}
}

func TestSummariesIncludesUnreadAndType(t *testing.T) {
	svc, _, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	actor := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	conv, err := svc.Create(actor, model.ConversationTypeDirect, []string{"uuid-b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	readStates := repository.NewReadStateRepository(db)
	_ = db.Create(&model.ReadState{UserUUID: "uuid-a", ScopeType: model.ReadScopeConversation,
		ScopeID: conv.ConversationID, UnreadCount: 3, MentionCount: 1}).Error
	_ = readStates
	list, err := svc.Summaries(actor, 10)
	if err != nil {
		t.Fatalf("Summaries: %v", err)
	}
	var found *ConversationSummary
	for i := range list {
		if list[i].ConversationID == conv.ConversationID {
			found = &list[i]
		}
	}
	if found == nil {
		t.Fatal("summary missing")
	}
	if found.UnreadCount != 3 || found.MentionCount != 1 || found.Type != model.ConversationTypeDirect {
		t.Fatalf("summary fields wrong: %+v", found)
	}
}
```

注意：测试文件 import 需含 `fmt` 与 `"gorm.io/gorm"`、`"github.com/glebarez/sqlite"`。`TestSummariesIncludesUnreadAndType` 中的 `readStates` 局部变量若未使用可删除（fixture 已注入）。

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run 'TestCreateDirect|TestCreateGroup|TestSummaries' -v`
预期：编译失败（方法/类型不存在）

- [ ] **步骤 3：实现**

`conversation_service.go`：

依赖与接口（加在 struct 定义附近）：

```go
type conversationUserLookup interface {
	GetByName(name string) (*model.User, error)
	GetByUUIDs(uuids []string) (map[string]*model.User, error)
}

type commonDomainChecker interface {
	HasCommonDomain(userA, userB string) (bool, error)
}
```

`ConversationService` struct 增字段（旧字段保留）：

```go
	store          *repository.ConversationStore
	readStates     *repository.ReadStateRepository
	userRepo       conversationUserLookup
	domainChecker  commonDomainChecker
```

构造器改为：

```go
func NewConversationService(
	convRepo *repository.ConversationRepository,
	messageRepo *repository.MessageRepository,
	store *repository.ConversationStore,
) *ConversationService {
	return &ConversationService{
		convRepo:         convRepo,
		messageRepo:      messageRepo,
		store:            store,
		syncWriteAllowed: true,
	}
}
```

新增 setter：

```go
func (s *ConversationService) SetUserRepo(repo conversationUserLookup) { s.userRepo = repo }
func (s *ConversationService) SetReadStates(repo *repository.ReadStateRepository) { s.readStates = repo }
func (s *ConversationService) SetDomainChecker(c commonDomainChecker) { s.domainChecker = c }
```

新 DTO：

```go
type ConversationMemberView struct {
	UserUUID    string `json:"user_uuid"`
	Identity    string `json:"identity"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar,omitempty"`
}

type ConversationSummary struct {
	ConversationID string                   `json:"conversation_id"`
	Type           string                   `json:"type"`
	LastContent    string                   `json:"last_content"`
	LastSenderUUID string                   `json:"last_sender_uuid"`
	LastMessageAt  int64                    `json:"last_message_at"`
	UnreadCount    int                      `json:"unread_count"`
	MentionCount   int                      `json:"mention_count"`
	Members        []ConversationMemberView `json:"members"`
}
```

成员校验 helper：

```go
func (s *ConversationService) requireMember(convUUID, userUUID string) (*model.Conversation, error) {
	conv, err := s.store.GetByUUID(convUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "conversation not found")
	}
	ok, err := s.store.IsMember(convUUID, userUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !ok {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not a conversation member")
	}
	return conv, nil
}
```

投影 helper（批量取用户 + read_states）：

```go
func (s *ConversationService) buildSummaries(convs []model.Conversation, viewerUUID string) []ConversationSummary {
	memberUUIDSet := make(map[string]struct{})
	convIDs := make([]string, 0, len(convs))
	for _, c := range convs {
		convIDs = append(convIDs, c.UUID)
	}
	type memberRow struct {
		ConversationUUID string
		UserUUID         string
	}
	var members []memberRow
	if len(convIDs) > 0 {
		_ = s.store.DB().Model(&model.ConversationMember{}).
			Where("conversation_uuid IN ?", convIDs).
			Order("user_uuid").
			Find(&members).Error
	}
	for _, m := range members {
		memberUUIDSet[m.UserUUID] = struct{}{}
	}
	uuids := make([]string, 0, len(memberUUIDSet))
	for u := range memberUUIDSet {
		uuids = append(uuids, u)
	}
	users, _ := s.userRepo.GetByUUIDs(uuids)

	states := map[string]*model.ReadState{}
	if s.readStates != nil {
		if rows, err := s.readStates.ListByUser(viewerUUID); err == nil {
			for i := range rows {
				states[rows[i].ScopeID] = &rows[i]
			}
		}
	}

	byConv := map[string][]ConversationMemberView{}
	for _, m := range members {
		u := users[m.UserUUID]
		view := ConversationMemberView{UserUUID: m.UserUUID}
		if u != nil {
			view.Identity = u.Name
			view.DisplayName = u.DisplayName
			if view.DisplayName == "" {
				view.DisplayName = u.Name
			}
			view.Avatar = u.Avatar
		}
		byConv[m.ConversationUUID] = append(byConv[m.ConversationUUID], view)
	}

	out := make([]ConversationSummary, 0, len(convs))
	for _, c := range convs {
		lat := int64(0)
		if c.LastMessageAt != nil {
			lat = c.LastMessageAt.UnixMilli()
		}
		sum := ConversationSummary{
			ConversationID: c.UUID,
			Type:           c.Type,
			LastContent:    c.LastContent,
			LastSenderUUID: c.LastSenderUUID,
			LastMessageAt:  lat,
			Members:        byConv[c.UUID],
		}
		if st, ok := states[c.UUID]; ok && st.ScopeType == model.ReadScopeConversation {
			sum.UnreadCount = st.UnreadCount
			sum.MentionCount = st.MentionCount
		}
		out = append(out, sum)
	}
	return out
}
```

（为此 ConversationStore 增加只读访问器 `func (r *ConversationStore) DB() *gorm.DB { return r.db }`，写入 conversation_store.go。）

Create：

```go
func (s *ConversationService) Create(actor MessageActor, convType string, memberUUIDs []string) (*ConversationSummary, error) {
	if actor.UserUUID == "" {
		return nil, pkg.NewAppError(pkg.TOKEN_WRONG, "user_uuid is required")
	}
	switch convType {
	case model.ConversationTypeDirect:
		return s.createDirect(actor, memberUUIDs)
	case model.ConversationTypeGroup:
		return s.createGroup(actor, memberUUIDs)
	default:
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid conversation type")
	}
}

func (s *ConversationService) validateDMTarget(targetUUID string) (*model.User, error) {
	users, err := s.userRepo.GetByUUIDs([]string{targetUUID})
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	u, ok := users[targetUUID]
	if !ok {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "user not found")
	}
	if u.IsBot {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "cannot DM a bot")
	}
	if u.IsGuest {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "cannot DM a guest")
	}
	return u, nil
}

func (s *ConversationService) requireCommonDomain(a, b string) error {
	if s.domainChecker == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "domain checker unavailable")
	}
	ok, err := s.domainChecker.HasCommonDomain(a, b)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !ok {
		return pkg.NewAppError(pkg.FORBIDDEN, "no shared server with this user")
	}
	return nil
}

func (s *ConversationService) createDirect(actor MessageActor, memberUUIDs []string) (*ConversationSummary, error) {
	if len(memberUUIDs) != 1 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "direct conversation requires exactly one member")
	}
	targetUUID := memberUUIDs[0]
	if targetUUID == actor.UserUUID {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "cannot DM yourself")
	}
	if _, err := s.validateDMTarget(targetUUID); err != nil {
		return nil, err
	}
	if err := s.requireCommonDomain(actor.UserUUID, targetUUID); err != nil {
		return nil, err
	}
	if conv, err := s.store.FindDirectByMembers(actor.UserUUID, targetUUID); err == nil {
		return s.buildSummaries([]model.Conversation{*conv}, actor.UserUUID)[0], nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	conv := &model.Conversation{Type: model.ConversationTypeDirect}
	members := []model.ConversationMember{
		{UserUUID: actor.UserUUID, JoinedAt: time.Now().UTC()},
		{UserUUID: targetUUID, JoinedAt: time.Now().UTC()},
	}
	if err := s.store.CreateConversation(conv, members); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.ensureMemberReadStates(conv.UUID, []string{actor.UserUUID, targetUUID})
	s.notifyConversationCreated(conv.UUID)
	return s.buildSummaries([]model.Conversation{*conv}, actor.UserUUID)[0], nil
}

func (s *ConversationService) createGroup(actor MessageActor, memberUUIDs []string) (*ConversationSummary, error) {
	if len(memberUUIDs) < 2 || 1+len(memberUUIDs) > model.MaxConversationMembers {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "group requires 2..49 invitees")
	}
	seen := map[string]struct{}{actor.UserUUID: {}}
	for _, u := range memberUUIDs {
		if _, dup := seen[u]; dup {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "duplicate member")
		}
		seen[u] = struct{}{}
		if _, err := s.validateDMTarget(u); err != nil {
			return nil, err
		}
		if err := s.requireCommonDomain(actor.UserUUID, u); err != nil {
			return nil, err
		}
	}
	conv := &model.Conversation{Type: model.ConversationTypeGroup}
	now := time.Now().UTC()
	members := make([]model.ConversationMember, 0, len(memberUUIDs)+1)
	members = append(members, model.ConversationMember{UserUUID: actor.UserUUID, JoinedAt: now})
	for _, u := range memberUUIDs {
		members = append(members, model.ConversationMember{UserUUID: u, InvitedBy: actor.UserUUID, JoinedAt: now})
	}
	if err := s.store.CreateConversation(conv, members); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	uuids := make([]string, 0, len(members))
	for _, m := range members {
		uuids = append(uuids, m.UserUUID)
	}
	s.ensureMemberReadStates(conv.UUID, uuids)
	s.notifyConversationCreated(conv.UUID)
	return s.buildSummaries([]model.Conversation{*conv}, actor.UserUUID)[0], nil
}
```

Invite / Leave：

```go
func (s *ConversationService) Invite(actor MessageActor, conversationUUID string, memberUUIDs []string) (*ConversationSummary, error) {
	conv, err := s.requireMember(conversationUUID, actor.UserUUID)
	if err != nil {
		return nil, err
	}
	if conv.Type != model.ConversationTypeGroup {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "only group conversations can be invited")
	}
	if len(memberUUIDs) == 0 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "member_uuids required")
	}
	count, err := s.store.CountMembers(conversationUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if int(count)+len(memberUUIDs) > model.MaxConversationMembers {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "group is full")
	}
	now := time.Now().UTC()
	added := make([]model.ConversationMember, 0, len(memberUUIDs))
	for _, u := range memberUUIDs {
		if _, err := s.validateDMTarget(u); err != nil {
			return nil, err
		}
		if err := s.requireCommonDomain(actor.UserUUID, u); err != nil {
			return nil, err
		}
		added = append(added, model.ConversationMember{ConversationUUID: conversationUUID, UserUUID: u, InvitedBy: actor.UserUUID, JoinedAt: now})
	}
	for i := range added {
		if err := s.store.AddMember(&added[i]); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	uuids := make([]string, 0, len(added))
	for _, m := range added {
		uuids = append(uuids, m.UserUUID)
	}
	s.ensureMemberReadStates(conversationUUID, uuids)
	s.notifyMemberChange(conv, actor.UserUUID, "conversation:member-joined")
	return s.Get(actor, conversationUUID)
}

func (s *ConversationService) Leave(actor MessageActor, conversationUUID string) error {
	conv, err := s.requireMember(conversationUUID, actor.UserUUID)
	if err != nil {
		return err
	}
	if conv.Type != model.ConversationTypeGroup {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "cannot leave a direct conversation")
	}
	if err := s.store.RemoveMember(conversationUUID, actor.UserUUID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.notifyMemberChange(conv, actor.UserUUID, "conversation:member-left")
	count, err := s.store.CountMembers(conversationUUID)
	if err == nil && count == 0 {
		_ = s.store.DeleteByUUID(conversationUUID)
	}
	return nil
}
```

Get / Summaries：

```go
func (s *ConversationService) Get(actor MessageActor, conversationUUID string) (*ConversationSummary, error) {
	if _, err := s.requireMember(conversationUUID, actor.UserUUID); err != nil {
		return nil, err
	}
	conv, err := s.store.GetByUUID(conversationUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "conversation not found")
	}
	return s.buildSummaries([]model.Conversation{*conv}, actor.UserUUID)[0], nil
}

func (s *ConversationService) Summaries(actor MessageActor, limit int) ([]ConversationSummary, error) {
	if actor.UserUUID == "" {
		return nil, pkg.NewAppError(pkg.TOKEN_WRONG, "user_uuid is required")
	}
	rows, err := s.store.ListByUser(actor.UserUUID, limit)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.buildSummaries(rows, actor.UserUUID), nil
}
```

内部通知 helpers（任务 7 补 SendToConversation；本任务先落 created/member-change 通知与 read-state 行初始化）：

```go
func (s *ConversationService) ensureMemberReadStates(convUUID string, memberUUIDs []string) {
	if s.readStates == nil {
		return
	}
	for _, u := range memberUUIDs {
		if err := s.readStates.EnsureScopeRow(model.ReadScopeConversation, convUUID, u); err != nil {
			log.Printf("[Conversation] ensure read state %s/%s failed: %v", convUUID, u, err)
		}
	}
}

func (s *ConversationService) publishToUsers(identities []string, event string, payload interface{}) {
	if s.eventBus == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	for _, id := range identities {
		if id == "" {
			continue
		}
		if err := s.eventBus.PublishRoom(context.Background(), "__user:"+id, event, json.RawMessage(data)); err != nil {
			log.Printf("[Conversation] publish %s to %s failed: %v", event, id, err)
		}
	}
}

func (s *ConversationService) memberSummariesForDelivery(convUUID string) ([]ConversationMemberView, []string) {
	uuids, err := s.store.GetMemberUUIDs(convUUID)
	if err != nil {
		return nil, nil
	}
	users, _ := s.userRepo.GetByUUIDs(uuids)
	views := make([]ConversationMemberView, 0, len(uuids))
	identities := make([]string, 0, len(uuids))
	for _, u := range uuids {
		v := ConversationMemberView{UserUUID: u}
		if user := users[u]; user != nil {
			v.Identity = user.Name
			v.DisplayName = user.DisplayName
			if v.DisplayName == "" {
				v.DisplayName = user.Name
			}
			v.Avatar = user.Avatar
		}
		views = append(views, v)
		identities = append(identities, v.Identity)
	}
	return views, identities
}

func (s *ConversationService) notifyConversationCreated(convUUID string) {
	views, identities := s.memberSummariesForDelivery(convUUID)
	s.publishToUsers(identities, "conversation:created", map[string]interface{}{
		"conversation_id": convUUID,
		"members":         views,
	})
}

func (s *ConversationService) notifyMemberChange(conv *model.Conversation, actorUUID, event string) {
	views, identities := s.memberSummariesForDelivery(conv.UUID)
	s.publishToUsers(identities, event, map[string]interface{}{
		"conversation_id": conv.UUID,
		"actor_uuid":      actorUUID,
		"members":         views,
	})
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run 'TestCreateDirect|TestCreateGroup|TestSummaries' -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/service/conversation_service.go app/server/internal/service/conversation_service_v2_test.go app/server/internal/repository/conversation_store.go
git commit -m "feat(im): conversation service v2 create/invite/leave/summaries" -- app/server/internal/service/conversation_service.go app/server/internal/service/conversation_service_v2_test.go app/server/internal/repository/conversation_store.go
```

---

### 任务 7：ConversationService v2 — 消息发送与持久化管道

**文件：**
- 修改：`app/server/internal/service/conversation_service.go`
- 测试：追加到 `app/server/internal/service/conversation_service_v2_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestSendToConversationDirect(t *testing.T) {
	svc, _, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	bus := &fakeBus{}
	svc.SetEventBus(bus)
	queue := &fakeQueue{}
	svc.SetJobQueue(queue)
	actor := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	conv, err := svc.Create(actor, model.ConversationTypeDirect, []string{"uuid-b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	dto, err := svc.SendToConversation(actor, conv.ConversationID, "hello world", "nonce-1", nil)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if dto.ConversationID != conv.ConversationID {
		t.Fatalf("dto conversation_id = %q", dto.ConversationID)
	}
	// conversation:new 双向投递 + direct 兼容 private:new 双发 = 4 次 PublishRoom
	if len(bus.calls) != 4 {
		t.Fatalf("publishes = %d (%v), want 4", len(bus.calls), bus.calls)
	}
	newCount, privCount := 0, 0
	for _, ev := range bus.calls {
		switch ev {
		case "conversation:new":
			newCount++
		case "private:new":
			privCount++
		}
	}
	if newCount != 2 || privCount != 2 {
		t.Fatalf("conversation:new=%d private:new=%d, want 2/2", newCount, privCount)
	}
	// job 已入队
	if len(queue.jobs) != 1 || queue.jobs[0].Type != "chat.private.persist" {
		t.Fatalf("queue = %+v", queue.jobs)
	}
	// 未读：bob 未读 +1
	var row model.ReadState
	if err := db.First(&row, "user_uuid = ? AND scope_id = ?", "uuid-b", conv.ConversationID).Error; err != nil {
		t.Fatalf("load read state: %v", err)
	}
	if row.UnreadCount != 1 {
		t.Fatalf("bob unread = %d, want 1", row.UnreadCount)
	}
}

func TestPersistPrivateFromJobV2(t *testing.T) {
	svc, store, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	actor := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	conv, _ := svc.Create(actor, model.ConversationTypeDirect, []string{"uuid-b"})

	payload, _ := json.Marshal(map[string]interface{}{
		"uuid":              "pm-1",
		"conversation_uuid": conv.ConversationID,
		"sender_identity":   "alice",
		"sender_uuid":       "uuid-a",
		"content":           "persisted",
		"created_at":        time.Now().UTC(),
	})
	if err := svc.PersistPrivateFromJob(payload); err != nil {
		t.Fatalf("PersistPrivateFromJob: %v", err)
	}
	stored, err := store.GetByUUID(conv.ConversationID)
	if err != nil || stored.LastContent != "persisted" {
		t.Fatalf("last message not updated: %+v err=%v", stored, err)
	}
	var msg model.Message
	if err := db.First(&msg, "uuid = ?", "pm-1").Error; err != nil {
		t.Fatalf("message persisted: %v", err)
	}
	if msg.ConversationID == nil || *msg.ConversationID != conv.ConversationID {
		t.Fatalf("message conversation_id wrong: %+v", msg)
	}
	// 幂等重放
	if err := svc.PersistPrivateFromJob(payload); err != nil {
		t.Fatalf("replay: %v", err)
	}
}

func TestSendToConversationRequiresMember(t *testing.T) {
	svc, _, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	actor := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	conv, _ := svc.Create(actor, model.ConversationTypeDirect, []string{"uuid-b"})
	outsider := MessageActor{Identity: "stranger", UserUUID: "uuid-x"}
	if _, err := svc.SendToConversation(outsider, conv.ConversationID, "hi", "", nil); err == nil {
		t.Fatal("non-member send must be rejected")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run 'TestSendToConversation|TestPersistPrivateFromJobV2' -v`
预期：编译失败

- [ ] **步骤 3：实现**

`conversation_service.go` 追加：

```go
// SendToConversation 在 direct/group 会话中发送消息：广播 conversation:new（全员定向投递），
// direct 额外兼容双发 private:new，然后入队 chat.private.persist 异步持久化。
func (s *ConversationService) SendToConversation(actor MessageActor, conversationUUID, content, clientNonce string, mentions []string) (*MessageDTO, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content is required")
	}
	if utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content too long")
	}
	if actor.UserUUID == "" {
		return nil, pkg.NewAppError(pkg.TOKEN_WRONG, "user_uuid is required")
	}
	conv, err := s.requireMember(conversationUUID, actor.UserUUID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	msgUUID := uuid.New().String()
	dto := &MessageDTO{
		UUID:           msgUUID,
		AuthorID:       actor.Identity,
		AuthorUUID:     actor.UserUUID,
		Content:        content,
		Deleted:        false,
		CreatedAt:      now,
		ClientNonce:    clientNonce,
		ConversationID: conv.UUID,
	}
	if conv.Type == model.ConversationTypeDirect {
		// 1:1 兼容投影：target = 对方
		_, identities := s.memberSummariesForDelivery(conv.UUID)
		for _, id := range identities {
			if id != actor.Identity {
				dto.TargetIdentity = id
			}
		}
	}

	_, identities := s.memberSummariesForDelivery(conv.UUID)
	// conversation:new 载荷 = 会话投影 + 消息（规格 §5）
	summary := s.buildSummaries([]model.Conversation{*conv}, actor.UserUUID)[0]
	s.publishToUsers(identities, "conversation:new", map[string]interface{}{
		"conversation": summary,
		"message":      dto,
	})
	if conv.Type == model.ConversationTypeDirect {
		// 兼容双发：旧前端消费裸 DTO 形态的 private:new
		s.publishToUsers(identities, "private:new", dto)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"uuid":              msgUUID,
		"conversation_uuid": conv.UUID,
		"sender_identity":   actor.Identity,
		"sender_uuid":       actor.UserUUID,
		"content":           content,
		"mentions":          mentions,
		"client_nonce":      clientNonce,
		"created_at":        now,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      msgUUID,
			Type:    "chat.private.persist",
			Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		if !s.syncWriteAllowed {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "message persistence unavailable on data-plane instance")
		}
		if err := s.persistConversationMessage(msgUUID, conv.UUID, actor.Identity, actor.UserUUID, content, mentions, clientNonce, now); err != nil {
			return nil, err
		}
	}
	return dto, nil
}

// PersistPrivateFromJob applies a conversation-message persistence job idempotently.
func (s *ConversationService) PersistPrivateFromJob(payload []byte) error {
	if !s.syncWriteAllowed {
		return errors.New("private message persistence is not allowed on data-plane instance")
	}
	var data struct {
		UUID             string    `json:"uuid"`
		ConversationUUID string    `json:"conversation_uuid"`
		SenderIdentity   string    `json:"sender_identity"`
		SenderUUID       string    `json:"sender_uuid"`
		Content          string    `json:"content"`
		Mentions         []string  `json:"mentions"`
		ClientNonce      string    `json:"client_nonce"`
		CreatedAt        time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}
	if data.UUID == "" || data.ConversationUUID == "" || data.SenderUUID == "" || data.Content == "" {
		return fmt.Errorf("invalid private message job")
	}
	ts := data.CreatedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	return s.persistConversationMessage(data.UUID, data.ConversationUUID, data.SenderIdentity, data.SenderUUID, data.Content, data.Mentions, data.ClientNonce, ts)
}

func (s *ConversationService) persistConversationMessage(msgUUID, convUUID, senderIdentity, senderUUID, content string, mentions []string, clientNonce string, now time.Time) error {
	if _, err := s.messageRepo.GetByUUID(msgUUID); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		convIDPtr := convUUID
		msg := &model.Message{
			UUID:             msgUUID,
			AuthorID:         senderIdentity,
			AuthorUUID:       senderUUID,
			Content:          content,
			CreatedAt:        now,
			UpdatedAt:        now,
			ConversationType: model.ConversationTypePrivate,
			ConversationID:   &convIDPtr,
			Status:           model.MessageStatusActive,
		}
		if len(mentions) > 0 {
			rows := make([]model.MessageMention, 0, len(mentions))
			for _, uid := range mentions {
				rows = append(rows, model.MessageMention{MessageUUID: msgUUID, UserID: uid})
			}
			if err := s.messageRepo.EnsureMentions(rows); err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
		}
		if err := s.messageRepo.Create(msg); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	if err := s.store.UpdateLastMessage(convUUID, msgUUID, content, senderUUID, now); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.readStates != nil {
		mentionUUIDs := s.resolveConversationMentionUUIDs(mentions, senderUUID)
		if err := s.readStates.ApplyConversationMessage(convUUID, senderUUID, mentionUUIDs); err != nil {
			log.Printf("[Conversation] apply read states %s failed: %v", convUUID, err)
		}
	}
	return nil
}

// resolveConversationMentionUUIDs 兼容 identity/UUID 两种提及载荷：UUID 形态直接透传，其余按用户名解析。
func (s *ConversationService) resolveConversationMentionUUIDs(mentions []string, senderUUID string) []string {
	if len(mentions) == 0 || s.userRepo == nil {
		return nil
	}
	users, err := s.userRepo.GetByUUIDs(mentions)
	if err != nil {
		return nil
	}
	resolved := make(map[string]struct{}, len(users))
	for _, u := range users {
		resolved[u.UUID] = struct{}{}
	}
	out := make([]string, 0, len(mentions))
	for _, m := range mentions {
		if _, isUUID := resolved[m]; isUUID {
			out = append(out, m)
			continue
		}
		u, err := s.userRepo.GetByName(m)
		if err != nil || u == nil || u.UUID == "" || u.UUID == senderUUID {
			continue
		}
		out = append(out, u.UUID)
	}
	return out
}
```

注意：`GetByNames` 已在 userByName 接口里，但 conversationUserLookup 只有 GetByName/GetByUUIDs——`resolveConversationMentionUUIDs` 用 `GetByName` 逐个解析即可（如上），无需扩接口。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run 'TestSendToConversation|TestPersistPrivateFromJobV2' -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/service/conversation_service.go app/server/internal/service/conversation_service_v2_test.go
git commit -m "feat(im): conversation message send + persist pipeline with read-states" -- app/server/internal/service/conversation_service.go app/server/internal/service/conversation_service_v2_test.go
```

---

### 任务 8：兼容切换 + 旧代码删除

**文件：**
- 修改：`app/server/internal/service/conversation_service.go`（重写 List/GetMessages/MarkRead/SendDirect 四个兼容方法；删除 md5 与旧仓储引用）
- 删除：`app/server/internal/model/conversation_participant.go`、`app/server/internal/repository/conversation_repo.go`
- 修改：`app/server/internal/repository/db_migrations.go`（确认已无 conversation_participants 引用）
- 测试：`app/server/internal/service/conversation_compat_test.go`（新建，旧格式字段断言）

- [ ] **步骤 1：编写失败的测试**

```go
package service

import (
	"testing"
	"time"

	"GOSpeak/internal/model"
)

func TestCompatListShape(t *testing.T) {
	svc, _, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	bus := &fakeBus{}
	svc.SetEventBus(bus)
	alice := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	conv, _ := svc.Create(alice, model.ConversationTypeDirect, []string{"uuid-b"})
	_, _ = svc.SendToConversation(alice, conv.ConversationID, "hi bob", "n1", nil)

	// 旧端点签名：按 identity（用户名）调用
	rows, err := svc.List("bob", 50)
	if err != nil {
		t.Fatalf("compat List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.OtherIdentity != "alice" || row.UnreadCount != 1 {
		t.Fatalf("compat fields wrong: %+v", row)
	}
	if row.ConversationID != conv.ConversationID {
		t.Fatalf("compat conversation_id must be new UUID: %q", row.ConversationID)
	}
	if row.LastContent != "hi bob" || row.LastMessageAt == 0 {
		t.Fatalf("last message projection wrong: %+v", row)
	}
}

func TestCompatGetMessagesAndMarkRead(t *testing.T) {
	svc, _, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	svc.SetEventBus(&fakeBus{})
	alice := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	conv, _ := svc.Create(alice, model.ConversationTypeDirect, []string{"uuid-b"})
	_, _ = svc.SendToConversation(alice, conv.ConversationID, "first", "n1", nil)
	_, _ = svc.SendToConversation(alice, conv.ConversationID, "second", "n2", nil)

	out, err := svc.GetMessages(conv.ConversationID, "bob", "", 50)
	if err != nil {
		t.Fatalf("compat GetMessages: %v", err)
	}
	if len(out.Messages) != 2 || out.UnreadCount != 2 {
		t.Fatalf("messages=%d unread=%d, want 2/2", len(out.Messages), out.UnreadCount)
	}
	if out.Messages[0].Content != "first" || out.Messages[1].Content != "second" {
		t.Fatalf("messages must be ASC: %+v", out.Messages)
	}
	if out.NextCursor == "" {
		t.Fatal("next_cursor should be set when hasMore")
	}

	if err := svc.MarkRead(conv.ConversationID, "bob"); err != nil {
		t.Fatalf("compat MarkRead: %v", err)
	}
	out2, _ := svc.GetMessages(conv.ConversationID, "bob", "", 50)
	if out2.UnreadCount != 0 {
		t.Fatalf("unread after mark-read = %d, want 0", out2.UnreadCount)
	}
	_ = time.Now
}

func TestCompatSendDirect(t *testing.T) {
	svc, _, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	svc.SetEventBus(&fakeBus{})
	dto, err := svc.SendDirect("alice", "bob", "legacy send", "n1")
	if err != nil {
		t.Fatalf("compat SendDirect: %v", err)
	}
	if dto.TargetIdentity != "bob" || dto.ConversationID == "" {
		t.Fatalf("dto wrong: %+v", dto)
	}
	// 幂等：再次发送复用同一会话
	dto2, err := svc.SendDirect("bob", "alice", "reply", "n2")
	if err != nil || dto2.ConversationID != dto.ConversationID {
		t.Fatalf("direct conversation must be reused: %q vs %q", dto2.ConversationID, dto.ConversationID)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run 'TestCompat' -v`
预期：FAIL（旧实现仍写 md5 / 双轨，字段对不上）

- [ ] **步骤 3：重写四个兼容方法**

`conversation_service.go`：删除 `ConversationRepository` 字段 `convRepo` 及 `crypto/md5`、`encoding/hex`、`sort` import；构造器去掉 `convRepo` 参数（恢复双参签名）：

```go
func NewConversationService(
	messageRepo *repository.MessageRepository,
	store *repository.ConversationStore,
) *ConversationService {
	return &ConversationService{messageRepo: messageRepo, store: store, syncWriteAllowed: true}
}
```

四个兼容方法重写（替换原实现）：

```go
// List 兼容旧端点 /conversation/list：以 identity（用户名）查询，投影旧 ConversationDTO。
func (s *ConversationService) List(identity string, limit int) ([]ConversationDTO, error) {
	if identity == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "identity required")
	}
	u, err := s.userRepo.GetByName(identity)
	if err != nil || u == nil {
		return []ConversationDTO{}, nil
	}
	sums, err := s.Summaries(MessageActor{Identity: identity, UserUUID: u.UUID}, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ConversationDTO, 0, len(sums))
	for _, sum := range sums {
		if sum.Type != model.ConversationTypeDirect {
			// 旧前端仅识别 1:1；群聊在 P2 切换后经 summaries 暴露
			continue
		}
		other := ConversationMemberView{}
		for _, m := range sum.Members {
			if m.UserUUID != u.UUID {
				other = m
			}
		}
		out = append(out, ConversationDTO{
			ConversationID:     sum.ConversationID,
			OtherIdentity:      other.Identity,
			OtherDisplayName:   other.DisplayName,
			OtherAvatar:        other.Avatar,
			LastContent:        sum.LastContent,
			LastSenderIdentity: s.identityOfUUID(sum.LastSenderUUID),
			LastMessageAt:      sum.LastMessageAt,
			UnreadCount:        sum.UnreadCount,
		})
	}
	return out, nil
}

func (s *ConversationService) identityOfUUID(userUUID string) string {
	if userUUID == "" || s.userRepo == nil {
		return ""
	}
	users, err := s.userRepo.GetByUUIDs([]string{userUUID})
	if err != nil || users[userUUID] == nil {
		return ""
	}
	return users[userUUID].Name
}

// GetMessages 兼容旧端点 /conversation/messages：conversation_id 现为会话 UUID。
func (s *ConversationService) GetMessages(conversationID, identity, before string, limit int) (*MessageListResult, error) {
	if conversationID == "" || identity == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "conversation_id and identity required")
	}
	u, err := s.userRepo.GetByName(identity)
	if err != nil || u == nil {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not a conversation participant")
	}
	if _, err := s.requireMember(conversationID, u.UUID); err != nil {
		return nil, err
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
	unread := 0
	if s.readStates != nil {
		if st, err := s.readStates.Get(model.ReadScopeConversation, conversationID, u.UUID); err == nil && st != nil {
			unread = st.UnreadCount
		}
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

// MarkRead 兼容旧端点 /conversation/mark-read。
func (s *ConversationService) MarkRead(conversationID, identity string) error {
	if conversationID == "" || identity == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "conversation_id and identity required")
	}
	u, err := s.userRepo.GetByName(identity)
	if err != nil || u == nil {
		return pkg.NewAppError(pkg.FORBIDDEN, "not a conversation participant")
	}
	if _, err := s.requireMember(conversationID, u.UUID); err != nil {
		return err
	}
	if s.readStates == nil {
		return nil
	}
	if err := s.readStates.MarkRead(model.ReadScopeConversation, conversationID, u.UUID, ""); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// SendDirect 兼容旧端点 /conversation/send 与 WS private:send：按用户名解析双方，
// 幂等复用/创建 direct 会话后走统一 conversation 发送管道。
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
	sender, err := s.userRepo.GetByName(senderIdentity)
	if err != nil || sender == nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "sender not found")
	}
	target, err := s.userRepo.GetByName(targetIdentity)
	if err != nil || target == nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "target not found")
	}
	sum, err := s.createDirect(MessageActor{Identity: senderIdentity, UserUUID: sender.UUID}, []string{target.UUID})
	if err != nil {
		return nil, err
	}
	return s.SendToConversation(MessageActor{Identity: senderIdentity, UserUUID: sender.UUID}, sum.ConversationID, content, clientNonce, nil)
}
```

同时删除 `model/conversation_participant.go` 与 `repository/conversation_repo.go` 两个文件：

```bash
git rm app/server/internal/model/conversation_participant.go app/server/internal/repository/conversation_repo.go
```

- [ ] **步骤 4：编译并运行测试验证通过**

运行：
```bash
cd app/server && go build ./... && go test ./internal/service/ ./internal/repository/ -v 2>&1 | tail -20
```
预期：编译通过，全部 PASS。若 `gin.go` 因 `NewConversationService` 签名变化编译失败，属预期——任务 13 统一修装配，但为保持每步可编译，本任务内同步修正 `server/gin.go:213` 的调用点（传入 `repository.NewConversationStore(db)` 结果）。若还有其他引用 `ConversationRepository` 的编译错误，就近修复（仅限机械替换，不改行为）。

- [ ] **步骤 5：Commit**

```bash
git add -u app/server/internal/service/conversation_service.go app/server/server/gin.go
git add app/server/internal/service/conversation_compat_test.go
git commit -m "feat(im)!: switch conversation service to unified store, drop md5 legacy" -- app/server/internal/service/conversation_service.go app/server/internal/service/conversation_participant.go app/server/internal/repository/conversation_repo.go app/server/internal/service/conversation_compat_test.go app/server/server/gin.go app/server/internal/model/conversation_participant.go
```

---

### 任务 9：MessageService conversation-scope 四操作

**文件：**
- 创建：`app/server/internal/service/message_conversation.go`
- 测试：`app/server/internal/service/message_conversation_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
package service

import (
	"testing"

	"GOSpeak/internal/model"
)

func newConvMsgFixture(t *testing.T) (*MessageService, *ConversationService, string, *gorm.DB) {
	t.Helper()
	_, convSvc, db := newConvV2Fixture(t)
	seedDomainPair(t, db)
	msgSvc := NewMessageService(repository.NewMessageRepository(db), nil, nil)
	msgSvc.SetUserRepo(repository.NewUserRepository(db))
	msgSvc.SetConversationStore(repository.NewConversationStore(db))
	alice := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	conv, err := convSvc.Create(alice, model.ConversationTypeDirect, []string{"uuid-b"})
	if err != nil {
		t.Fatalf("create conv: %v", err)
	}
	dto, err := convSvc.SendToConversation(alice, conv.ConversationID, "hello", "n1", nil)
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}
	return msgSvc, convSvc, dto.UUID, db
}

func TestEditInConversationAuthorOnly(t *testing.T) {
	msgSvc, _, msgUUID, _ := newConvMsgFixture(t)
	author := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	out, err := msgSvc.EditInConversation(msgUUID, author, "edited!")
	if err != nil || out.Content != "edited!" {
		t.Fatalf("edit: %v %+v", err, out)
	}
	other := MessageActor{Identity: "bob", UserUUID: "uuid-b"}
	if _, err := msgSvc.EditInConversation(msgUUID, other, "hijack"); err == nil {
		t.Fatal("non-author edit must be rejected")
	}
}

func TestDeleteInConversationAuthorOnly(t *testing.T) {
	msgSvc, _, msgUUID, _ := newConvMsgFixture(t)
	other := MessageActor{Identity: "bob", UserUUID: "uuid-b"}
	if err := msgSvc.DeleteInConversation(msgUUID, other); err == nil {
		t.Fatal("non-author delete must be rejected even for members")
	}
	author := MessageActor{Identity: "alice", UserUUID: "uuid-a"}
	if err := msgSvc.DeleteInConversation(msgUUID, author); err != nil {
		t.Fatalf("author delete: %v", err)
	}
}

func TestReactInConversation(t *testing.T) {
	msgSvc, _, msgUUID, _ := newConvMsgFixture(t)
	member := MessageActor{Identity: "bob", UserUUID: "uuid-b"}
	if err := msgSvc.ReactInConversation(msgUUID, member, "👍"); err != nil {
		t.Fatalf("react: %v", err)
	}
	if err := msgSvc.UnreactInConversation(msgUUID, member, "👍"); err != nil {
		t.Fatalf("unreact: %v", err)
	}
	outsider := MessageActor{Identity: "stranger", UserUUID: "uuid-x"}
	if err := msgSvc.ReactInConversation(msgUUID, outsider, "👍"); err == nil {
		t.Fatal("non-member react must be rejected")
	}
}
```

注意：`NewMessageService` 的 `roomRepo` 参数传 `nil`——conversation-scope 方法不得触碰 roomRepo（若 NewMessageService 对 nil 无守卫导致测试路径意外调用，属实现 bug，用例会 panic 暴露）。测试文件 import 需含 `"gorm.io/gorm"`、`"GOSpeak/internal/repository"`。`newConvMsgFixture` 复用任务 6 的 `newConvV2Fixture`（同包可见，任务顺序执行时必然存在）。

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run 'TestEditInConversation|TestDeleteInConversation|TestReactInConversation' -v`
预期：编译失败

- [ ] **步骤 3：实现 `message_conversation.go`**

```go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
)

// conversationScopeMessage 校验消息属于会话且 actor 是成员，返回消息与会话。
func (s *MessageService) conversationScopeMessage(messageUUID string, actor MessageActor) (*model.Message, *model.Conversation, error) {
	if s.convStore == nil {
		return nil, nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "conversation store unavailable")
	}
	msg, err := s.msgRepo.GetByUUID(messageUUID)
	if err != nil || msg.DeletedAt.Valid {
		return nil, nil, pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.ConversationType != model.ConversationTypePrivate || msg.ConversationID == nil {
		return nil, nil, pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	conv, err := s.convStore.GetByUUID(*msg.ConversationID)
	if err != nil {
		return nil, nil, pkg.NewAppError(pkg.NOT_FOUND, "conversation not found")
	}
	ok, err := s.convStore.IsMember(conv.UUID, actor.UserUUID)
	if err != nil {
		return nil, nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !ok {
		return nil, nil, pkg.NewAppError(pkg.FORBIDDEN, "not a conversation member")
	}
	return msg, conv, nil
}

// broadcastToConversation 将事件按成员逐一定向投递到 __user:{identity}。
func (s *MessageService) broadcastToConversation(convUUID, event string, payload interface{}) {
	if s.convStore == nil || s.bus == nil {
		return
	}
	uuids, err := s.convStore.GetMemberUUIDs(convUUID)
	if err != nil || s.userRepo == nil {
		return
	}
	users, _ := s.userRepo.GetByUUIDs(uuids)
	data, _ := json.Marshal(payload)
	for _, u := range uuids {
		user := users[u]
		if user == nil || user.Name == "" {
			continue
		}
		_ = s.bus.PublishRoom(context.Background(), "__user:"+user.Name, event, json.RawMessage(data))
	}
}

// EditInConversation 仅作者可编辑（DM/群聊无管理员概念）。
func (s *MessageService) EditInConversation(messageUUID string, actor MessageActor, content string) (*MessageDTO, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content is required")
	}
	if utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content too long")
	}
	msg, conv, err := s.conversationScopeMessage(messageUUID, actor)
	if err != nil {
		return nil, err
	}
	if !isMessageAuthor(msg, actor) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not your message")
	}
	now := time.Now().UTC()
	dto := &MessageDTO{
		UUID:           messageUUID,
		AuthorID:       msg.AuthorID,
		AuthorUUID:     msg.AuthorUUID,
		Content:        content,
		ReplyTo:        msg.ReplyTo,
		EditedAt:       &now,
		Deleted:        false,
		CreatedAt:      msg.CreatedAt,
		ConversationID: conv.UUID,
	}
	s.enrichAuthorDTO(dto)
	s.broadcastToConversation(conv.UUID, "message:updated", dto)

	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "edit",
		"message_uuid": messageUUID,
		"content":      content,
		"author_id":    actor.Identity,
		"author_uuid":  actor.UserUUID,
		"timestamp":    now,
	})
	if s.queue != nil {
		_ = s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      messageUUID + "-edit-" + fmt.Sprintf("%d", now.UnixNano()),
			Type:    "chat.mutate",
			Payload: payload,
		})
	} else if s.syncWriteAllowed {
		if err := s.msgRepo.UpdateContent(messageUUID, content, now); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return dto, nil
}

// DeleteInConversation 仅作者可删除。
func (s *MessageService) DeleteInConversation(messageUUID string, actor MessageActor) error {
	msg, conv, err := s.conversationScopeMessage(messageUUID, actor)
	if err != nil {
		return err
	}
	if !isMessageAuthor(msg, actor) {
		return pkg.NewAppError(pkg.FORBIDDEN, "not your message")
	}
	dto := &MessageDTO{
		UUID:           messageUUID,
		AuthorID:       msg.AuthorID,
		AuthorUUID:     msg.AuthorUUID,
		Content:        "",
		Deleted:        true,
		CreatedAt:      msg.CreatedAt,
		ConversationID: conv.UUID,
	}
	s.broadcastToConversation(conv.UUID, "message:deleted", dto)

	now := time.Now().UTC()
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "delete",
		"message_uuid": messageUUID,
		"author_id":    actor.Identity,
		"author_uuid":  actor.UserUUID,
		"timestamp":    now,
	})
	if s.queue != nil {
		_ = s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      messageUUID + "-del-" + fmt.Sprintf("%d", now.UnixNano()),
			Type:    "chat.mutate",
			Payload: payload,
		})
	} else if s.syncWriteAllowed {
		if err := s.msgRepo.SoftDelete(messageUUID); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

func (s *MessageService) ReactInConversation(messageUUID string, actor MessageActor, emoji string) error {
	return s.reactInConversation(messageUUID, actor, emoji, true)
}

func (s *MessageService) UnreactInConversation(messageUUID string, actor MessageActor, emoji string) error {
	return s.reactInConversation(messageUUID, actor, emoji, false)
}

func (s *MessageService) reactInConversation(messageUUID string, actor MessageActor, emoji string, add bool) error {
	if emoji == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "emoji is required")
	}
	_, conv, err := s.conversationScopeMessage(messageUUID, actor)
	if err != nil {
		return err
	}
	action := "removed"
	if add {
		action = "added"
	}
	s.broadcastToConversation(conv.UUID, "message:reaction", map[string]interface{}{
		"action":          action,
		"message_uuid":    messageUUID,
		"user_id":         actor.Identity,
		"emoji":           emoji,
		"conversation_id": conv.UUID,
	})

	now := time.Now().UTC()
	name := "unreact"
	if add {
		name = "react"
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       name,
		"message_uuid": messageUUID,
		"user_id":      actor.Identity,
		"emoji":        emoji,
		"timestamp":    now,
	})
	jobID := messageUUID + "-" + name + "-" + actor.Identity + "-" + emoji
	if s.queue != nil {
		_ = s.queue.Publish(context.Background(), bus.JobEnvelope{ID: jobID, Type: "chat.mutate", Payload: payload})
	} else if s.syncWriteAllowed {
		if add {
			err = s.msgRepo.AddReaction(&model.MessageReaction{MessageUUID: messageUUID, UserID: actor.Identity, Emoji: emoji})
		} else {
			err = s.msgRepo.RemoveReaction(messageUUID, actor.Identity, emoji)
		}
		if err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run 'InConversation' -v`
预期：PASS（含 Edit/Delete/React 用例与既有 room-scope 用例）

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/service/message_conversation.go app/server/internal/service/message_conversation_test.go
git commit -m "feat(im): conversation-scope message edit/delete/react" -- app/server/internal/service/message_conversation.go app/server/internal/service/message_conversation_test.go
```

---

### 任务 10：ReadStateService + Handler + 路由

**文件：**
- 创建：`app/server/internal/service/read_state_service.go`、`app/server/internal/handler/read_state_handler.go`、`app/server/internal/router/routes/readstate/routes.go`
- 修改：`app/server/internal/router/router.go`
- 测试：`app/server/internal/service/read_state_service_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
)

func TestReadStateServiceMarkConversationRequiresMember(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.Conversation{}, &model.ConversationMember{}, &model.ReadState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := repository.NewConversationStore(db)
	conv := &model.Conversation{Type: model.ConversationTypeDirect}
	_ = store.CreateConversation(conv, []model.ConversationMember{
		{ConversationUUID: conv.UUID, UserUUID: "u-1"},
		{ConversationUUID: conv.UUID, UserUUID: "u-2"},
	})
	svc := NewReadStateService(repository.NewReadStateRepository(db), store)

	if err := svc.Mark("u-1", model.ReadScopeConversation, conv.UUID, "m-1"); err != nil {
		t.Fatalf("member mark: %v", err)
	}
	if err := svc.Mark("u-x", model.ReadScopeConversation, conv.UUID, "m-1"); err == nil {
		t.Fatal("non-member mark must be rejected")
	}
	if err := svc.Mark("u-1", "bogus", conv.UUID, "m-1"); err == nil {
		t.Fatal("invalid scope must be rejected")
	}
	overview, err := svc.Overview("u-1")
	if err != nil || overview.TotalMentions != 0 {
		t.Fatalf("overview: %v %+v", err, overview)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run TestReadStateServiceMark -v`
预期：编译失败

- [ ] **步骤 3：实现**

`read_state_service.go`：

```go
package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
)

type ReadStateService struct {
	readStates *repository.ReadStateRepository
	convStore  *repository.ConversationStore
}

func NewReadStateService(readStates *repository.ReadStateRepository, convStore *repository.ConversationStore) *ReadStateService {
	return &ReadStateService{readStates: readStates, convStore: convStore}
}

// Mark 归零某 scope 的未读。conversation scope 校验成员身份；room scope 允许任意已认证用户
// （read_state 行仅影响本人徽标，room:join 时由信令层惰性初始化）。
func (s *ReadStateService) Mark(userUUID, scopeType, scopeID, lastReadMessageUUID string) error {
	if userUUID == "" || scopeID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "user and scope required")
	}
	switch scopeType {
	case model.ReadScopeConversation:
		ok, err := s.convStore.IsMember(scopeID, userUUID)
		if err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if !ok {
			return pkg.NewAppError(pkg.FORBIDDEN, "not a conversation member")
		}
	case model.ReadScopeRoom:
	default:
		return pkg.NewAppError(pkg.INVALID_PARAMS, "invalid scope_type")
	}
	if err := s.readStates.MarkRead(scopeType, scopeID, userUUID, lastReadMessageUUID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

type ReadStateSummary struct {
	ScopeType    string `json:"scope_type"`
	ScopeID      string `json:"scope_id"`
	UnreadCount  int    `json:"unread_count"`
	MentionCount int    `json:"mention_count"`
}

type ReadStateOverview struct {
	Items         []ReadStateSummary `json:"items"`
	TotalMentions int                `json:"total_mentions"`
}

// Overview 返回用户全部 scope 未读汇总与总提及数。
func (s *ReadStateService) Overview(userUUID string) (*ReadStateOverview, error) {
	rows, err := s.readStates.ListByUser(userUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	out := &ReadStateOverview{Items: make([]ReadStateSummary, 0, len(rows))}
	for _, r := range rows {
		out.Items = append(out.Items, ReadStateSummary{
			ScopeType:    r.ScopeType,
			ScopeID:      r.ScopeID,
			UnreadCount:  r.UnreadCount,
			MentionCount: r.MentionCount,
		})
		out.TotalMentions += r.MentionCount
	}
	return out, nil
}
```

`read_state_handler.go`：

```go
package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type ReadStateHandler struct {
	svc *service.ReadStateService
}

func NewReadStateHandler(svc *service.ReadStateService) *ReadStateHandler {
	return &ReadStateHandler{svc: svc}
}

func (h *ReadStateHandler) Mark(c *gin.Context) {
	var req struct {
		ScopeType           string `json:"scope_type" binding:"required"`
		ScopeID             string `json:"scope_id" binding:"required"`
		LastReadMessageUUID string `json:"last_read_message_uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userUUID, ok := h.userUUID(c)
	if !ok {
		return
	}
	if err := h.svc.Mark(userUUID, req.ScopeType, req.ScopeID, req.LastReadMessageUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *ReadStateHandler) List(c *gin.Context) {
	userUUID, ok := h.userUUID(c)
	if !ok {
		return
	}
	out, err := h.svc.Overview(userUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}

func (h *ReadStateHandler) userUUID(c *gin.Context) (string, bool) {
	v, exists := c.Get("user_uuid")
	if !exists {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return "", false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "invalid context")
		return "", false
	}
	return s, true
}
```

（`user_uuid` 为 middleware.JWTAuth 注入的 context key；实现时以 `internal/middleware/auth.go` 实际 key 为准，如为 `c.Set("user_uuid", ...)` 则照抄。）

`readstate/routes.go`：

```go
package readstate

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.ReadStateHandler) {
	r.POST("/mark", h.Mark)
	r.POST("/list", h.List)
}
```

`router.go`：仿照第 9 行 import 与第 125 行注册：

```go
	readstateRoutes "GOSpeak/internal/router/routes/readstate"
```
```go
		readstateRoutes.RegisterProtected(protected.Group("/read-state"), h.ReadState)
```

（`h.ReadState` 字段在任务 13 装配；本任务先在 `handler.Handlers` 聚合结构（若存在）加字段或直接在 router.go 以局部变量接线——以 router.go 既有 handler 聚合方式为准，保持一致。）

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run TestReadStateServiceMark -v`
预期：PASS（router.go 若因 handler 未装配而编译失败，给 router.go 加一个临时 nil-safe 注册跳过或延后到任务 13 一起编译——选择：本任务先不注册 router.go，注册动作放任务 13；测试与 handler/service 文件先落地）

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/service/read_state_service.go app/server/internal/service/read_state_service_test.go app/server/internal/handler/read_state_handler.go app/server/internal/router/routes/readstate/routes.go
git commit -m "feat(im): read-state mark/list service and endpoints" -- app/server/internal/service/read_state_service.go app/server/internal/service/read_state_service_test.go app/server/internal/handler/read_state_handler.go app/server/internal/router/routes/readstate/routes.go
```

---

### 任务 11：ConversationHandler 新端点 + 路由扩展

**文件：**
- 修改：`app/server/internal/handler/conversation_handler.go`、`app/server/internal/router/routes/conversation/routes.go`
- 测试：`app/server/internal/handler/conversation_handler_v2_test.go`（gin httptest 风格，参照 `message_handler_domain_test.go` 的既有模式）

- [ ] **步骤 1：编写失败的测试**

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestConversationCreateEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Domain{}, &model.DomainMember{},
		&model.Conversation{}, &model.ConversationMember{}, &model.ReadState{}, &model.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Create(&model.User{Name: "alice", UUID: "uuid-a"}).Error
	_ = db.Create(&model.User{Name: "bob", UUID: "uuid-b"}).Error
	_ = db.Create(&model.Domain{UUID: "dom-1", Name: "d"}).Error
	_ = db.Create(&model.DomainMember{DomainUUID: "dom-1", UserUUID: "uuid-a", RoleName: "member"}).Error
	_ = db.Create(&model.DomainMember{DomainUUID: "dom-1", UserUUID: "uuid-b", RoleName: "member"}).Error

	convSvc := service.NewConversationService(repository.NewMessageRepository(db), repository.NewConversationStore(db))
	convSvc.SetUserRepo(repository.NewUserRepository(db))
	convSvc.SetReadStates(repository.NewReadStateRepository(db))
	convSvc.SetDomainChecker(domainCheckerFromDB(db))
	h := NewConversationHandler(convSvc, nil)

	r := gin.New()
	r.POST("/create", func(c *gin.Context) {
		c.Set("username", "alice")
		c.Set("user_uuid", "uuid-a")
		h.Create(c)
	})
	body, _ := json.Marshal(map[string]interface{}{"type": "direct", "member_uuids": []string{"uuid-b"}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			ConversationID string `json:"conversation_id"`
			Type           string `json:"type"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 || resp.Data.ConversationID == "" || resp.Data.Type != "direct" {
		t.Fatalf("resp = %+v", resp)
	}
}
```

`domainCheckerFromDB(db)` 为测试内 helper（同文件定义）：

```go
type dbDomainChecker struct{ db *gorm.DB }

func (d *dbDomainChecker) HasCommonDomain(a, b string) (bool, error) {
	return repository.NewDomainRepository(d.db).HasCommonDomain(a, b)
}

func domainCheckerFromDB(db *gorm.DB) *dbDomainChecker { return &dbDomainChecker{db: db} }
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/handler/ -run TestConversationCreateEndpoint -v`
预期：编译失败（构造器签名/方法不存在）

- [ ] **步骤 3：实现**

`conversation_handler.go`：构造器增 `msgSvc *service.MessageService` 参数：

```go
type ConversationHandler struct {
	svc    *service.ConversationService
	msgSvc *service.MessageService
}

func NewConversationHandler(svc *service.ConversationService, msgSvc *service.MessageService) *ConversationHandler {
	return &ConversationHandler{svc: svc, msgSvc: msgSvc}
}
```

新增通用 actor 提取（替换四个兼容 handler 里重复的 username 提取亦可，但**不改旧 handler 行为**，仅新增）：

```go
func (h *ConversationHandler) actor(c *gin.Context) (service.MessageActor, bool) {
	username, _ := c.Get("username")
	identity, _ := username.(string)
	uuidV, _ := c.Get("user_uuid")
	userUUID, _ := uuidV.(string)
	if identity == "" || userUUID == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return service.MessageActor{}, false
	}
	return service.MessageActor{Identity: identity, UserUUID: userUUID}, true
}
```

新增 handler 方法：

```go
func (h *ConversationHandler) Create(c *gin.Context) {
	var req struct {
		Type        string   `json:"type" binding:"required"`
		MemberUUIDs []string `json:"member_uuids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.svc.Create(actor, req.Type, req.MemberUUIDs)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}

func (h *ConversationHandler) Get(c *gin.Context) {
	var req struct {
		ConversationUUID string `json:"conversation_uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.svc.Get(actor, req.ConversationUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}

func (h *ConversationHandler) Invite(c *gin.Context) {
	var req struct {
		ConversationUUID string   `json:"conversation_uuid" binding:"required"`
		MemberUUIDs      []string `json:"member_uuids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.svc.Invite(actor, req.ConversationUUID, req.MemberUUIDs)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}

func (h *ConversationHandler) Leave(c *gin.Context) {
	var req struct {
		ConversationUUID string `json:"conversation_uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if err := h.svc.Leave(actor, req.ConversationUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *ConversationHandler) Summaries(c *gin.Context) {
	var req struct {
		Limit int `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.svc.Summaries(actor, req.Limit)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}

func (h *ConversationHandler) EditMessage(c *gin.Context) {
	var req struct {
		MessageUUID string `json:"message_uuid" binding:"required"`
		Content     string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.msgSvc.EditInConversation(req.MessageUUID, actor, req.Content)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}

func (h *ConversationHandler) DeleteMessage(c *gin.Context) {
	var req struct {
		MessageUUID string `json:"message_uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if err := h.msgSvc.DeleteInConversation(req.MessageUUID, actor); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *ConversationHandler) ReactMessage(c *gin.Context) {
	var req struct {
		MessageUUID string `json:"message_uuid" binding:"required"`
		Emoji       string `json:"emoji" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if err := h.msgSvc.ReactInConversation(req.MessageUUID, actor, req.Emoji); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *ConversationHandler) UnreactMessage(c *gin.Context) {
	var req struct {
		MessageUUID string `json:"message_uuid" binding:"required"`
		Emoji       string `json:"emoji" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if err := h.msgSvc.UnreactInConversation(req.MessageUUID, actor, req.Emoji); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}
```

`routes/conversation/routes.go` 扩展为：

```go
package conversation

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.ConversationHandler) {
	// 兼容旧前端（P1 期间保持旧响应格式；P2 切换后删除）
	r.POST("/list", h.List)
	r.POST("/messages", h.Messages)
	r.POST("/mark-read", h.MarkRead)
	r.POST("/send", h.Send)

	// v2
	r.POST("/summaries", h.Summaries)
	r.POST("/create", h.Create)
	r.POST("/get", h.Get)
	r.POST("/invite", h.Invite)
	r.POST("/leave", h.Leave)
	r.POST("/messages/edit", h.EditMessage)
	r.POST("/messages/delete", h.DeleteMessage)
	r.POST("/messages/react", h.ReactMessage)
	r.POST("/messages/unreact", h.UnreactMessage)
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/handler/ -run TestConversationCreateEndpoint -v`
预期：PASS。注意 `gin.go:446` 的 `NewConversationHandler(conversationSvc)` 需同步加第二参 `msgH` 同源 `messageSvc`——为保证可编译，在本任务内同步修改 `server/gin.go:446`：

```go
	conversationH := handler.NewConversationHandler(conversationSvc, messageSvc)
```

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/handler/conversation_handler.go app/server/internal/handler/conversation_handler_v2_test.go app/server/internal/router/routes/conversation/routes.go app/server/server/gin.go
git commit -m "feat(im): conversation v2 endpoints (create/get/invite/leave/summaries + message ops)" -- app/server/internal/handler/conversation_handler.go app/server/internal/handler/conversation_handler_v2_test.go app/server/internal/router/routes/conversation/routes.go app/server/server/gin.go
```

---

### 任务 12：WS 事件与 conversation_bridge

**文件：**
- 修改：`app/server/internal/signal/events.go`、`app/server/internal/signal/hub.go`、`app/server/internal/signal/private_bridge.go`
- 创建：`app/server/internal/signal/conversation_bridge.go`
- 测试：`app/server/internal/signal/conversation_bridge_test.go`（参照 `message_bridge_test.go` 的 Hub 测试脚手架风格；若 Hub 构造过重，则退化为对 bridge 函数的直接表驱动测试 + fake ClientMessenger）

- [ ] **步骤 1：编写失败的测试**

```go
package signal

import (
	"strings"
	"testing"

	"GOSpeak/internal/pkg/message"
)

type fakeConvV2 struct {
	convSender    string
	convUUID      string
	content       string
	clientNonce   string
	mentions      []string
	retDTO        *message.DTO
	retErr        error
}

func (f *fakeConvV2) SendDirect(senderIdentity, targetIdentity, content, clientNonce string) (*message.DTO, error) {
	return f.retDTO, f.retErr
}

func (f *fakeConvV2) SendToConversation(actor message.Actor, conversationUUID, content, clientNonce string, mentions []string) (*message.DTO, error) {
	f.convUUID, f.content, f.clientNonce, f.mentions = conversationUUID, content, clientNonce, mentions
	f.retDTO = &message.DTO{UUID: "m-1", Content: content, ConversationID: conversationUUID}
	return f.retDTO, f.retErr
}

func TestOnConversationSendParsesPayload(t *testing.T) {
	h := &Hub{}
	f := &fakeConvV2{}
	h.convSvc = f
	payload := `{"conversation_uuid":"conv-1","content":"hello","client_nonce":"n1"}`
	ack, err := h.OnConversationSend(newFakeClient("alice", "uuid-a"), payload)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !strings.Contains(ack, "conv-1") {
		t.Fatalf("ack should carry conversation: %s", ack)
	}
	if f.convUUID != "conv-1" || f.content != "hello" || f.clientNonce != "n1" {
		t.Fatalf("service args: %+v", f)
	}
}
```

（`newFakeClient(identity, userUUID)` 若 `message_bridge_test.go` 已有等价 fake `ws.ClientMessenger`，复用之；否则在该测试文件内实现最小 fake——`Claims()` 返回含 `Username`/`UserUUID` 的 claims，其余方法空实现。）

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/signal/ -run TestOnConversationSend -v`
预期：编译失败

- [ ] **步骤 3：实现**

`events.go`：删除第 68 行 `EventMessageAck = "message:ack"`；消息事件区追加：

```go
	// 会话消息事件（客户端 → 服务端）
	EventConversationSend    = "conversation:send"
	EventConversationEdit    = "conversation:edit"
	EventConversationDel     = "conversation:delete"
	EventConversationReact   = "conversation:react"
	EventConversationUnreact = "conversation:unreact"

	// 会话事件（服务端推送）
	EventConversationNew         = "conversation:new"
	EventConversationCreated     = "conversation:created"
	EventConversationMemberJoin  = "conversation:member-joined"
	EventConversationMemberLeave = "conversation:member-left"
```

`hub.go`：在 `clientIdentity`（第 234 行）旁追加：

```go
func clientUserUUID(c ws.ClientMessenger) string {
	if c == nil || c.Claims() == nil {
		return ""
	}
	return c.Claims().UserUUID
}
```

（若 `Claims` 字段名非 `UserUUID`，以 `internal/pkg/jwt.go` 实际字段为准。）

在事件注册区（第 219-224 行附近）追加：

```go
	handler.HandleAck(EventConversationSend, h.OnConversationSend)
	handler.HandleAck(EventConversationEdit, h.OnConversationMessageEdit)
	handler.HandleAck(EventConversationDel, h.OnConversationMessageDelete)
	handler.HandleAck(EventConversationReact, h.OnConversationMessageReact)
	handler.HandleAck(EventConversationUnreact, h.OnConversationMessageUnreact)
```

`private_bridge.go`：`conversationSender` 接口扩展（实现方 `*service.ConversationService` 自动满足）：

```go
type conversationSender interface {
	SendDirect(senderIdentity, targetIdentity, content, clientNonce string) (*message.DTO, error)
	SendToConversation(actor message.Actor, conversationUUID, content, clientNonce string, mentions []string) (*message.DTO, error)
	EditInConversation(messageUUID string, actor message.Actor, content string) (*message.DTO, error)
	DeleteInConversation(messageUUID string, actor message.Actor) error
	ReactInConversation(messageUUID string, actor message.Actor, emoji string) error
	UnreactInConversation(messageUUID string, actor message.Actor, emoji string) error
}
```

`conversation_bridge.go`：

```go
package signal

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/pkg/message"
	"GOSpeak/internal/ws"
)

type conversationSendPayload struct {
	ConversationUUID string   `json:"conversation_uuid"`
	Content          string   `json:"content"`
	ClientNonce      string   `json:"client_nonce,omitempty"`
	Mentions         []string `json:"mentions,omitempty"`
}

// OnConversationSend 处理 conversation:send：direct/group 统一入口，不要求房间连接槽。
func (h *Hub) OnConversationSend(c ws.ClientMessenger, data string) (string, error) {
	if h.convSvc == nil {
		return marshalAck(map[string]interface{}{"error": "conversation service unavailable"})
	}
	var req conversationSendPayload
	if err := parseJSON(data, &req); err != nil || req.ConversationUUID == "" || req.Content == "" {
		return marshalAck(map[string]interface{}{"error": "conversation_uuid and content are required"})
	}
	actor := message.Actor{Identity: clientIdentity(c), UserUUID: clientUserUUID(c)}
	if actor.Identity == "" {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}
	dto, err := h.convSvc.SendToConversation(actor, req.ConversationUUID, req.Content, req.ClientNonce, req.Mentions)
	if err != nil {
		_, msg := pkg.ClientError(err)
		return marshalAck(map[string]interface{}{"error": msg})
	}
	return marshalAck(dto)
}

type conversationMessagePayload struct {
	MessageUUID string `json:"message_uuid"`
	Content     string `json:"content"`
	Emoji       string `json:"emoji"`
}

func (h *Hub) conversationActor(c ws.ClientMessenger) (message.Actor, string) {
	actor := message.Actor{Identity: clientIdentity(c), UserUUID: clientUserUUID(c)}
	if actor.Identity == "" {
		return actor, "unauthorized"
	}
	return actor, ""
}

func (h *Hub) OnConversationMessageEdit(c ws.ClientMessenger, data string) (string, error) {
	if h.convSvc == nil {
		return marshalAck(map[string]interface{}{"error": "conversation service unavailable"})
	}
	var req conversationMessagePayload
	if err := parseJSON(data, &req); err != nil || req.MessageUUID == "" || req.Content == "" {
		return marshalAck(map[string]interface{}{"error": "message_uuid and content are required"})
	}
	actor, deny := h.conversationActor(c)
	if deny != "" {
		return marshalAck(map[string]interface{}{"error": deny})
	}
	dto, err := h.convSvc.EditInConversation(req.MessageUUID, actor, req.Content)
	if err != nil {
		_, msg := pkg.ClientError(err)
		return marshalAck(map[string]interface{}{"error": msg})
	}
	return marshalAck(dto)
}

func (h *Hub) OnConversationMessageDelete(c ws.ClientMessenger, data string) (string, error) {
	if h.convSvc == nil {
		return marshalAck(map[string]interface{}{"error": "conversation service unavailable"})
	}
	var req conversationMessagePayload
	if err := parseJSON(data, &req); err != nil || req.MessageUUID == "" {
		return marshalAck(map[string]interface{}{"error": "message_uuid is required"})
	}
	actor, deny := h.conversationActor(c)
	if deny != "" {
		return marshalAck(map[string]interface{}{"error": deny})
	}
	if err := h.convSvc.DeleteInConversation(req.MessageUUID, actor); err != nil {
		_, msg := pkg.ClientError(err)
		return marshalAck(map[string]interface{}{"error": msg})
	}
	return marshalAck(map[string]interface{}{"ok": true})
}

func (h *Hub) OnConversationMessageReact(c ws.ClientMessenger, data string) (string, error) {
	return h.conversationReactBase(c, data, true)
}

func (h *Hub) OnConversationMessageUnreact(c ws.ClientMessenger, data string) (string, error) {
	return h.conversationReactBase(c, data, false)
}

func (h *Hub) conversationReactBase(c ws.ClientMessenger, data string, add bool) (string, error) {
	if h.convSvc == nil {
		return marshalAck(map[string]interface{}{"error": "conversation service unavailable"})
	}
	var req conversationMessagePayload
	if err := parseJSON(data, &req); err != nil || req.MessageUUID == "" || req.Emoji == "" {
		return marshalAck(map[string]interface{}{"error": "message_uuid and emoji are required"})
	}
	actor, deny := h.conversationActor(c)
	if deny != "" {
		return marshalAck(map[string]interface{}{"error": deny})
	}
	var err error
	if add {
		err = h.convSvc.ReactInConversation(req.MessageUUID, actor, req.Emoji)
	} else {
		err = h.convSvc.UnreactInConversation(req.MessageUUID, actor, req.Emoji)
	}
	if err != nil {
		_, msg := pkg.ClientError(err)
		return marshalAck(map[string]interface{}{"error": msg})
	}
	return marshalAck(map[string]interface{}{"ok": true})
}
```

若 `messageSender` 接口（hub.go:105 `msgSvc`）因这些事件走 `convSvc` 而无需变更，则不动它。**验证点**：`*service.ConversationService` 是否已实现全部 6 个接口方法（SendDirect 任务 8、其余任务 6/7/9）。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/signal/ -run 'TestOnConversationSend' -v && go build ./...`
预期：PASS + 全仓编译通过

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/signal/events.go app/server/internal/signal/hub.go app/server/internal/signal/private_bridge.go app/server/internal/signal/conversation_bridge.go app/server/internal/signal/conversation_bridge_test.go
git commit -m "feat(im): conversation:* websocket events and bridge" -- app/server/internal/signal/events.go app/server/internal/signal/hub.go app/server/internal/signal/private_bridge.go app/server/internal/signal/conversation_bridge.go app/server/internal/signal/conversation_bridge_test.go
```

---

### 任务 13：gin.go 装配收口 + 全量回归

**文件：**
- 修改：`app/server/server/gin.go`（readstate handler + 路由注册 + ConversationService 构造收敛）、`app/server/internal/signal/hub.go`（room:join 未读行初始化）、`app/server/internal/signal/hub_room_join.go`
- 测试：无新文件；全量回归

- [ ] **步骤 1：装配**

`gin.go` 修改点（按实际代码就近调整）：

1. `conversationSvc` 构造（原第 213 行 `service.NewConversationService(conversationRepo, messageRepo)`）改为：

```go
	conversationStore := repository.NewConversationStore(db)
	readStateRepo := repository.NewReadStateRepository(db)
	conversationSvc := service.NewConversationService(messageRepo, conversationStore)
	conversationSvc.SetUserRepo(userRepo)
	conversationSvc.SetReadStates(readStateRepo)
	conversationSvc.SetDomainChecker(domainSvc)
```

（`userRepo`/`db` 变量名以 gin.go 实际为准；`domainSvc` 若与 `commonDomainChecker` 接口不匹配，在 DomainService 上加 `HasCommonDomain(a, b string) (bool, error)` 委托 `domainRepo.HasCommonDomain`。）

2. `messageSvc` 追加：

```go
	messageSvc.SetReadStates(readStateRepo)
	messageSvc.SetConversationStore(conversationStore)
```

3. handler 聚合处增：

```go
	readStateH := handler.NewReadStateHandler(service.NewReadStateService(readStateRepo, conversationStore))
```

4. 路由注册（router.go 或 gin.go 中调用注册函数处）：

```go
	readstateRoutes.RegisterProtected(protected.Group("/read-state"), readStateH)
```

（若 handler 经结构体字段传递，则加字段 `ReadState *handler.ReadStateHandler` 并在 router.go 使用 `h.ReadState`。）

5. **room:join 未读行初始化**（规格 §4：用户加入文字频道时惰性建行）。`hub.go` 的 `HubOptions` 增可选依赖：

```go
type ReadStateInit interface {
	EnsureScopeRow(scopeType, scopeID, userUUID string) error
}
```

（`HubOptions` 结构体加字段 `ReadStateInit ReadStateInit`，`NewHubWithOptions` 里 `h.readStateInit = opts.ReadStateInit`，`Hub` 结构体加对应字段。）

`hub_room_join.go` 中文字房 join 成功的路径（双槽设计里 text 槽加入处）追加非阻塞调用：

```go
	if h.readStateInit != nil {
		_ = h.readStateInit.EnsureScopeRow(model.ReadScopeRoom, roomUUID, userUUID)
	}
```

（具体插入点：`hub_room_join.go:290-307` 双槽逻辑中 text 类型加入成功后；失败仅忽略，不影响 join。）

gin.go 装配：`HubOptions{... ReadStateInit: readStateRepo}`（`*repository.ReadStateRepository` 天然满足接口）。

- [ ] **步骤 2：全量编译 + Go 测试**

```bash
cd app/server && go build ./... && go vet ./... && go test ./... 2>&1 | tail -30
```

预期：全部 PASS。重点关注既有 `message_service_test.go`、`message_bridge_test.go`、`conversation` 相关用例无回归。

- [ ] **步骤 3：Commit**

```bash
git add app/server/server/gin.go app/server/internal/router/router.go app/server/internal/service/domain_service.go app/server/internal/signal/hub.go app/server/internal/signal/hub_room_join.go
git commit -m "feat(im): wire read-state and conversation v2 into server assembly" -- app/server/server/gin.go app/server/internal/router/router.go app/server/internal/service/domain_service.go app/server/internal/signal/hub.go app/server/internal/signal/hub_room_join.go
```

---

### 任务 14：API 集成测试

**文件：**
- 修改：`test/conversation/conversation.test.ts`
- （回归）`test/message/message.test.ts` 不改，必须保持通过

- [ ] **步骤 1：编写集成测试**（在既有 describe 内追加用例；沿用 `helpers.ts` 的 `api` / `registerUser`，域创建/加入直接调 API）

```ts
  it("creates direct and group conversations via v2 endpoints", async () => {
    const [alice, bob, carol] = await Promise.all([
      registerUser("imref"),
      registerUser("imref"),
      registerUser("imref"),
    ]);

    // 三人共域：alice 建域拿邀请码，bob/carol 加入
    const dom = await api<{ uuid: string; invite_code: string }>("/api/v1/domain/create", {
      token: alice.access_token,
      body: { name: "im-refactor-test", is_public: false },
    });
    expect(dom.code).toBe(0);
    for (const u of [bob, carol]) {
      const joined = await api("/api/v1/domain/join", {
        token: u.access_token,
        body: { invite_code: dom.data!.invite_code },
      });
      expect(joined.code).toBe(0);
    }

    // direct 幂等
    const first = await api<{ conversation_id: string; type: string }>(
      "/api/v1/conversation/create",
      { token: alice.access_token, body: { type: "direct", member_uuids: [bob.user_uuid] } },
    );
    expect(first.code).toBe(0);
    const second = await api<{ conversation_id: string }>(
      "/api/v1/conversation/create",
      { token: bob.access_token, body: { type: "direct", member_uuids: [alice.user_uuid] } },
    );
    expect(second.data!.conversation_id).toBe(first.data!.conversation_id);

    // 群聊
    const group = await api<{ conversation_id: string; members: unknown[] }>(
      "/api/v1/conversation/create",
      { token: alice.access_token, body: { type: "group", member_uuids: [bob.user_uuid, carol.user_uuid] } },
    );
    expect(group.code).toBe(0);
    expect(group.data!.members).toHaveLength(3);

    // 群消息 REST（经兼容 send 无法发群，改用 REST messages/edit 验证操作管道：先无消息可编辑——用 direct 消息验证）
    const sent = await api<{ uuid: string; conversation_id: string }>("/api/v1/conversation/send", {
      token: alice.access_token,
      body: { target_identity: bob.username, content: "compat path" },
    });
    expect(sent.code).toBe(0);
    expect(sent.data!.conversation_id).toBe(first.data!.conversation_id);

    const edited = await api<{ content: string }>("/api/v1/conversation/messages/edit", {
      token: alice.access_token,
      body: { message_uuid: sent.data!.uuid, content: "edited via v2" },
    });
    expect(edited.code).toBe(0);
    expect(edited.data!.content).toBe("edited via v2");

    // bob 未读（summaries）
    const summaries = await api<{ items: Array<{ conversation_id: string; unread_count: number }> }>(
      "/api/v1/conversation/summaries",
      { token: bob.access_token, body: {} },
    );
    const row = summaries.data!.items.find((i) => i.conversation_id === first.data!.conversation_id);
    expect(row?.unread_count).toBeGreaterThan(0);

    // mark-read 归零
    const marked = await api("/api/v1/read-state/mark", {
      token: bob.access_token,
      body: { scope_type: "conversation", scope_id: first.data!.conversation_id },
    });
    expect(marked.code).toBe(0);
    const after = await api<{ items: Array<{ conversation_id: string; unread_count: number }> }>(
      "/api/v1/conversation/summaries",
      { token: bob.access_token, body: {} },
    );
    const row2 = after.data!.items.find((i) => i.conversation_id === first.data!.conversation_id);
    expect(row2?.unread_count).toBe(0);

    // 非成员不能拉群外陌生人（stranger 未共域）
    const outsider = await registerUser("imref");
    const denied = await api("/api/v1/conversation/create", {
      token: alice.access_token,
      body: { type: "direct", member_uuids: [outsider.user_uuid] },
    });
    expect(denied.code).not.toBe(0);
  });
```

（`registerUser` 返回体字段名 `user_uuid`/`username`/`access_token` 以 `test/helpers.ts` 实际为准；若 helper 未返回 `user_uuid`，改用 `/api/v1/user/profile` 获取。`/api/v1/domain/create` 请求/响应字段以 `internal/handler` 实现为准。）

- [ ] **步骤 2：启动 server 并运行**

```bash
cd app/server && pnpm dev &   # 或复用已在运行的 8998 实例（先探测，勿重复起）
cd /Users/noelorin/GOSpeak && pnpm test:server 2>&1 | tail -30
```

预期：`test/conversation` 全部 PASS；`test/message` 保持 PASS（旧格式兼容回归）。

- [ ] **步骤 3：按 AGENTS.md 测试日志规范写 `agent_test_logs/im-p1-integration-test-2026-11-07.md`**（✅/❌ 状态 + 覆盖点清单）

- [ ] **步骤 4：Commit**

```bash
git add test/conversation/conversation.test.ts agent_test_logs/im-p1-integration-test-2026-11-07.md
git commit -m "test(im): p1 conversation v2 integration coverage" -- test/conversation/conversation.test.ts agent_test_logs/im-p1-integration-test-2026-11-07.md
```

---

### 任务 15：最终验收

- [ ] **步骤 1：全量回归**

```bash
cd app/server && go vet ./... && go test ./... 2>&1 | tail -10
cd /Users/noelorin/GOSpeak && pnpm typecheck && pnpm lint 2>&1 | tail -10
```

预期：全部通过（前端零改动 ⇒ typecheck 必须无新错误）。

- [ ] **步骤 2：手动验收（现有前端无感）**

启动 server 后用浏览器验证（复用已运行实例，勿新起）：
1. 登录 → /chat 页会话列表仍正常显示旧 direct 会话语义（新会话为空属预期——旧数据已按决策不迁移）
2. 两账号互发私聊（旧 /chat 页）→ 双方收到、未读数字正确
3. 文字频道发消息 → 消息正常（read_states 无感累积）

- [ ] **步骤 3：收尾 commit（如有遗漏文件）**

```bash
git status --short   # 确认无遗漏本计划文件；无关 WIP 不动
```

---

## 自检记录（计划完成后核对）

1. **规格覆盖度**：§1 三表 → 任务 1；§1 存量处置（删旧模型/不迁移）→ 任务 8；§2 新端点 → 任务 10/11；§2 兼容策略 → 任务 8/11（旧四端点保留旧格式）；§3 权限管道 → 任务 9（conversation scope：作者删改/成员 react）+ 任务 5/7 落库路径 GuestCaps 与域校验未动（room scope 现状保留）；§4 未读 → 任务 3/5/7/10 + 任务 13 步骤 1.5（room:join 惰性建行）；§5 WS 事件 → 任务 12（含 MESSAGE_ACK 删除）；§6 错误码 → 各服务方法内 `*AppError`；§6 测试 → 各任务 TDD + 任务 14 集成。
2. **占位符**：无「待定/TODO」；两处「以实际代码为准」的说明（context key、helpers 字段名）属对既有代码的事实核对指令，非实现空缺。
3. **类型一致性**：`SendToConversation(actor, convUUID, content, clientNonce, mentions)` 在任务 7（定义）、任务 9 测试（调用）、任务 12（接口+bridge）一致；`NewConversationService` 最终签名 `(messageRepo, store)` 在任务 8 定型、任务 13 使用一致；`ApplyRoomMessage/ApplyConversationMessage/MarkRead/EnsureScopeRow/ListByUser` 任务 3 定义、任务 5/7/8/10 使用一致。
