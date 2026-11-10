# 站内通知（系统公告）与定向推送能力 设计规格

日期：2026-11-10
状态：已批准（用户逐项确认：纯 WS 传输、公告落库+精确已读、全站+域级双 scope、Markdown 正文、管理后台新页发布、点开单条才标已读、v1 只做 badge + toast）

## 背景与动机

当前服务端到客户端的推送只有两条路径：

1. **房间级广播**：`publishRoom`，只投给加入该房间的连接。
2. **全局广播**：`publishNamespace`，投给所有在线连接。

「对某个具体用户的通知」没有一等能力。唯一近似物是私聊用的伪房间 `__user:{identity}`（`app/server/internal/signal/hub.go:293-297`），它是 fanout 的房间名，不是总线概念；`private:new` 走它投递（`app/server/internal/service/conversation_service.go:232-241`）。

系统公告这类需求需要三件事：**定向推送能力**、**落库的历史**、**未读/已读状态**。三者在仓库中都不存在。本规格先建一条可复用的定向推送能力 + 公告中心，后续 @提及、踢房、禁言等业务通知按同一套接口接入，不重做传输层。

### 传输层选型：复用 WebSocket，不新增 SSE

原需求描述为「前端通过 SSE 订阅」，调研后改为复用现有 `/ws`。理由：

- 现有 `/ws` 已具备完整生命周期：cookie 鉴权（`app/server/internal/ws/upgrader.go:49-59`）、指数退避重连（`app/web/src/socket/wsClient.ts:205-225`）、`tabLock` 单标签页保活（`app/web/src/stores/socketStore.ts:299-310`）、桌面回环代理（`app/desktop/proxyserver.go:116-121`）。
- SSE 只能靠 cookie 鉴权（`app/server/internal/middleware/auth.go:164-181`），且 `tabLock` 在 `connect()` 之前就 claim，双通道会与「单连接」假设冲突，还要自行实现重连/去重。
- 定向推送所需的 `__user:` 房间机制已经在线运行，SSE 订阅者无法注册进 `ws.Broadcaster`（接口只接受 `*ws.Client`，`app/server/internal/ws/broadcaster.go:7`），`NATSBus` 又只有一个 deliverer 槽位（`app/server/internal/bus/nats_bus.go:43`）——走 SSE 必须改总线结构，走 WS 则零改动。

结论：**不新增 SSE 端点**。前端订阅 = 在现有 WS 事件绑定器上注册新事件。

## 已拍板的关键决策

| 决策点 | 结论 |
|--------|------|
| 传输 | 复用 `/ws`，不新增 SSE |
| 通知类型 v1 | 仅系统公告（平台 + 域级） |
| 持久化 | 公告落库 + 精确已读表 |
| 已读粒度 | 精确已读表（`announcement_reads`），非 `last_read_at` 游标 |
| 内容格式 | Markdown 正文 |
| 受众 | 平台公告（全员）+ 域级公告（域成员） |
| 发布入口 | 管理后台新页 `/manage/announcements`；域级公告同页选择 scope |
| 已读时机 | 点开单条才标已读，badge 逐条递减 |
| 提醒方式 v1 | 侧栏 badge + 新公告 toast；**不做系统通知** |

## §1 定向推送能力

新增 `app/server/internal/notify/`，只做一件事：把「发给某个用户」翻译成现有的房间投递。

```go
package notify

// NotifyUser 向单个用户的全部在线连接推送。用户离线时静默丢弃，不补投。
func NotifyUser(ctx context.Context, bus EventBus, userUUID, event string, payload any) error

// NotifyAll 向所有在线连接推送。
func NotifyAll(ctx context.Context, bus EventBus, event string, payload any) error
```

- `NotifyUser` 内部调 `bus.PublishRoom(ctx, "__user:"+userUUID, event, payload)`。
- 伪房间的成员关系由 `Hub.OnConnect` 建立、`hub_disconnect.go:31` 清理，本包不碰。
- **`payload` 必须传 struct / map / `json.RawMessage`，禁止传 `[]byte`。** 现有 `private:new` 传的是 `[]byte`（`conversation_service.go:235`），`Fanout.BroadcastToRoom` 再 marshal 一次后变成 base64 字符串（`app/server/internal/ws/fanout.go:106`），前端拿到的是字符串而非对象。这是既有缺陷，不在本规格修复范围，但新代码不能重犯。

`EventBus` 以最小接口注入，避免 `notify` 依赖整个 `bus` 包：

