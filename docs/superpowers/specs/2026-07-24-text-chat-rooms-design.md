# 文字聊天房间设计（Discord 风格，扁平双类型）

> 状态：已定稿（设计评审通过，待写实现计划）  
> 日期：2026-07-24  
> 范围：第一版文字房 + 与现有语音房双槽并发

---

## 1. 目标与非目标

### 目标

- 在现有扁平 `Room` 模型上增加 **文字房**（`type=text`），与 **语音房**（`type=voice`）并列。
- 文字房支持接近 Discord 的消息能力：发送、历史、实时、编辑、删除、回复、表情反应、@mention。
- 同一用户可 **同时** 在 1 个文字房 + 1 个语音房。
- 主区 **上下分栏**，下为文字、上为语音，中间可拖拽 split。
- 消息列表 **虚拟滚动**，上滚加载历史；每页 100～200 条（按总量/配置）。
- 发送路径：**优先队列/总线广播，落库可异步滞后**；JetStream 不可用时降级同步写库。

### 非目标（明确不做）

- Server / Category 层级
- 线程（thread）侧栏（用 `reply_to` 即可）
- 消息附件 / 图片消息协议
- 置顶、全文搜索、已读回执
- @everyone / 角色提及
- 消息作者历史快照（读时 join 用户表）
- 移动端专用布局
- 独立 `TextRoom` 模型或独立 WS 通道

---

## 2. 决策摘要

| 项 | 选择 |
|----|------|
| 架构方案 | **A**：扩展现有 Room + 独立 Message 表 |
| 房间组织 | 扁平 `Room.type = text \| voice` |
| 并发 | 每连接最多 1 text + 1 voice 槽 |
| 消息能力 | 编辑 / 删除 / 回复 / 反应 / @user |
| 发送路径 | 先 `EventBus.PublishRoom`，再 `JobQueue` 异步落库 |
| 历史 | REST cursor（`before`），非 page offset |
| UI | 主区垂直 split：上语音、下文字、可拖拽 |
| 列表 | 虚拟滚动 + 顶触加载 |

否决方案：

- **B** Channel 挂在 Room 下 → 超 scope，引入伪 Server
- **C** 文字完全独立模型 → 成员/权限/列表双份，维护成本高

---

## 3. 数据模型

### 3.1 Room 扩展

```go
// model/room.go 增量字段
Type string `gorm:"size:16;not null;default:voice;index" json:"type"` // "text" | "voice"
```

规则：

- 旧数据无 Type → default `voice`，零破坏。
- **创建后不可改 type**（改类型会搞乱消息与 SFU 状态）。
- 语音房：沿用现有 SFU / 成员 / 踢人。
- 文字房：不进 SFU；仍可 `room:join` 做在线成员与实时广播。
- `room:join:sfu` 对 `type=text` **拒绝**，错误信息明确（如 `"text room has no media"`）。

### 3.2 Message

```go
type Message struct {
    ID        uint           `gorm:"primaryKey" json:"id"`
    UUID      string         `gorm:"type:uuid;uniqueIndex" json:"uuid"`
    RoomUUID  string         `gorm:"size:36;index:idx_msg_room_created,priority:1;not null" json:"room_uuid"`
    AuthorID  string         `gorm:"size:64;index;not null" json:"author_id"` // user.uuid
    Content   string         `gorm:"type:text;not null" json:"content"`
    ReplyTo   string         `gorm:"size:36;index" json:"reply_to,omitempty"`
    EditedAt  *time.Time     `json:"edited_at,omitempty"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
    CreatedAt time.Time      `gorm:"index:idx_msg_room_created,priority:2" json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
}
```

规则：

- 内容上限 **2000 rune**。
- soft delete：行保留；对客户端 `content=""` 且 `deleted: true`。
- `ReplyTo` 必须同房且目标未删除；非法 → `INVALID_PARAMS`。
- 作者展示信息读时 join `users`（`display_name` / `avatar`），不做作者快照。

### 3.3 MessageReaction

