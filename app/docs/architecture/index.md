# 架构

## 整体长什么样

```
┌─────────────┐
│   浏览器     │  或桌面客户端
└──────┬──────┘
       │
       ├──── HTTPS ─────► Go 应用 :8998 ──► 数据库
       │                    │
       ├──── WebSocket ─────┘  （信令：谁进谁出、谁在说话、新消息）
       │                    │
       │                    └── /livekit ──► LiveKit :7880（同源反代）
       │
       └──── WebRTC ──────► LiveKit :7882  （音频，UDP 直连）
```

三块东西，各干各的：

- **Go 应用**：业务逻辑、权限、数据存储，外加一个 WebSocket 信令服务
- **LiveKit**：只负责音频转发，不碰业务
- **数据库**：SQLite / PostgreSQL / MySQL

**音频不经过 Go 应用。** 这是整个设计里最重要的一点——应用不会被音频流量拖垮，两者可以独立扩容。

## 为什么这么拆

如果音频也走应用：

- 一个 20 人房间，每人说一句话要复制 19 份转发，应用直接跪
- 音频卡顿会拖累所有 HTTP 请求
- 想扩容就得把业务和媒体绑在一起扩

拆开之后，应用只需要：签个令牌、告诉前端连哪、在成员进出时更新状态。

## 后端分层

```
请求
 │
 ├─ Router           路由匹配
 ├─ Middleware       验令牌、查权限、限流
 ├─ Handler          解析参数、调 service、返回 JSON
 ├─ Service          业务逻辑，唯一有权限返回 AppError 的层
 ├─ Repository       数据访问
 └─ DB
```

规则很死：

- 每层只能调下一层，不能跨层
- Service 返回 `*AppError`，Handler 决定怎么转成响应
- Handler 里不写业务逻辑，Repository 里不写业务规则

这样改一个地方不会牵连一片。

## 目录结构

```
app/
├── server/                  Go 后端
│   ├── main.go              入口（cobra）
│   ├── server/gin.go        依赖注入，把所有层组装起来
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
└── docs/                    这个文档站

packages/
├── components/              UI 组件库（Radix + Tailwind）
├── sfu-client/              前端语音客户端抽象
└── desktop/                 桌面能力 SDK
```

## 语音抽象层

代码里有个 `sfu.Provider` 接口，把语音服务的能力抽象出来：

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

当前只有 LiveKit 一个实现。接口存在的意义是：业务层不关心底层是谁，将来接别的服务不用改业务代码。

`sfu.NewDynamicProvider()` 会在运行时解析当前用哪个实现——支持在管理后台切换而不重启。

## 数据模型

启动时 GORM 自动建表、加字段。不用手动跑 SQL。

### 用户与权限

| 表 | 存什么 |
|----|--------|
| `users` | 用户账号，含角色、令牌版本、是否访客 |
| `roles` | 角色（`admin` / `user` / `ban`） |
| `permissions` | 权限码定义 |
| `role_permissions` | 角色和权限的关联 |
| `user_groups` | 用户组 |

### 域与房间

| 表 | 存什么 |
|----|--------|
| `domains` | 语音域，含访客开关 |
| `domain_members` | 域的成员关系 |
| `domain_roles` | 域内角色 |
| `domain_role_permissions` | 域角色的权限 |
| `domain_guest_bans` | 访客封禁列表 |
| `rooms` | 频道/房间 |

### 消息

| 表 | 存什么 |
|----|--------|
| `messages` | 消息，软删除 |
| `message_reactions` | 表情反应 |
| `message_mentions` | @ 提及 |
| `conversation_participants` | 私聊会话参与人 |

### 其他

| 表 | 存什么 |
|----|--------|
| `mutes` | 禁言记录 |
| `audit_logs` | 审计日志 |
| `oauth_providers` / `oauth_accounts` | 第三方登录配置与绑定 |
| `email_configs` / `email_verification_codes` | 邮件配置与验证码 |
| `storage_configs` | 存储配置（密钥加密存） |
| `sfu_configs` / `sfu_active_providers` | 语音服务配置 |