```go
type EventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
}
```

**能力边界（写进包注释）**：定向推送仅对在线连接可达。没有 user subject、没有 ack/offset、WAL 只覆盖「发布时 NATS 断线」且默认关闭（`app/server/internal/bus/nats_publish.go:56-105`）。因此任何「必须送达」的通知必须自带持久化 inbox —— 公告中心正是这样做的。

## §2 数据模型

### 表 `announcements`

| 字段 | 类型 / 约束 | 说明 |
|------|------------|------|
| `ID` | uint PK | |
| `UUID` | uuid, uniqueIndex | 对外标识 |
| `ScopeType` | size:16, index | `platform` \| `domain` |
| `ScopeID` | size:32, index | 域 UUID；`platform` 时为空 |
| `Title` | size:200, not null | |
| `Content` | text, not null | Markdown 源文 |
| `Level` | size:16, default `info` | `info` \| `warning` \| `critical` |
| `Pinned` | bool, default false | 置顶排序 |
| `PublishedAt` | *time.Time, index | `nil` = 草稿，不投递 |
| `ExpiresAt` | *time.Time | `nil` = 永久；过期后不再出现在列表 |
| `CreatedByUUID` | size:36, index | 发布者 |
| `CreatedByName` | size:64 | 冗余，列表直接展示，免 join |
| `DeletedAt` | gorm.DeletedAt | 软删除 |
| `CreatedAt` / `UpdatedAt` | time | |

复合索引：`idx_announcement_visible (scope_type, scope_id, published_at)`。

### 表 `announcement_reads`

| 字段 | 类型 / 约束 |
|------|------------|
| `ID` | uint PK |
| `AnnouncementUUID` | size:36, `uniqueIndex:idx_announcement_user` |
| `UserUUID` | size:36, `uniqueIndex:idx_announcement_user` |
| `ReadAt` | time |

未读数 = 可见公告数 − 该用户已读行数。公告硬删除时级联删 reads；软删除的公告不计入可见集合，其 reads 行保留（量级可忽略）。

`BeforeCreate` 补 UUID、`Level` 默认值，风格对齐 `app/server/internal/model/message.go:38-45`。

### 可见性规则

```
platform 公告:  published_at IS NOT NULL
               AND (expires_at IS NULL OR expires_at > now)
domain 公告:    同上 AND scope_id IN (用户所属域集合)
```

域成员集合查 `domain_members`（`app/server/internal/repository/domain_repo.go:162` 已有 `ListUserDomains`）。

## §3 权限码

新增两个平台权限码（`app/server/internal/permcode/permcode.go`）：

- `announcement:read` —— 查看公告列表与未读数
- `announcement:manage` —— 创建 / 编辑 / 删除公告

同步改动点（缺一处会导致权限校验失效或前端入口不显示）：

1. `app/server/internal/permcode/permcode.go` —— 常量
2. `app/server/internal/model/permission.go` —— `PermAnnouncementXxx` 别名 + `DefaultPermissions` 种子项
3. `app/server/internal/model/permission.go:114` —— `DefaultRolePermissions["admin"]` 追加两项；`"user"` **只加 `announcement:read`**（普通用户不该看到管理入口）
4. `app/web/src/utils/permissions.ts` —— 前端兜底表 `rolePermissions` + `MANAGE_ENTRY_PERMISSIONS` 追加 `announcement:manage`
5. `app/server/internal/model/permission_test.go` —— 补断言

域级公告的发布权限**不新增权限码**，复用现有 `domain:manage`（`permcode.PermDomainManage`），判定走 `DomainService.HasDomainPermission`（`app/server/server/gin.go:223` 已注入为 `DomainPermissionChecker`）。

## §4 API

全部 POST、统一 `/api/v1/announcement/` 前缀、走 `protected` 组（`app/server/internal/router/router.go:106-109`）。访客不进白名单 → 访客天然不可见公告（`app/server/internal/middleware/guest.go:135-145`）。