```go
type MessageReaction struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    MessageUUID string    `gorm:"size:36;uniqueIndex:idx_react_unique,priority:1;not null" json:"message_uuid"`
    UserID      string    `gorm:"size:64;uniqueIndex:idx_react_unique,priority:2;not null" json:"user_id"`
    Emoji       string    `gorm:"size:32;uniqueIndex:idx_react_unique,priority:3;not null" json:"emoji"`
    CreatedAt   time.Time `json:"created_at"`
}
```

- 唯一键 `(message_uuid, user_id, emoji)`。
- 列表默认聚合：`[{ emoji, count, me }]`；完整 users 列表可后置。

### 3.4 MessageMention

```go
type MessageMention struct {
    ID          uint   `gorm:"primaryKey" json:"id"`
    MessageUUID string `gorm:"size:36;index;not null" json:"message_uuid"`
    UserID      string `gorm:"size:64;index;not null" json:"user_id"`
}
```

- 客户端 **显式传 `mentions: string[]`**（user uuid），避免 `@display_name` 重名歧义。
- 本期只做 **@user**，不做 @everyone / 角色。

### 3.5 迁移

- GORM AutoMigrate：3 新表 + `room.type`。
- 无需手工数据迁移脚本（default voice 即可）。

---

## 4. 发送路径与实时

### 4.1 原则

**先出队广播，落库可晚。** 用户体感延迟 = 校验 + 广播，不含 DB 写。

### 4.2 发送流水线

```
Client message:send
  → Hub 校验（JWT / 在房 / type=text / 长度 / mute）
  → 分配 message.uuid + created_at（服务端）
  → ① EventBus.PublishRoom → 本机 + 跨实例 message:created
  → ② JobQueue.Publish(type=chat.persist)
  → ③ Worker 异步 INSERT message + mentions
  → （可选）message:ack { client_nonce, message_uuid }
```

| 步骤 | 同步？ | 失败行为 |
|------|--------|----------|
| 校验 + 分配 UUID | 同步 | 直接拒，无广播 |
| `PublishRoom` | 同步本地 / 尽力跨实例 | 本机必达；NATS 挂时仍本地投递（现有 bus 语义） |
| `JobQueue` 入队 | 同步入队 | JetStream 不可用 → **降级同步写 DB**，保证不丢 |
| Worker 落库 | 异步 | Nak 重试；耗尽后打日志 |

### 4.3 编辑 / 删除 / 反应

- 同样：**先广播** `message:updated` / `message:deleted` / `message:reaction`，**再入队** `chat.mutate`。
- Worker 按 `message_uuid` 写库。
- **竞态**：persist 未完成就 mutate → Worker 找不到行时 **Nak 短延迟重试**（有上限），保证最终一致。
- 删除权限：作者 **或** 持有 `message:delete_others`。
- 编辑：仅作者；置 `edited_at`；内容 ≤ 2000 rune。

### 4.4 Job 类型（复用 `bus.JobQueue`）

| Type | 动作 |
|------|------|
| `chat.persist` | INSERT message + mentions |
| `chat.mutate` | UPDATE content / soft delete / reaction upsert|delete |

Subject：`{prefix}.jobs.chat.persist` / `{prefix}.jobs.chat.mutate`。

### 4.5 客户端 nonce 与乐观 UI

- 发送带 `client_nonce`；本地先插 `pending`。
- 收到 `message:created`（含自己的 nonce）或 `message:ack` 后，用 `message_uuid` 替换临时 id 并去重。

### 4.6 与异步落库的历史缺口

- 历史 REST **只读 DB**，可能缺最近几秒未落库消息。
- 补偿：前端合并  
  `history(DB) ∪ realtimeBuffer`，按 `(created_at, uuid)` 去重排序。
- 本期不做服务端 room 环形热缓存。

---

## 5. API 与信令

