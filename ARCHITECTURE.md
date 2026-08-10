# GOSpeak 项目架构图

## 整体架构

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              GOSpeak Monorepo                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐                              │
│  │    app/web      │    │   app/server    │                              │
│  │   (SolidJS)     │    │      (Go)       │                              │
│  └────────┬────────┘    └────────┬────────┘                                  │
│           │                      │                                             │
│           └──────────────────────┘                                             │
│                                  │                                              │
└──────────────────────────────────┼──────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Infrastructure                                     │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                        │
│  │ LiveKit  │  │   SRS    │  │  Agora   │  │Cloudflare│                        │
│  │   SFU    │  │   SFU    │  │   SFU    │  │   SFU    │                        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘                        │
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐              │
│  │    NATS KV      │    │    Database     │    │     OAuth       │              │
│  │   (Shared)      │    │  (SQLite/PG/My) │    │  (GitHub/GG/QQ) │              │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘              │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 目录结构

```
GOSpeak/
├── app/
│   ├── server/              # Go 后端 (Gin + GORM)
│   │   ├── cmd/             # CLI 入口 (cobra)
│   │   ├── internal/
│   │   │   ├── config/      # 配置读取 (env)
│   │   │   ├── model/       # 数据模型 (GORM entities)
│   │   │   ├── repository/  # DAO 层
│   │   │   ├── service/     # 业务逻辑层
│   │   │   ├── handler/     # HTTP 控制器
│   │   │   ├── middleware/  # JWT 认证、CORS、RBAC
│   │   │   ├── router/      # 路由注册
│   │   │   ├── sfu/         # SFU 抽象层
│   │   │   ├── livekit/     # LiveKit SFU 实现
│   │   │   ├── signal/      # WebSocket 信令 Hub
│   │   │   ├── authstate/   # JWT 认证状态 (NATS KV)
│   │   │   └── pkg/         # 共享工具 (errors, response, jwt, oauth)
│   │   ├── test/            # API 集成测试 (Node.js)
│   │   └── db/              # SQLite 数据库存储
│   ├── web/                 # SolidJS 前端
│   │   └── src/
│   │       ├── api/         # apiClient (axios) + auth API
│   │       ├── assets/      # 静态资源 (SVG icons, CSS)
│   │       ├── components/  # UI 组件 (room, modal, chat, form, common)
│   │       ├── hooks/       # 业务钩子 (livekit/, media.ts)
│   │       ├── layouts/     # 布局 (header, sidebar, main, footer)
│   │       ├── stores/      # 状态管理 (socket, user, theme, audioDevice, voiceChat)
│   │       └── types/       # TypeScript 类型定义
├── package.json             # Root scripts
├── pnpm-workspace.yaml      # Workspace config
└── AGENTS.md                # AI Agent 开发指南
```

## 前端架构 (app/web)

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           SolidJS Application                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                            Stores                                       │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │   │
│  │  │  userStore   │  │socketStore   │  │ voiceChat    │                  │   │
│  │  │  (Auth JWT)  │  │ (WebSocket)  │  │   Store      │                  │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘                  │   │
│  │  ┌──────────────┐  ┌──────────────┐                                    │   │
│  │  │ themeStore   │  │ audioDevice  │  ← IndexedDB 持久化               │   │
│  │  │ (Light/Dark) │  │    Store     │                                    │   │
│  │  └──────────────┘  └──────────────┘                                    │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                            Hooks                                        │   │
│  │  ┌──────────────────────────┐  ┌──────────────┐                        │   │
│  │  │     LiveKit Hooks        │  │    Media     │                        │   │
│  │  │  ┌────────┐ ┌────────┐  │  │   Hooks      │                        │   │
│  │  │  │Token   │ │Room    │  │  │              │                        │   │
│  │  │  │        │ │Action  │  │  └──────────────┘                        │   │
│  │  │  └────────┘ └────────┘  │                                           │   │
│  │  └──────────────────────────┘                                           │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                           Layouts                                        │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐               │   │
│  │  │  layout  │  │  header  │  │  sidebar │  │   main   │               │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘               │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                          Components                                     │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐               │   │
│  │  │   Room   │  │  Modal   │  │   Chat   │  │   Form   │               │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘               │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐                              │   │
│  │  │  Common  │  │   Home   │  │ userBar  │                              │   │
│  │  └──────────┘  └──────────┘  └──────────┘                              │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                           API Layer                                     │   │
│  │  ┌──────────────────┐  ┌──────────────────┐                            │   │
│  │  │   apiClient      │  │   auth API       │                            │   │
│  │  │   (Axios)        │  │   (Login/JWT)    │                            │   │
│  │  └──────────────────┘  └──────────────────┘                            │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘

Tech Stack:
- Framework: SolidJS + TanStack Router + TanStack Query
- State: SolidJS Store + IndexedDB (idb-keyval)
- Real-time: WebSocket Client + LiveKit Client SDK
- HTTP: Axios
- Build: Vite
```

## 后端架构 (app/server)

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                            Go Application                                       │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         Layered Call Flow                                │   │
│  │                                                                         │   │
│  │    Request → Router → Middleware → Handler → Service → Repository → DB │   │
│  │               (JWT+RBAC)    ↓         ↓         ↓                      │   │
│  │                         OAuth      SFU       AuthState                 │   │
│  │                     (standalone) (provider)(optional)                   │   │
│  │                                     ↓                                   │   │
│  │                                 Signal/WS                               │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         Repository Layer                                 │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐               │   │
│  │  │   User   │  │   Room   │  │  OAuth   │  │   DB     │               │   │
│  │  │   Repo   │  │   Repo   │  │   Repo   │  │  Init    │               │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘               │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                          Service Layer                                   │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐               │   │
│  │  │   Auth   │  │   User   │  │  OAuth   │  │   SFU    │               │   │
│  │  │ Service  │  │ Service  │  │ Service  │  │  Config  │               │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘               │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                          Handler Layer                                   │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐               │   │
│  │  │   Auth   │  │   User   │  │  OAuth   │  │  Signal  │               │   │
│  │  │ Handler  │  │ Handler  │  │ Handler  │  │ Handler  │               │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘               │   │
│  │  ┌──────────┐                                                            │   │
│  │  │   SFU    │                                                            │   │
│  │  │ Config   │                                                            │   │
│  │  └──────────┘                                                            │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         Middleware Layer                                 │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │   │
│  │  │   JWTAuth    │  │ RequireRole  │  │    CORS      │                  │   │
│  │  │  (token)     │  │   (RBAC)     │  │  (preflight) │                  │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘                  │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         Core Modules                                    │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │   │
│  │  │   Signal     │  │    SFU       │  │  LiveKit     │                  │   │
│  │  │   Hub        │  │  Provider    │  │  Service     │                  │   │
│  │  │ (WebSocket)  │  │  Interface   │  │   (SDK)      │                  │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘                  │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │   │
│  │  │  AuthState   │  │   Config     │  │   OAuth      │                  │   │
│  │  │  (Optional)  │  │  (Env Vars)  │  │  Providers   │                  │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘                  │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘

Tech Stack:
- Language: Go 1.22+
- HTTP: Gin
- Database: GORM (SQLite/PostgreSQL/MySQL)
- Shared state: NATS JetStream KV (黑名单、密钥轮换、房间状态)
- Real-time: WebSocket (nhooyr.io/websocket)
- WebRTC: LiveKit Server SDK
- Auth: JWT + OAuth2
```