| 端点 | 权限 | 说明 |
|------|------|------|
| `POST /announcement/list` | JWT | 分页返回当前用户可见公告，每项带 `is_read`；`scope_type` / `only_unread` / `page` / `page_size` 可选 |
| `POST /announcement/unread-count` | JWT | 返回 `{count}`，重连兜底与 badge 初值 |
| `POST /announcement/mark-read` | JWT | body `{uuid}`；幂等（重复调用不报错、不重复写）；公告不可见时按 `NOT_FOUND` 拒绝 |
| `POST /announcement/admin/list` | `announcement:manage` 见下 | 管理端列表，含草稿与已过期 |
| `POST /announcement/admin/create` | `announcement:manage` 见下 | body 含 `scope_type` / `scope_id` / `title` / `content` / `level` / `pinned` / `publish` / `expires_at` |
| `POST /announcement/admin/update` | `announcement:manage` 见下 | |
| `POST /announcement/admin/delete` | `announcement:manage` 见下 | 软删除 |

### 管理端权限判定（按 scope 分叉，避免两套权限互相放大）

管理端四个端点权限**按操作对象的 scope 判定**，不是一律要求 `announcement:manage`：

| 目标 scope | 需要的权限 |
|-----------|-----------|
| `platform` | 平台 `announcement:manage` |
| `domain` | 该域 `domain:manage`（`DomainService.HasDomainPermission`）**或**平台 `announcement:manage` |

`admin/list` 的可见范围随之收窄：平台管理员看全部；仅有域管理权的用户只能看到自己可管理域的公告 + 自己的草稿。判定逻辑集中在 service 层一个 `canManage(userUUID, scopeType, scopeID) bool`，handler 只调用它，避免四个端点各写一份。


响应统一走 `pkg.Success` / `pkg.Fail`（`app/server/internal/pkg/response.go`）。

DTO：