### 5.1 REST

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/rooms/:uuid/messages?before=&limit=` | cursor 历史 |
| POST | `/api/v1/rooms/:uuid/messages` | 发消息（与 Socket 同 Service） |
| PATCH | `/api/v1/rooms/:uuid/messages/:msg_uuid` | 编辑 |
| DELETE | `/api/v1/rooms/:uuid/messages/:msg_uuid` | soft delete |
| POST | `/api/v1/rooms/:uuid/messages/:msg_uuid/reactions` | `{ emoji }` |
| DELETE | `/api/v1/rooms/:uuid/messages/:msg_uuid/reactions/:emoji` | 取消自己的 |

**历史查询规则：**

| 项 | 规则 |
|----|------|
| limit | 默认 **100**，clamp **50–200** |
| 排序 | DB：`created_at DESC, id DESC` 取 N；响应反转为 ASC |
| 游标 | `before` = 上一页最旧一条 uuid；首屏不传 = 最新 N 条 |
| 响应 | `{ items, has_more, next_before }` |
| 权限 | 登录 + `message:read`；**不强制**当前在房 |

创建房间：现有 create 增加 `type` 字段；list 支持 `type=text|voice|all`。

### 5.2 Socket.IO 事件

| 方向 | 事件 | 载荷要点 |
|------|------|----------|
| C→S | `message:send` | `{ room, content, reply_to?, mentions?, client_nonce? }` |
| C→S | `message:edit` | `{ room, message_uuid, content }` |
| C→S | `message:delete` | `{ room, message_uuid }` |
| C→S | `message:react` | `{ room, message_uuid, emoji }` |
| C→S | `message:unreact` | `{ room, message_uuid, emoji }` |
| S→C | `message:created` | 完整消息 DTO + `client_nonce?` |
| S→C | `message:updated` | `{ uuid, content, edited_at }` |
| S→C | `message:deleted` | `{ uuid, deleted: true }` |
| S→C | `message:reaction` | `{ message_uuid, emoji, user_id, op: add\|remove }` |
| S→C | `message:ack` | `{ client_nonce, message_uuid }`（可选） |
| S→C | `message:error` | `{ client_nonce?, code, msg }` |

REST 与 Socket **共用 MessageService**。

### 5.3 权限码（新增）

```
message:send
message:read
message:delete_others
```

- 进文字房：现有 `room:join` 成员校验。
- 文字房拒绝 `room:join:sfu`。
- 默认角色赋权在实现计划中按现有 RBAC 种子策略补齐。

### 5.4 错误

| 场景 | 处理 |
|------|------|
| 空内容 / 超长 / 坏 reply / 坏 emoji | `INVALID_PARAMS` (2001) |
| 无发送权 / 删他人无权 | FORBIDDEN |
| 消息不存在 | NOT_FOUND (3001) |
| 文字房调 SFU | FORBIDDEN + 明确文案 |
| 被 mute | 沿用现有 mute 语义 |
| Socket 侧错误 | `message:error`，不走 HTTP |

---

## 6. 双房状态机

### 6.1 槽

```
textSlot:  { roomUUID, phase, members } | null
voiceSlot: { roomUUID, phase, sfuClient, members } | null
```

| 动作 | 规则 |
|------|------|
| 加入文字房 | 已有文字 → leave 旧文字 → join 新文字；**不碰**语音 |
| 加入语音房 | 已有语音 → leave 旧语音（含 SFU）→ join 新语音；**不碰**文字 |
| 离开文字 / 语音 | 只清对应槽 |
| 同类型点当前房 | no-op 或 focus |
| 断线重连 | 两槽各自恢复；文字只 `room:join`，语音 `room:join` + `room:join:sfu` |

Hub：conn 上下文记 `textRoom` / `voiceRoom`；join 按目标 `Room.Type` 选槽，同类型替换。

### 6.2 前端 Store

- **`chatStore`（新）**：textSlot、消息缓冲、游标、`has_more`、nonce 去重。
- **`socketStore` / `voiceChatStore`**：语音槽；从单 `currentRoom` 演进为 voice 专用（可保留 getter 兼容）。

房间列表：可按 type 过滤 / tab；文字 `#`、语音现有样式；可同时高亮一个文字 + 一个语音。

---

## 7. UI

### 7.1 布局

现状：左右 `cui-solid Split`（侧栏 | 主区）。主区内再加 **垂直** Split：

```
┌── Sidebar │ 主区 ──────────────────────┐
│  房间列表  │  ┌─ 上：语音区 ──────────┐ │
│  # gen ✓  │  │  VoiceChat + 成员     │ │
│  🔊 lou ✓ │  ├──── drag handle ──────┤ │
│           │  │  下：文字区           │ │
│           │  │  虚拟列表 + 输入      │ │
│  [UserBar]│  └───────────────────────┘ │
└───────────┴────────────────────────────┘
```

