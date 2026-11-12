# 架构

## 整体架构

```
┌─────────────┐
│   浏览器     │  或桌面客户端
└──────┬──────┘
       │
       ├──── HTTPS ─────► Go 应用 :8998 ──► 数据库
       │                    │
       ├──── WebSocket ─────┘  （信令：成员进出、发言状态、消息）
       │                    │
       │                    └── /livekit ──► LiveKit :7880（同源反代）
       │
       └──── WebRTC ──────► LiveKit :7882  （音频，UDP 直连）
```

系统由三个相互独立的部分组成：

- **Go 应用**：承载业务逻辑、权限控制与数据存储，并提供 WebSocket 信令服务
- **LiveKit**：只负责音频转发，不涉及业务逻辑
- **数据库**：SQLite / PostgreSQL / MySQL

**音频不经过 Go 应用。** 这是整个设计中最重要的一点：应用不会因音频流量而过载，两者可独立扩容。

## 拆分设计的原理

如果音频流量也经过应用转发：

- 一个 20 人的房间中，每人每次发言都需要复制 19 份转发，应用将迅速过载
- 音频卡顿会影响所有 HTTP 请求的响应速度
- 扩容时必须将业务与媒体一并扩展

拆分之后，应用仅需承担三项职责：签发令牌、告知前端连接地址、在成员进出时更新状态。

## 后端分层

```
请求
 │
 ├─ Router           路由匹配
 ├─ Middleware       验证令牌、检查权限、限流
 ├─ Handler          解析参数、调用 service、返回 JSON
 ├─ Service          业务逻辑，唯一可返回 AppError 的层
 ├─ Repository       数据访问
 └─ DB
```

分层遵循以下固定规则：

- 每层只能调用相邻的下一层，不允许跨层调用
- Service 层返回 `*AppError`，由 Handler 层决定如何转换为响应
- Handler 层不包含业务逻辑，Repository 层不包含业务规则

这种设计使局部修改不会波及其他部分。

## 目录结构

```
app/
├── server/                  Go 后端
│   ├── main.go              入口（cobra）
│   ├── server/gin.go        依赖注入，组装所有层
│   └── internal/
│       ├── config/          环境变量
│       ├── model/           GORM 实体
│       ├── repository/      数据访问
│       ├── service/         业务逻辑
│       ├── handler/         HTTP 控制器
│       ├── middleware/      认证、权限、限流
│       ├── router/          路由注册（按模块分组）
│       ├── signal/          WebSocket 信令
│       ├── sfu/             语音服务抽象层
│       ├── storage/         对象存储抽象
│       ├── authstate/       令牌黑名单、密钥轮换
│       └── pkg/             错误、响应、JWT、OAuth
├── web/                     React 19 前端
│   └── src/
│       ├── pages/           文件路由，自动生成 routeTree
│       ├── api/             HTTP 客户端
│       ├── socket/          WebSocket 客户端
│       ├── stores/          zustand 状态
│       ├── components/      业务组件
│       ├── layouts/         布局
│       └── hooks/           React hooks
├── desktop/                 桌面端宿主（Go + MyGo）
└── docs/                    文档站

packages/
├── components/              UI 组件库（Radix + Tailwind）
├── sfu-client/              前端语音客户端抽象
└── desktop/                 桌面能力 SDK
```

## 语音抽象层

代码中通过 `sfu.Provider` 接口抽象语音服务的能力：

```go
type Provider interface {
    GenerateToken(room, identity string) (string, error)
    ListRooms() ([]RoomSummary, error)
    ListParticipants(room string) ([]ParticipantSummary, error)
    MuteParticipant(room, identity, trackSid string, muted bool) error
    RemoveParticipant(room, identity string) error
    DeleteRoom(room string) error
    GetHost() string
}
```

当前仅有 LiveKit 一个实现。接口的意义在于：业务层不依赖具体的底层实现，将来接入其他服务时无需修改业务代码。

`sfu.NewDynamicProvider()` 在运行时解析当前使用的实现，支持在管理后台切换而无需重启。

## 数据模型

应用启动时，GORM 自动建表并新增字段，无需手动执行 SQL。

### 用户与权限

| 表 | 存储内容 |
|----|--------|
| `users` | 用户账号，含角色、令牌版本、是否访客 |
| `roles` | 角色（`admin` / `user` / `ban`） |
| `permissions` | 权限码定义 |
| `role_permissions` | 角色和权限的关联 |
| `user_groups` | 用户组 |

### 域与房间

| 表 | 存储内容 |
|----|--------|
| `domains` | 语音域，含访客开关 |
| `domain_members` | 域的成员关系 |
| `domain_roles` | 域内角色 |
| `domain_role_permissions` | 域角色的权限 |
| `domain_guest_bans` | 访客封禁列表 |
| `rooms` | 频道/房间 |

### 消息

| 表 | 存储内容 |
|----|--------|
| `messages` | 消息，软删除 |
| `message_reactions` | 表情反应 |
| `message_mentions` | @ 提及 |
| `conversation_participants` | 私聊会话参与人 |