## SFU 抽象层

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        SFU Provider Abstraction                                │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         Provider Interface                              │   │
│  │                                                                         │   │
│  │  type Provider interface {                                              │   │
│  │      GenerateToken(room, identity string) (string, error)              │   │
│  │      GenerateAdminToken() (string, error)                               │   │
│  │      ListRooms() (interface{}, error)                                  │   │
│  │      ListParticipants(room string) (interface{}, error)                │   │
│  │      MuteParticipant(room, identity, trackSid string, muted bool) err  │   │
│  │      RemoveParticipant(room, identity string) error                     │   │
│  │      DeleteRoom(room string) error                                      │   │
│  │      GetHost() string                                                   │   │
│  │  }                                                                      │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                    │                                           │
│                    ┌───────────────┼───────────────┐                           │
│                    ▼               ▼               ▼                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                        │
│  │ LiveKit  │  │   SRS    │  │  Agora   │  │Cloudflare│                        │
│  │ Highest  │  │  High    │  │  Medium  │  │  Medium  │                        │
│  │ Full API │  │ Partial  │  │ Partial  │  │ Partial  │                        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘                        │
│  配置: SFU_PROVIDER="livekit" | "srs" | "agora" | "cloudflare"             │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 数据流架构

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Data Flow                                          │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  Client (Browser)                                                               │
│       │                                                                         │
│       ├─── HTTP ──────────────────────► Go Server (REST API)                   │
│       │                                    │                                    │
│       │                                    ├─── GORM ──► Database              │
│       │                                    │     (SQLite/PG/MySQL)             │
│       │                                    │                                    │
│       │                                    └─── NATS KV ──► Shared State      │
│       │                                          (Blacklist/Key Rotation)      │
│       │                                                                         │
│       ├─── WebSocket (WebSocket) ─────► Go Server (Signal Hub)                 │
│       │                                    │                                    │
│       │                                    ├─── Memory ──► Room Registry       │
│       │                                    │                                    │
│       │                                    └─── SFU Provider ──► SFU Server    │
│       │                                          (LiveKit/SRS/Agora/Cloudflare)     │
│       │                                                                         │
│       └─── WebRTC ────────────────────► SFU Server                             │
│                                            │                                    │
│                                            └─── Media ──► Other Peers          │
│                                                                                 │
│  OAuth Flow:                                                                    │
│       │                                                                         │
│       └─── 302 Redirect ─────► GitHub/Google/QQ OAuth                          │
│                                    │                                           │
│                                    ├─── code ──► Exchange Token                │
│                                    │                                           │
│                                    └─── access_token ──► Get User Info        │
│                                                                  │              │
│                                               ◄──────────────────┘              │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 部署架构

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           Docker Compose                                        │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                        SFU Backend (任选其一)                            │   │
│  │                                                                         │   │
│  │  LiveKit: ┌──────────────────┐                                          │   │
│  │           │  livekit-server  │                                          │   │
│  │           │  :7880 (HTTP)    │                                          │   │
│  │           │  :7881 (WebRTC)  │                                          │   │
│  │           └──────────────────┘                                          │   │
│  │                                                                         │   │
│  │  SRS:     ┌──────────────────┐                                          │   │
│  │           │    SRS Server    │                                          │   │
│  │           │  :1985 (HTTP)    │                                          │   │
│  │           │  :8000 (RTMP)    │                                          │   │
│  │           └──────────────────┘                                          │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                        Go Server                                         │   │
│  │  ┌──────────────────────────────────────────────────────────────────┐   │   │
│  │  │  :8998 (HTTP API + WebSocket + Swagger)                          │   │   │
│  │  └──────────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                        Web App (Static)                                  │   │
│  │  ┌──────────────────────────────────────────────────────────────────┐   │   │
│  │  │  Nginx/Caddy :80/:443                                            │   │   │
│  │  └──────────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                        Optional Services                                 │   │
│  │  ┌──────────────────┐     ┌──────────────────┐                          │   │
│  │  │   NATS           │     │   PostgreSQL     │                          │   │
│  │  │  (embedded)      │     │   :5432           │                          │   │
│  │  └──────────────────┘     └──────────────────┘                          │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## API 路由结构

```
/api/v1
├── /auth
│   ├── POST /login                    # 登录
│   ├── POST /register                 # 注册
│   ├── POST /refresh_token            # 获取 Refresh Token
│   ├── POST /reset_password           # 重置密码
│   ├── POST /logout                   # 登出 (需认证)
│   ├── POST /refresh                  # 刷新 Token (需认证)
│   ├── POST /change_password          # 修改密码 (需认证)
│   └── POST /first_change_password    # 首次修改密码 (需认证)
│
├── /oauth
│   ├── GET  /login/:provider          # OAuth 登录 (重定向)
│   ├── GET  /callback/:provider       # OAuth 回调
│   ├── GET  /admin/providers          # 列出 OAuth 提供商 (admin)
│   ├── POST /admin/providers          # 创建 OAuth 提供商 (admin)
│   ├── PUT  /admin/providers          # 更新 OAuth 提供商 (admin)
│   └── DELETE /admin/providers/:id    # 删除 OAuth 提供商 (admin)
│
├── /signal
│   ├── POST /token                    # 获取加入房间 Token
│   ├── POST /signal                   # 信令消息
│   ├── POST /webhook                  # LiveKit Webhook
│   ├── GET  /rooms                    # 列出房间
│   └── GET  /participants             # 列出参与者
│
├── /sfu
│   ├── GET  /config                   # 获取 SFU 配置 (需认证)
│   └── POST /update-config            # 更新 SFU 配置 (需认证)
│
├── /user
│   ├── POST /profile                  # 获取个人资料 (需认证)
│   ├── POST /list                     # 列出用户 (需认证)
│   ├── POST /get                      # 获取用户 (需认证)
│   ├── POST /delete                   # 删除用户 (admin)
│   └── POST /update-role              # 更新角色 (admin)
│
├── /ping                              # 健康检查
│
└── /swagger/*any                      # Swagger UI
```

## WebSocket 事件 (WebSocket)

```
Client → Server:                              Server → Client:
───────────────                               ───────────────

Lifecycle:
├── connection                                (内部处理)
└── disconnect

Room Management:
├── room:create    { room }           ──►    ├── room:created    RoomInfo
├── room:join      { room, identity } ──►    ├── room:joined     { room, members[] }
├── room:leave     { room }           ──►    ├── room:left       { room, identity }
└── room:list      (无 payload)       ──►    └── room:list:result { rooms[] }

Member Events:
                                            ├── room:updated    RoomInfo (成员数变化)
                                            ├── member:joined   MemberInfo
                                            ├── member:left     { identity }
                                            └── member:updated  MemberInfo

WS Endpoint: /ws
```