## 状态存在哪

| 东西 | 位置 | 为什么 |
|------|------|--------|
| 用户、消息、房间 | 数据库 | 要持久 |
| 登录令牌黑名单 | 进程内 NATS KV | 高频读写，过期自动清 |
| 房间成员状态 | 进程内 NATS KV + 内存 | 实时性要求高，不需要落盘 |
| 前端界面状态 | localStorage / IndexedDB | 主题、音量偏好这些 |
| 上传的文件 | 本机磁盘或对象存储 | 体积大 |

**NATS 是进程内嵌的**，不是外部服务。它承担内部事件分发、KV 状态、异步任务队列，随应用启动，不用也不该单独部署。

## 前端架构

React 19 + TanStack Router（文件路由）+ TanStack Query（服务端状态）+ zustand（客户端状态）。

状态管理分两类：

- **服务端状态**（房间列表、消息、用户资料）→ TanStack Query，自带缓存和失效
- **客户端状态**（主题、当前选中、音量偏好）→ zustand，`useXxxStore` 订阅

错误处理有三层边界：

| 层 | 组件 | 管什么 |
|----|------|--------|
| 组件 | `ErrorBoundary` | 某个小组件崩了，不影响整页 |
| 页面 | `PageErrorBoundary` | 整个页面崩了，能重试 |
| 路由 | `RouteErrorComponent` | 路由级兜底 |

异步错误（Promise、事件回调里的）由 `useErrorHandler` 重抛到渲染阶段，交给最近的边界。

## 桌面端

桌面端是独立的一套：Go + MyGo 写的原生宿主，把同一份前端产物装进系统 WebView。

```
桌面窗口 ──► 127.0.0.1:随机端口（本地代理）
                    ├─ 前端（内嵌）
                    ├─ /api  ──► 你配的服务器
                    ├─ /ws   ──► 你配的服务器
                    └─ /uploads ──► 你配的服务器
```

起本地代理是为了让页面和接口同源——没有跨域问题，Cookie 也能正常用。

前端代码用编译期常量区分平台：桌面独享的模块（托盘、浮窗）在 Web 构建时被整体剔除，不进包。

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
                                  WebRTC（客户端直连，不经反代）
                                             │
                                       浏览器/桌面端
```

反代层只面对一个上游。`/api`、`/ws`、`/uploads`、`/livekit` 全在 `gospeak:8998` 里，所以 LiveKit 不需要自己的域名、证书或反代规则；媒体走 UDP 直连。

小规模可以全塞一台机器。规模上去之后，LiveKit 单独一台、数据库单独一台。

## 技术选型

| 层 | 用了什么 | 为什么 |
|----|---------|--------|
| 后端语言 | Go | 单文件部署、并发好、编译快 |
| HTTP 框架 | Gin | 生态成熟、中间件机制清晰 |
| ORM | GORM | 自动迁移，三种数据库通用 |
| 数据库 | SQLite / PostgreSQL / MySQL | 渐进式，从小到大都覆盖 |
| 事件总线 | 内嵌 NATS | 不引入外部依赖，KV + 队列都有 |
| 语音 | LiveKit | 功能完整，自建和云服务都能用 |
| 前端 | React 19 | 生态最大，招人好招 |
| 路由 | TanStack Router | 文件路由 + 类型安全 |
| 状态 | zustand | 比 Redux 简单，够用 |
| 样式 | Tailwind 4 | 不用维护 CSS 文件 |
| 组件 | Radix + 自建 | 无样式、可访问性好 |
| 构建 | rolldown-vite | 比 vite 快 |
| 桌面 | MyGo | 纯 Go，无 cgo，产物小 |

## 下一步

- [API 参考](/reference/api) — 接口细节
- [权限说明](/guide/permissions) — 鉴权怎么走
- [部署到服务器](/deploy/) — 部署形态