### 其他

| 表 | 存储内容 |
|----|--------|
| `mutes` | 禁言记录 |
| `audit_logs` | 审计日志 |
| `oauth_providers` / `oauth_accounts` | 第三方登录配置与绑定 |
| `email_configs` / `email_verification_codes` | 邮件配置与验证码 |
| `storage_configs` | 存储配置（密钥加密后存储） |
| `sfu_configs` / `sfu_active_providers` | 语音服务配置 |

## 状态的存储位置

| 内容 | 存储位置 | 原因 |
|------|------|--------|
| 用户、消息、房间 | 数据库 | 需持久化存储 |
| 登录令牌黑名单 | 进程内 NATS KV | 高频读写，过期自动清理 |
| 房间成员状态 | 进程内 NATS KV + 内存 | 实时性要求高，不需要落盘 |
| 前端界面状态 | localStorage / IndexedDB | 主题、音量等偏好设置 |
| 上传的文件 | 本机磁盘或对象存储 | 数据体积较大 |

**NATS 内嵌于应用进程**，不是外部服务。它承担内部事件分发、KV 状态存储与异步任务队列，随应用启动，无需也不应单独部署。

## 前端架构

React 19 + TanStack Router（文件路由）+ TanStack Query（服务端状态）+ zustand（客户端状态）。

状态管理分为两类：

- **服务端状态**（房间列表、消息、用户资料）→ TanStack Query，内置缓存与失效机制
- **客户端状态**（主题、当前选中、音量偏好）→ zustand，通过 `useXxxStore` 订阅

错误处理包含三层边界：

| 层 | 组件 | 职责 |
|----|------|--------|
| 组件 | `ErrorBoundary` | 组件崩溃不影响整页渲染 |
| 页面 | `PageErrorBoundary` | 页面级错误可整页重试 |
| 路由 | `RouteErrorComponent` | 路由级兜底 |

异步错误（Promise 与事件回调中产生的）由 `useErrorHandler` 重抛至渲染阶段，交由最近的边界处理。

## 桌面端

桌面端是一套独立的实现：由 Go + MyGo 编写的原生宿主，将同一份前端产物装入系统 WebView。

```
桌面窗口 ──► 127.0.0.1:随机端口（本地代理）
                    ├─ 前端（内嵌）
                    ├─ /api  ──► 所配置的服务器
                    ├─ /ws   ──► 所配置的服务器
                    └─ /uploads ──► 所配置的服务器
```

本地代理的作用是使页面与接口同源，从而避免跨域问题，并保证 Cookie 正常工作。

前端代码通过编译期常量划分平台：桌面独享的模块（托盘、浮窗）在 Web 构建时被整体剔除，不会进入产物包。

## 部署形态

```
             公网
               │
          ┌────▼─────────┐
          │ Caddy / 反代  │  HTTPS 终结（可选 edge 档位）
          └────┬─────────┘
               │  唯一上游 gospeak:8998
          ┌────▼────┐
          │ Go 应用 │ ── /livekit 同源反代 ──┐
          └────┬────┘                        │
               │                        ┌────▼────┐
          ┌────▼────┐                   │ LiveKit │
          │ 数据库  │                   └────▲────┘
          └─────────┘                        │
                                  WebRTC（客户端直连，不经过反代）
                                             │
                                       浏览器/桌面端
```

反代层只面对一个上游。`/api`、`/ws`、`/uploads`、`/livekit` 全部由 `gospeak:8998` 提供，因此 LiveKit 无需独立的域名、证书或反代规则；媒体流量走 UDP 直连。

小规模部署可以将全部组件放在同一台机器上。规模扩大后，LiveKit 与数据库可各自独立部署。

## 技术选型

| 层 | 采用技术 | 选型理由 |
|----|---------|--------|
| 后端语言 | Go | 单文件部署、并发性能好、编译速度快 |
| HTTP 框架 | Gin | 生态成熟、中间件机制清晰 |
| ORM | GORM | 自动迁移，三种数据库通用 |
| 数据库 | SQLite / PostgreSQL / MySQL | 渐进式选型，覆盖从小到大的部署规模 |
| 事件总线 | 内嵌 NATS | 不引入外部依赖，同时提供 KV 与队列能力 |
| 语音 | LiveKit | 功能完整，支持自建与云服务部署 |
| 前端 | React 19 | 生态最大，社区成熟、招聘容易 |
| 路由 | TanStack Router | 文件路由 + 类型安全 |
| 状态 | zustand | 较 Redux 更简洁，满足当前需求 |
| 样式 | Tailwind 4 | 无需维护独立 CSS 文件 |
| 组件 | Radix + 自建 | 无样式、可访问性好 |
| 构建 | rolldown-vite | 构建速度快于 vite |
| 桌面 | MyGo | 纯 Go，无 cgo，产物小 |

## 下一步

- [API 参考](/reference/api) — 接口细节
- [权限说明](/guide/permissions) — 鉴权流程说明
- [部署到服务器](/deploy/) — 部署形态