## 统一响应格式

```json
// Success
{
  "code": 0,
  "msg": "success",
  "data": { ... }
}

// Error
{
  "code": 1011,
  "msg": "user not found",
  "data": null
}
```

## 错误码定义

| Code | Constant | 说明 |
|------|----------|------|
| 0 | `SUCCESS` | 成功 |
| 1001 | `TOKEN_NOT_EXIST` | Token 不存在 |
| 1002 | `TOKEN_WRONG` | Token 错误 |
| 1003 | `TOKEN_EXPIRED` | Token 已过期 |
| 1010 | `INVALID_PASSWORD` | 密码错误 |
| 1011 | `USER_NOT_FOUND` | 用户不存在 |
| 1012 | `USERNAME_EXISTS` | 用户名已存在 |
| 1013 | `FORBIDDEN` | 禁止访问 |
| 1014 | `TOKEN_REVOKED` | Token 已撤销 |
| 2001 | `INVALID_PARAMS` | 参数无效 |
| 3001 | `NOT_FOUND` | 资源不存在 |
| 3002 | `ALREADY_EXISTS` | 资源已存在 |
| 5001 | `INTERNAL_ERROR` | 服务器内部错误 |
| 6001 | `SFU_NOT_CONFIGURED` | SFU 未配置 |
| 6002 | `SFU_ERROR` | SFU 错误 |
| 7001 | `OAUTH_PROVIDER_NOT_FOUND` | OAuth 提供商不存在 |
| 7002 | `OAUTH_PROVIDER_DISABLED` | OAuth 提供商已禁用 |
| 7003 | `OAUTH_TOKEN_EXCHANGE_FAILED` | OAuth Token 交换失败 |
| 7004 | `OAUTH_GET_USER_FAILED` | OAuth 获取用户信息失败 |

## 技术栈总结

| 层级 | 技术 | 用途 |
|------|------|------|
| **前端框架** | SolidJS | 响应式 UI |
| **路由** | TanStack Router | 客户端路由 |
| **数据获取** | TanStack Query | 服务端状态管理 |
| **实时通信** | WebSocket | 信令服务器 |
| **WebRTC** | LiveKit / SRS / Agora / Cloudflare | 音视频通信 |
| **后端语言** | Go | 服务器逻辑 |
| **HTTP 框架** | Gin | REST API |
| **ORM** | GORM | 数据库操作 |
| **数据库** | SQLite / PostgreSQL / MySQL | 数据持久化 |
| **跨实例状态** | NATS JetStream KV | 黑名单/密钥轮换/房间状态 |
| **认证** | JWT + OAuth2 (GitHub/Google/QQ) | 用户认证 |
| **构建工具** | Vite | 前端打包 |
| **包管理** | pnpm | Monorepo 管理 |
| **容器化** | Docker Compose | 部署编排 |

## 核心设计原则

1. **分层架构**: Handler → Service → Repository，单向依赖
2. **SFU 抽象**: 多 SFU 后端可插拔，通过 Provider 接口统一
3. **可选依赖**: NATS 内嵌/外部可选，状态共享优雅降级
4. **渐进式数据库**: SQLite 开箱即用 → PostgreSQL/MySQL 可扩展
5. **统一响应**: 所有 API 使用相同 JSON 格式
6. **错误处理**: Service 层返回 `*AppError`，Handler 层统一响应

## 可选 NATS 信号事件总线

GOSpeak 在进程内通过 EventBus 做信令 fanout：

- `NATS_URL` 空：启动内嵌 `nats-server`（默认随机端口；`NATS_EMBEDDED_PORT` 可固定）
- `NATS_URL` 非空：先探测外部；可用则 external，不可用则 Warn 并回退内嵌
- Hub 广播（`member:*` / `room:*` / `user:muted` / `sfu:provider-changed` 等）先本地 WebSocket 投递，再经 NATS 复制到其他实例
二进制仍为单文件：链接 `nats.go` 客户端 + `nats-server` 库，不附带独立 nats-server 可执行文件。

内部事件使用 `PublishInternal`（subject `{prefix}.internal`），例如权限缓存失效 `cache:permissions-invalidated`，不经 WebSocket Deliverer。

房间人数视图：membership JetStream KV 为跨实例真相源；本机 join/leave 写 KV 后 `PublishInternal(state:room-changed)`，对端 `ApplyRemoteRoomState` 从 KV 合并并本地广播 `room:updated`/`room:list:result`。Socket 连接与踢人控制仍在本机 Hub。