规则：

- **下 = 文字**（固定约定）。
- 高度存 `localStorage.splitHeight`（类比现有 `splitWidth`）。
- 仅语音：上拉满，下可收起为「选文字房」占位。
- 仅文字：下拉满，上收起为薄条「未连接语音」。
- 双房默认上:下 ≈ **40:60**。
- 最小高度：上 ≥ ~120px，下 ≥ ~200px。
- 路由：主区常驻双槽面板；不因 `/channel` 互斥藏掉另一房。现有 `isChannel()` 演进为「任一槽 active 则显示双槽面板」。

### 7.2 文字区组件

路径建议：`components/chat/text/` 或 `components/textRoom/`，**不要**与现有 `micControl` / `speakerControl` 混名。

| 组件 | 职责 |
|------|------|
| `TextRoomPanel` | 顶栏 + 列表 + 输入 |
| `MessageList` | 虚拟滚动；顶触 `before` 加载；底跟滚 |
| `MessageItem` | 头像/名/时间/内容/edited；悬停：回复/编辑/删除/反应 |
| `MessageInput` | 多行；Enter 发、Shift+Enter 换行；reply 预览；@ 候选 |
| `ReactionBar` | emoji chip，点 = toggle 自己的 |

虚拟滚动：

- 优先 `@tanstack/solid-virtual`。
- 估算行高 + 动态 measure；顶 50px 内触发 load-more。
- limit 默认 100，可配 50–200（本地存储）。
- 仅当用户已在底部时新消息贴底。
- 本地缓冲上限约 **1000** 条，超出剪最旧（`has_more` 仍可再拉）。

### 7.3 列表合并

```
display = merge(historyPages, realtimeBuffer)
  .dedupeBy(uuid)
  .sort(created_at, uuid)
```

---

## 8. 后端分层与文件边界

```
model/message.go
model/message_reaction.go
model/message_mention.go
repository/message_repo.go
service/message_service.go
handler/message_handler.go
router/routes/message/routes.go
signal: message handlers + events.go 常量
bus.JobQueue worker: chat.persist / chat.mutate
gin.go DI + router 注册
permcode: message:send / read / delete_others
```

约束（项目既有）：

- 分层：`model → repository → service → handler → router`，只向下调用。
- Service 返回 `*pkg.AppError`；Handler 用 `pkg.HandleError`。
- 响应 `{ code, msg, data }`；错误时 `data: null`。
- 代码不加无必要注释、不用 emoji（文档可用）。

---

## 9. 测试边界

### 必测（后端优先）

1. Service 发消息 → 广播被调 + job 入队（mock bus/queue）。
2. JetStream 不可用 → 同步落库仍成功。
3. Repo `ListBefore` 游标、`has_more`、limit clamp。
4. 编辑/删除权限：作者 OK；他人无 `delete_others` 拒。
5. 反应唯一约束 + toggle。
6. Hub：text 拒绝 `room:join:sfu`；双槽同类型替换。
7. Worker：mutate 早于 persist → 重试后成功。

### 前端（有则写）

1. `merge(history, realtime)` 去重排序。
2. nonce → uuid 升格。
3. load-more 触发条件（store 单测）。

### 不做本期

- 多浏览器 E2E 压测。
- 跨实例 NATS 全链路（bus mock 即可）。

---

## 10. 实现切片（供后续 plan 拆任务）

1. Room.Type + 迁移 + 列表过滤  
2. Message 模型 / Repo / Service + REST 历史  
3. 发送路径：广播优先 + JobQueue + Worker  
4. Socket `message:*` + 双槽 join  
5. 编辑 / 删除 / 反应 / @  
6. 前端 chatStore + 虚拟列表 + 输入  
7. 主区上下 split 接双槽  

---

## 11. 与现状的关系

- `docs/project-gaps.md` 将文本聊天标为最大缺口；本设计填该缺口的第一版。
- 现有 `bot:message` **不替代** 通用聊天：无持久化、无前端监听、语义为 bot。本期新 `message:*`；bot 可后续复用 MessageService 或桥接。
- 现有 `components/chat/*` 是音量控件，**不复用文件名语义**。