```go
type AnnouncementDTO struct {
	UUID          string `json:"uuid"`
	ScopeType     string `json:"scope_type"`
	ScopeID       string `json:"scope_id,omitempty"`
	ScopeName     string `json:"scope_name,omitempty"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	Level         string `json:"level"`
	Pinned        bool   `json:"pinned"`
	PublishedAt   *int64 `json:"published_at,omitempty"`
	ExpiresAt     *int64 `json:"expires_at,omitempty"`
	CreatedByName string `json:"created_by_name"`
	IsRead        bool   `json:"is_read"`
	CreatedAt     int64  `json:"created_at"`
}
```

### 发布时的投递

发布成功（`PublishedAt` 从 nil 变为有值）后：

- `scope_type == platform` → `notify.NotifyAll(ctx, bus, "announcement:new", dto)`。全员可见，namespace 广播无泄露面。
- `scope_type == domain` → 查域成员 UUID 列表，逐个 `notify.NotifyUser`。**不走 namespace**，否则域外用户会收到不属于自己的公告内容。

投递失败只记日志，不影响发布结果 —— 公告已落库，前端重连后靠 REST 兜底拿到。

## §5 前端

### 5.1 事件常量与绑定

`app/web/src/socket/events.ts` 追加：

```ts
ANNOUNCEMENT_NEW: "announcement:new",
```

`app/web/src/socket/socketEvents.ts` 的 `bindServerEvents` 内追加一个 `adapter.onServerEvent(EVENTS.ANNOUNCEMENT_NEW, ...)`，通过新增的 deps 回调转给 store（与 `setPrivateNewHandler` / `setMessageCreatedHandler` 同构，规避 store 间循环 import，见 `app/web/src/stores/socketStore.ts:25-37`）。

### 5.2 `notificationStore`

新增 `app/web/src/stores/notificationStore.ts`（zustand）：

```ts
interface NotificationState {
  unreadCount: number;
  items: AnnouncementDTO[];
  loading: boolean;
  loadUnreadCount(): Promise<void>;
  loadList(opts?): Promise<void>;
  markRead(uuid: string): Promise<void>;
  handleIncoming(dto: AnnouncementDTO): void;
}
```

- `handleIncoming`：`unreadCount + 1`、插入列表头、`toast.info(dto.title)`。
- `markRead`：先本地 `unreadCount - 1`、该项 `is_read = true`，再 `POST /announcement/mark-read`；失败回滚并 `toastActionError`。
- 门面导出 `notificationStore`，风格对齐 `app/web/src/stores/socketStore.ts:642-675`。

**重连兜底（必须实现）**：`wsClient` 重连不重放事件（`app/web/src/socket/wsClient.ts:205-225`），断线期间的 `announcement:new` 永久丢失。`socketStore` 当前**未对外暴露 `onConnected`**（门面导出列表 `app/web/src/stores/socketStore.ts:642-675` 无此项），需新增转发 `onConnected(cb)`，在重连成功时调 `notificationStore.loadUnreadCount()`。

### 5.3 Badge 组件

仓库**没有** Badge 组件，现有两处是复制粘贴的 inline span：`app/web/src/components/chat/roomItem.tsx:256-263`、`.../conversationList.tsx:65-69`，类名 `bg-primary text-primary-foreground rounded-full px-1.5 text-2xs`。

新增 `packages/components/src/components/badge.tsx`（`Badge`，支持 `count` / `max=99` / `dot` 模式），在 `packages/components/src/index.ts` 导出，并把上述两处迁移过去，避免出现第三份复制。

### 5.4 侧栏铃铛入口

`app/web/src/layouts/common/sidebar.tsx`：在「分享链接」与「设置」之间插入一个 `OptionSquare`，`label="通知"`，`<Bell size={20} />`，`onClick` 打开通知弹窗。

`OptionSquare` 当前无 badge 插槽（`app/web/src/components/common/optionSquare.tsx:4-13`），需加可选 prop `badge?: number | boolean`，在 `<button>` 内右上角渲染 `<Badge>`（定位范式参考 `app/web/src/components/room/components/memberItemButton.tsx:29` 的 `absolute -right-0.5 -bottom-0.5`）。

弹窗状态由 `layout.tsx` 持有（与 `SettingModal` 单例同层，`app/web/src/layouts/layout.tsx:513-516`），Sidebar 通过 prop 回调打开。

### 5.5 通知弹窗

新增 `app/web/src/components/modal/notificationModal.tsx`，用 `@gospeak/components` 的 Radix Dialog（`packages/components/src/components/dialog.tsx`），写法对齐 `app/web/src/components/modal/createRoomModal.tsx`。

布局：左列公告列表（未读圆点 + 标题 + 级别色条 + 时间），右侧详情区渲染 Markdown。选中某条时触发 `markRead`。

Markdown 渲染：`marked` + `dompurify` 均已在 `app/web/package.json` 依赖中（`marked ^18.1.3`、`dompurify ^3.4.17`），无需新增包。`marked.parse` 后必须过 `DOMPurify.sanitize`，禁止直接 `dangerouslySetInnerHTML`。

级别配色走主题 token：`info` → `text-info`，`warning` → `text-warning`，`critical` → `text-destructive`。

空态：「暂无通知」（对齐截图里聊天页的「暂无会话」样式）。

### 5.6 管理页

新增 `app/web/src/pages/(app)/manage/announcements/index.tsx`：

- 列表：标题 / scope / 级别 / 状态（草稿·已发布·已过期）/ 发布时间 / 操作
- 新建与编辑：标题、scope 选择（平台 / 指定域，域列表来自 `getMyDomainsDetailed`）、级别、置顶、正文 textarea + 实时 Markdown 预览、发布与到期时间
- 路由注册：`app/web/src/components/manage/manageNav.tsx` 的 `ManagePath` 联合类型与 `MANAGE_TABS` 追加一项，图标 `Megaphone`，`permissions: ["announcement:manage", "domain:manage"]`（域管理员也能进，页面内 scope 选择器只列出其可管理的域；平台公告的新建按钮仅 `announcement:manage` 持有者可见）

## §6 明确不做（v1）

| 不做 | 原因 |
|------|------|
| 系统通知横幅（`packages/desktop` 的 `showNotification`） | 浏览器降级路径直接 `new Notification` 不查 permission、无 try/catch（`packages/desktop/src/capabilities/notification.ts:40-53`），且全仓零调用方。接入需先补保护与权限申请 UI，独立改动 |
| 多标签页 badge 同步 | 非活跃标签页根本没有 WS 连接（`tabLock` 在 connect 前就拦，`app/web/src/stores/socketStore.ts:299-310`）；BroadcastChannel 协议只有 4 种消息（`app/web/src/socket/tabLock.ts:2-6`），扩展是独立改动 |
| @提及 / DM / 踢房 / 禁言接入通知中心 | 定向推送能力已具备，但各业务的事件生产是后续独立迭代 |
| 「必须确认」型公告 | 需要额外的确认状态机与登录拦截，v1 不做 |
| 公告附件、定时发布、富文本编辑器 | YAGNI |
| 公告已读的跨端实时同步 | 已读状态在下次 `list` / `unread-count` 时收敛即可 |

## §7 风险与处置

| 风险 | 处置 |
|------|------|
| `__user:` 投递的 payload 被二次 marshal 成 base64 | `notify` 签名收 `any`，包注释明确禁止 `[]byte`；单测断言前端可解析的对象形状 |
| 重连丢事件导致 badge 少算 | `socketStore` 暴露 `onConnected`，重连后 `loadUnreadCount()` 兜底 |
| 域公告误投给域外用户 | 域公告只走 `NotifyUser` 逐成员投递，不走 `NotifyAll` |
| `announcement_reads` 随 公告×用户 膨胀 | 公告硬删除级联删 reads；查询走 `idx_announcement_user` 联合唯一索引；软删除公告不计入可见集合 |
| 域公告可见性 join 慢 | 公告表低频写入、量级小，先直查；实测慢再加域成员缓存 |
| 访客访问公告 403 | 预期行为，访客不在此功能受众内 |
| Markdown XSS | 强制 `DOMPurify.sanitize`，单测覆盖 `<script>` / `onerror` 注入 |

## §8 测试

**Go 单测**

- `service/announcement_service_test.go`：可见性过滤（平台/域/过期/草稿）、未读数计算、`mark-read` 幂等
- `notify/notify_test.go`：`memDeliverer` 替身（范式见 `app/server/internal/bus/nats_bus_test.go:15-33`）断言 `NotifyUser` 落到 `__user:{uuid}` 房间、`NotifyAll` 落到 namespace；断言 payload 是 struct 而非 `[]byte`
- `model/permission_test.go`：追加 `announcement:read` / `announcement:manage` 断言

**Go 集成测试**（`test/announcement/`，骨架参考 `test/helpers/api.ts` + `test/helpers/ws.ts`）

1. admin 发布平台公告 → 普通用户 `list` 可见 → `unread-count` = 1 → `mark-read` → = 0
2. 普通用户调 `admin/create` → 403
3. 域管理员发布域公告 → 域内成员可见、域外成员不可见
4. 草稿公告不出现在普通用户列表
5. 在线用户 `waitForEvent("announcement:new")`（`test/helpers/ws.ts:139-155`）收到 payload 且 `title` 字段可读（回归 base64 陷阱）

**前端 vitest**

- `notificationStore.test.ts`：WS 增量、`markRead` 乐观更新与失败回滚、重连兜底调用
- `notificationModal.test.tsx`：列表渲染、点击触发 `markRead`、Markdown 清洗（注入 `<img onerror>` 应被剥离）
- `badge.test.tsx`：`count=0` 不渲染、`count>99` 显示 `99+`、`dot` 模式

## §9 落地清单

| 层 | 文件 |
|----|------|
| 能力 | `app/server/internal/notify/notify.go`（新） |
| 模型 | `app/server/internal/model/announcement.go`（新，含 `Announcement` + `AnnouncementRead`） |
| 仓库 | `app/server/internal/repository/announcement_repo.go`（新） |
| 服务 | `app/server/internal/service/announcement_service.go`（新） |
| 处理器 | `app/server/internal/handler/announcement_handler.go`（新） |
| 路由 | `app/server/internal/router/routes/announcement/routes.go`（新）+ `app/server/internal/router/router.go`（改） |
| 装配 | `app/server/server/gin.go`（handler 构造 + `Handlers` 字面量） |
| 权限 | `app/server/internal/permcode/permcode.go`、`app/server/internal/model/permission.go`（改） |
| 迁移 | `app/server/internal/repository/db_migrations.go` 的 `autoMigrate()` 列表末尾追加 `&model.Announcement{}`、`&model.AnnouncementRead{}` |
| 前端 store | `app/web/src/stores/notificationStore.ts`（新） |
| 前端事件 | `app/web/src/socket/events.ts`、`app/web/src/socket/socketEvents.ts`、`app/web/src/stores/socketStore.ts`（改，含 `onConnected` 转发） |
| 前端 UI | `packages/components/src/components/badge.tsx`（新）、`app/web/src/components/modal/notificationModal.tsx`（新）、`app/web/src/layouts/common/sidebar.tsx`、`app/web/src/components/common/optionSquare.tsx`、`app/web/src/layouts/layout.tsx`（改） |
| 前端管理页 | `app/web/src/pages/(app)/manage/announcements/index.tsx`（新）、`app/web/src/components/manage/manageNav.tsx`（改） |
| 前端 API | `app/web/src/api/announcement.ts`（新）、`app/web/src/protocol/announcement.ts`（新，DTO） |
| 权限前端 | `app/web/src/utils/permissions.ts`（改） |
| 测试 | 上述各层对应测试文件 + `test/announcement/` |
