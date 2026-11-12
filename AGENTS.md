# GOSpeak Monorepo — AI Agent Guide

## Project Overview

GOSpeak 是一个自托管的**游戏语音平台**，类似自部署版 Discord 语音房间。基于 WebRTC，pnpm monorepo：Go 后端 + React 19 前端，SFU 抽象层基于 LiveKit。

**为什么自建？** 游戏语音数据不经第三方、自定义语音路由策略、无用户数限制、完全控制部署架构。

关键能力：
- 语音房间管理（创建/加入/密码保护/踢人）
- 实时发言检测 + 成员独立音量控制
- LiveKit SFU 音视频引擎
- 渐进式数据库（SQLite 开箱即用 → PostgreSQL → MySQL → Turso 远程库）
- JWT + OAuth2 三端登录（GitHub / Google / QQ），HttpOnly Cookie 会话 + 惰性续期
- Domain（语音服务器）多租户隔离 + 访客访问体系
- 文字房间消息（房间消息 + 私聊）
- 内嵌 NATS 事件总线（JetStream KV 状态共享：JWT 黑名单/密钥轮换/禁言规则）

```
GOSpeak/
├── app/
│   ├── server/          # Go 后端 (Gin + GORM + SFU abstraction)
│   ├── web/             # React 19 前端 (TypeScript + rolldown-vite + @tanstack/react-router)
│   │   └── src/_solid-legacy/   # 迁移暂存的 SolidJS 旧代码，不参与构建
│   └── docs/            # VitePress 文档站 (architecture / deployment / guide / sfu)
├── packages/
│   ├── components/       # UI 组件库 (Radix 无头组件 + 自有主题 token，shadcn 风格)
│   ├── sfu-client/       # 共享 SFU 客户端类型与工厂
│   └── desktop/          # 桌面能力 SDK（MyGo 桥接 + 条件编译工具，见「Web / 桌面能力边界」）
├── test/                # API 集成测试 (@gospeak/test，Vitest + Node)
├── docs/                # 设计/运维/历史档案（design、review、superpowers specs/plans）
├── deploy/              # docker-compose、Caddyfile、livekit 配置、observability（入口 DEPLOY.md）
├── scripts/             # changelog 生成等辅助脚本
├── .github/             # CI workflows（build/ci/release-please 等）+ dependabot
├── agent_test_logs/     # Agent 测试日志输出目录
├── pnpm-workspace.yaml  # Workspace 配置（packages/*、app/*、test）
├── turbo.json           # 任务编排（build/dev/lint/test 走 turbo）
└── AGENTS.md            # ← This file
```

> Swagger 生成产物在 `app/server/docs/`（由 swag 生成，`/swagger/*any` 托管），顶层 `docs/` 是设计与运维文档。

---

## Server Architecture — Enterprise Layered Design

```
app/server/
├── main.go                 # Entry point
├── cmd/
│   └── root.go             # CLI (cobra): `server`, `version` commands
├── server/
│   ├── gin.go                    # DI container, initializes all layers
│   └── bootstrap.go               # 启动写面初始化（种子、回填）
├── internal/
│   ├── config/             # Config reading from env (DB, SFU, JWT, NATS, Storage)
│   ├── model/              # Data models (GORM entities)
│   ├── repository/         # DAO layer, direct DB access
│   ├── service/            # Business logic layer
│   ├── handler/            # HTTP controller (Gin handlers)
│   ├── middleware/         # JWT auth, CORS, permission-based RBAC, ban check
│   ├── router/             # Route registration (sub-route modules)
│   │   └── routes/         # Per-module route groups
│   ├── sfu/                # SFU provider abstraction layer
│   │   ├── provider.go     # Provider interface + StreamProvider/ClientInfoProvider
│   │   ├── types.go        # Capabilities, EnforcementLevel, RoomSummary, ParticipantSummary
│   │   ├── capabilities.go # CapabilitiesFor() per provider
│   │   ├── mute_rule_store.go  # SFU 端禁言规则存储
│   │   └── providers/      # SFU provider 实现
│   │       └── livekit/    # LiveKit gRPC SDK
│   ├── signal/             # WebSocket signaling hub
│   │   ├── events.go       # 46 event name constants
│   │   ├── types.go        # RoomRequest, MemberInfo, RoomInfo
│   │   ├── hub.go          # Hub (room registry + all event handlers)
│   │   ├── state_sync.go   # Domain 命名空间 + 状态同步
│   │   └── message_bridge.go # 消息事件桥接
│   ├── ws/                 # WebSocket 基础设施（nhooyr.io/websocket）
│   │   ├── client.go       # Client (read loop, goroutine-safe write)
│   │   ├── fanout.go       # Fanout Broadcaster (room-based fan-out)
│   │   ├── upgrader.go     # HTTP→WS upgrade + access cookie 鉴权 + origin 校验
│   │   └── handler.go      # HandlerRegistry (event dispatch + panic recover)
│   ├── bus/                # Embedded NATS event bus (内部事件/KV 状态/任务队列)
│   │   ├── bus.go          # EventBus interface + Envelope
│   │   ├── embedded.go     # Embedded NATS server
│   │   ├── nats_bus.go     # Embedded NATS bus 实现
│   │   ├── ws_deliverer.go # NATS → local WS bridge
│   │   ├── membership_store.go  # Room member / stream 状态共享（NATS KV）
│   │   ├── mute_rule_store.go   # 降级 mute 缓存
│   │   ├── auth_store.go   # JWT blacklist + signing key rotation
│   │   └── factory.go      # Init() 启动内嵌 NATS 并创建总线
│   ├── authstate/          # JWT auth state (blacklist, key rotation; NATS KV/memory)
│   ├── logger/             # Unified logrus wrapper (level/format/output + gin middleware)
│   ├── permcode/           # Permission code constants
│   │   ├── permcode.go     # Platform permissions
│   │   └── domain_permcode.go  # Domain permissions
│   ├── storage/            # Object storage abstraction (local/S3)
│   ├── jobs/               # Background jobs (placeholder)
│   └── pkg/                # Shared utilities
│       ├── errors.go       # Business error codes + AppError + ErrSFUNotSupported
│       ├── response.go     # Unified JSON response + HandleError
│       ├── jwt.go          # JWT token generation/parsing
│       └── oauth/          # Generic OAuth2 provider abstraction
├── db/                     # SQLite database storage
├── .env.dev / .env.prod    # Environment config
└── go.mod
```

### Layered Call Flow

```
Request → Router → Middleware → Handler → Service → Repository → DB
                  (JWT+RBAC)      ↓         ↓         ↓
                               OAuth       SFU      AuthState
                            (standalone) (provider) (optional)
                                          ↓
                                       Signal/WS
```

Each layer communicates **only with the layer directly below it**:
- **Handler** receives HTTP request, validates input, calls Service
- **Service** implements business logic, calls Repository and SFU-related modules
- **Repository** is pure data access, returns GORM errors
- **SFU providers** are standalone packages wrapping provider-specific APIs
- **Signal** is a standalone WebSocket hub with its own event system
- **Bus** is an embedded NATS event bus for internal events, KV-backed state (membership/mute rules/auth), and async job queues

---

## Unified API Response Format

All API responses follow the exact same JSON structure:

```json
// Success
{
  "code": 0,
  "msg": "success",
  "data": { ... }
}

// Error — data is always null
{
  "code": 1012,
  "msg": "username already exists",
  "data": null
}
```

### How to return responses in Go

```go
// Success — data can be any value (struct, map, slice, nil)
pkg.Success(c, data)

// Error with AppError (service layer)
return nil, pkg.NewAppError(pkg.USER_NOT_FOUND, "user not found")

// Error with HandleError (handler layer)
pkg.HandleError(c, err)    // auto-detects *AppError, hides internal detail for 5xxx/6xxx

// Error with explicit code
pkg.Fail(c, pkg.INVALID_PARAMS, "custom message")  // msg is optional
```

### Business Status Codes

| Code | Constant | Meaning | HTTP |
|------|----------|---------|------|
| 0 | `SUCCESS` | success | 200 |
| 1001 | `TOKEN_NOT_EXIST` | token does not exist | 401 |
| 1002 | `TOKEN_WRONG` | token is wrong | 401 |
| 1003 | `TOKEN_EXPIRED` | token has expired | 401 |
| 1010 | `INVALID_PASSWORD` | invalid password | 401（登录失败统一 401 防用户名枚举） |
| 1011 | `USER_NOT_FOUND` | user not found | 401（同上） |
| 1012 | `USERNAME_EXISTS` | username already exists | 400 |
| 1013 | `FORBIDDEN` | forbidden | 403 |
| 1014 | `TOKEN_REVOKED` | token has been revoked | 401 |
| 1015 | `USER_BANNED` | user has been banned | 403 |
| 1016 | `USER_MUTED` | user has been muted | 403 |
| 1017 | `RATE_LIMITED` | too many requests | 429 |
| 2001 | `INVALID_PARAMS` | invalid parameters | 400 |
| 3001 | `NOT_FOUND` | resource not found | 404 |
| 3002 | `ALREADY_EXISTS` | resource already exists | 409 |
| 5001 | `INTERNAL_ERROR` | internal server error | 500 |
| 6001 | `SFU_NOT_CONFIGURED` | sfu not configured | 503 |
| 6002 | `SFU_ERROR` | sfu error | 502 |
| 7001 | `OAUTH_PROVIDER_NOT_FOUND` | oauth provider not found | 404 |
| 7002 | `OAUTH_PROVIDER_DISABLED` | oauth provider is disabled | 503 |
| 7003 | `OAUTH_TOKEN_EXCHANGE_FAILED` | oauth token exchange failed | 502 |
| 7004 | `OAUTH_GET_USER_FAILED` | oauth get user info failed | 502 |
| 8001 | `EMAIL_ALREADY_EXISTS` | email already exists | 400 |
| 8002 | `EMAIL_CODE_INVALID` | invalid verification code | 400 |
| 8003 | `EMAIL_CODE_EXPIRED` | verification code expired | 400 |
| 8004 | `EMAIL_CODE_ALREADY_USED` | verification code already used | 400 |
| 8005 | `EMAIL_SEND_TOO_FREQUENT` | send too frequent | 400 |
| 8006 | `EMAIL_SEND_FAILED` | email send failed | 502 |
| 8007 | `EMAIL_NOT_VERIFIED` | email not verified | 400 |
| 8008 | `EMAIL_CODE_MAX_ATTEMPTS` | too many attempts | 400 |
| 8009 | `EMAIL_NOT_CONFIGURED` | email not configured | 503 |
| 8010 | `PASSWORD_RESET_DISABLED` | password reset disabled | 403 |
| 8101 | `STORAGE_NOT_CONFIGURED` | storage not configured | 503 |
| 8102 | `STORAGE_ERROR` | storage error | 502 |
| 8103 | `STORAGE_FILE_TOO_LARGE` | file too large | 400 |
| 8104 | `STORAGE_FILE_TYPE_NOT_ALLOWED` | file type not allowed | 400 |

### HandleError 细节策略

`HandleError` 会判断业务码，**隐藏内部实现细节**（不透传 err.Error()）对于：INTERNAL_ERROR、SFU_ERROR、STORAGE_ERROR、OAUTH_*、EMAIL_SEND_FAILED，客户端只收到默认文案。

---

## Error Handling Pattern

### Service Layer — Return `*AppError`

```go
func (s *UserService) GetByID(id uint) (*model.User, error) {
    user, err := s.userRepo.GetByID(id)
    if err != nil {
        if err == gorm.ErrRecordNotFound {
            return nil, pkg.NewAppError(pkg.NOT_FOUND, "user not found")
        }
        return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
    }
    return user, nil
}
```

### Handler Layer — Use `HandleError`

```go
func (h *UserHandler) GetByID(c *gin.Context) {
    user, err := h.userService.GetByID(id)
    if err != nil {
        pkg.HandleError(c, err)  // auto-maps *AppError → JSON
        return
    }
    pkg.Success(c, user)
}
```

### Validation Errors — Use `Fail` directly

```go
if err := c.ShouldBindJSON(&req); err != nil {
    pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
    return
}
```

---

## API Route Conventions

- **All routes** are prefixed with `/api/v1/`
- Grouped by module: `auth`, `guest`, `user`, `user-group`, `signal`, `oauth`, `role`, `permission`, `mute`, `room`, `message`(挂 `/room`), `storage`, `email`, `email_config`, `sfu`, `system`, `domain`, `conversation`, `audit`
- The `protected` group applies `middleware.JWTAuth()` + `middleware.BanCheck()` + `middleware.GuestGuard()` to every route
- Permission-gated routes additionally use `middleware.RequirePermission(permcode.X)` (permission-based RBAC)
- Public routes (login/register/refresh_token/reset_password、guest 签发、email 验证码、oauth、signal exchange) are outside the protected group；登录/注册/重置 10 次/分、refresh_token 60 次/分、email 验证码 5 次/分、guest 签发 60 次/时限流
- Users resolve permissions via `role → role_permissions`

### Current Route Table

> 「Guard」列的 `RequireDomainMember()` 等为域成员校验型中间件；`—` 表示仅 protected 组默认鉴权，无额外权限门。限流标注在 Auth 列。

| Method | Path | Auth | Guard / Permission | Handler |
|--------|------|------|-----------|---------|
| POST | `/api/v1/auth/login` | No（限流 10/min） | — | AuthHandler.Login |
| POST | `/api/v1/auth/register` | No（限流 10/min） | — | AuthHandler.Register |
| POST | `/api/v1/auth/refresh_token` | No（限流 60/min） | — | AuthHandler.GetRefreshToken |
| POST | `/api/v1/auth/reset_password` | No（限流 10/min） | — | AuthHandler.ResetPassword |
| POST | `/api/v1/auth/guest` | No（限流 60/h） | — | GuestHandler.Join |
| POST | `/api/v1/auth/logout` | JWT | — | AuthHandler.Logout |
| POST | `/api/v1/auth/refresh` | JWT | — | AuthHandler.RefreshToken |
| POST | `/api/v1/auth/change_password` | JWT | — | AuthHandler.ChangePassword |
| POST | `/api/v1/auth/first_change_password` | JWT | — | AuthHandler.FirstChangePassword |
| POST | `/api/v1/auth/guest/renew` | JWT(guest) | — | GuestHandler.Renew |
| POST | `/api/v1/user/profile` | JWT | — | UserHandler.GetProfile |
| POST | `/api/v1/user/info` | JWT | — | UserHandler.GetByName |
| GET/POST | `/api/v1/user/preset-avatars` | JWT | — | UserHandler.PresetAvatars |
| POST | `/api/v1/user/update-profile` | JWT | — | UserHandler.UpdateProfile |
| POST | `/api/v1/user/list` | JWT | `user:read` | UserHandler.List |
| POST | `/api/v1/user/get` | JWT | `user:read` | UserHandler.GetByID |
| POST | `/api/v1/user/delete` | JWT | `user:delete` | UserHandler.Delete |
| POST | `/api/v1/user/update-role` | JWT | `user:update` | UserHandler.UpdateRole |
| POST | `/api/v1/user-group/list` | JWT | — | UserGroupHandler.List |
| POST | `/api/v1/user-group/create` | JWT | — | UserGroupHandler.Create |
| POST | `/api/v1/user-group/update` | JWT | — | UserGroupHandler.Update |
| POST | `/api/v1/user-group/delete` | JWT | — | UserGroupHandler.Delete |
| POST | `/api/v1/signal/token` | JWT | — | SignalHandler.GetJoinToken |
| GET | `/api/v1/signal/ws-endpoint` | JWT | — | SignalHandler.GetWSEndpoint |
| GET | `/api/v1/signal/rooms` | JWT | — | SignalHandler.ListRooms |
| GET | `/api/v1/signal/participants` | JWT | — | SignalHandler.ListParticipants |
| POST | `/api/v1/signal/webhook` | No | — | SignalHandler.LivekitWebhook |
| GET | `/api/v1/oauth/login/:provider` | No | — | OAuthHandler.Login (redirect) |
| GET | `/api/v1/oauth/callback/:provider` | No | — | OAuthHandler.Callback |
| GET | `/api/v1/oauth/providers` | No | — | OAuthHandler.ListEnabledProviders |
| GET | `/api/v1/oauth/admin/providers` | JWT | `oauth:read` | OAuthHandler.ListProviders |
| POST | `/api/v1/oauth/admin/providers` | JWT | `oauth:manage` | OAuthHandler.CreateProvider |
| PUT | `/api/v1/oauth/admin/providers` | JWT | `oauth:manage` | OAuthHandler.UpdateProvider |
| DELETE | `/api/v1/oauth/admin/providers/:id` | JWT | `oauth:manage` | OAuthHandler.DeleteProvider |
| POST | `/api/v1/role/list` | JWT | `role:read` | RoleHandler.List |
| POST | `/api/v1/role/create` | JWT | `role:manage` | RoleHandler.Create |
| POST | `/api/v1/role/update` | JWT | `role:manage` | RoleHandler.Update |
| POST | `/api/v1/role/delete` | JWT | `role:manage` | RoleHandler.Delete |
| POST | `/api/v1/permission/list` | JWT | `role:read` | PermissionHandler.ListPermissions |
| POST | `/api/v1/permission/role` | JWT | `role:read` | PermissionHandler.GetRolePermissions |
| POST | `/api/v1/permission/sync` | JWT | `role:manage` | PermissionHandler.SyncRolePermissions |
| POST | `/api/v1/mute/create` | JWT | `mute:manage` | MuteHandler.CreateMute |
| POST | `/api/v1/mute/cancel` | JWT | `mute:manage` | MuteHandler.CancelMute |
| POST | `/api/v1/mute/status` | JWT | `mute:manage` | MuteHandler.GetMuteStatus |
| POST | `/api/v1/mute/list` | JWT | `mute:manage` | MuteHandler.ListMutes |
| POST | `/api/v1/room/create` | JWT | `RequirePlatformAdminOrDomainMember()` | RoomHandler.Create |
| POST | `/api/v1/room/list` | JWT | `RequirePlatformAdminOrDomainMemberIfProvided()` | RoomHandler.List |
| POST | `/api/v1/room/get` | JWT | — | RoomHandler.Get |
| POST | `/api/v1/room/update` | JWT | — | RoomHandler.Update |
| POST | `/api/v1/room/delete` | JWT | — | RoomHandler.Delete |
| POST | `/api/v1/room/messages/list` | JWT | — | MessageHandler.List |
| POST | `/api/v1/room/messages/search` | JWT | — | MessageHandler.Search |
| POST | `/api/v1/room/messages/react` | JWT | — | MessageHandler.React |
| POST | `/api/v1/storage/presign` | JWT | — | StorageHandler.PresignUpload |
| POST | `/api/v1/storage/confirm` | JWT | — | StorageHandler.ConfirmUpload |
| POST | `/api/v1/storage/upload` | JWT | — | StorageHandler.Upload |
| POST | `/api/v1/storage/delete` | JWT | `storage:delete` | StorageHandler.DeleteObject |
| POST | `/api/v1/storage/config` | JWT | `storage:read` | StorageHandler.GetConfig |
| POST | `/api/v1/storage/update-config` | JWT | `storage:manage` | StorageHandler.UpdateConfig |
| POST | `/api/v1/storage/test-config` | JWT | `storage:read` | StorageHandler.TestConfig |
| POST | `/api/v1/email/send_code` | No（限流 5/min） | — | EmailVerificationHandler.SendCode |
| POST | `/api/v1/email/verify_code` | No（限流 5/min） | — | EmailVerificationHandler.VerifyCode |
| POST | `/api/v1/email/config` | JWT | `email_config:read` | EmailConfigHandler.Get |
| POST | `/api/v1/email/update-config` | JWT | `email_config:manage` | EmailConfigHandler.Update |
| POST | `/api/v1/sfu/config` | JWT | `sfu:manage` | SFUConfigHandler.Get |
| POST | `/api/v1/sfu/config/:provider` | JWT | `sfu:manage` | SFUConfigHandler.GetProvider |
| POST | `/api/v1/sfu/update-config` | JWT | `sfu:manage` | SFUConfigHandler.Update |
| POST | `/api/v1/sfu/switch-provider` | JWT | `sfu:manage` | SFUConfigHandler.SwitchProvider |
| POST | `/api/v1/sfu/providers` | JWT | `sfu:manage` | SFUConfigHandler.ListProviders |
| GET | `/api/v1/system/stream` | JWT | `RequireRole("admin")` | MonitorHandler.HealthStream |
| POST | `/api/v1/domain/guest/config` | JWT | — | GuestHandler.Config |
| POST | `/api/v1/domain/guest/ban` | JWT | — | GuestHandler.Ban |
| POST | `/api/v1/domain/guest/unban` | JWT | — | GuestHandler.Unban |
| POST | `/api/v1/domain/guest/ban-list` | JWT | — | GuestHandler.BanList |
| POST | `/api/v1/domain/guest/cleanup` | JWT | — | GuestHandler.Cleanup |
| POST | `/api/v1/domain/create` | JWT（限流 10/min） | `domain:create` | DomainHandler.Create |
| POST | `/api/v1/domain/get` | JWT | `RequireDomainMember()` | DomainHandler.Get |
| POST | `/api/v1/domain/list` | JWT | `domain:read` | DomainHandler.List |
| POST | `/api/v1/domain/list-public` | JWT | — | DomainHandler.ListPublic |
| POST | `/api/v1/domain/my-domains` | JWT | — | DomainHandler.MyDomains |
| POST | `/api/v1/domain/update` | JWT | `RequireDomainMember()` | DomainHandler.Update |
| POST | `/api/v1/domain/reset-invite` | JWT | `RequireDomainMember()` | DomainHandler.ResetInvite |
| POST | `/api/v1/domain/delete` | JWT | `RequireDomainMember()` | DomainHandler.Delete |
| POST | `/api/v1/domain/join` | JWT | — | DomainHandler.Join |
| POST | `/api/v1/domain/preview` | JWT | — | DomainHandler.Preview |
| POST | `/api/v1/domain/leave` | JWT | `RequireDomainMember()` | DomainHandler.Leave |
| POST | `/api/v1/domain/kick` | JWT | `RequireDomainMember()` | DomainHandler.Kick |
| POST | `/api/v1/domain/members` | JWT | `RequireDomainMember()` | DomainHandler.Members |
| POST | `/api/v1/domain/members/update-role` | JWT | `RequireDomainMember()` | DomainHandler.UpdateMemberRole |
| POST | `/api/v1/domain/roles/list` | JWT | `RequireDomainMember()` | DomainHandler.ListRoles |
| POST | `/api/v1/domain/roles/create` | JWT | `RequireDomainMember()` | DomainHandler.CreateRole |
| POST | `/api/v1/domain/roles/update` | JWT | `RequireDomainMember()` | DomainHandler.UpdateRole |
| POST | `/api/v1/domain/roles/delete` | JWT | `RequireDomainMember()` | DomainHandler.DeleteRole |
| POST | `/api/v1/domain/my-permissions` | JWT | `RequireDomainMember()` | DomainHandler.MyPermissions |
| GET | `/api/v1/conversation/list` | JWT | — | ConversationHandler.List |
| POST | `/api/v1/conversation/messages` | JWT | — | ConversationHandler.Messages |
| POST | `/api/v1/conversation/mark-read` | JWT | — | ConversationHandler.MarkRead |
| POST | `/api/v1/conversation/send` | JWT | — | ConversationHandler.Send |
| POST | `/api/v1/audit/list` | JWT | `audit:read` | AuditHandler.List |
| WS | `/ws` | Cookie（access token） | — | WebSocket signaling（子协议仅传协议名 `gospeak`，不携带 token） |
| GET | `/ping` | No | — | Health check |
| GET | `/readyz` | No | — | Readiness probe（探测 DB 等依赖） |
| GET | `/swagger/*any` | No | — | Swagger UI |
| GET | `/uploads/*filepath` | No | — | Static avatar/uploads (safe path traversal) |

> SPA 托管：`STATIC_DIR` > `/app/static` > `./static` > go:embed 内嵌资源，由 NoRoute fallback 提供。

---

## Configuration

All configuration is injected via environment variables (`APP_ENV` selects dev/prod；`.env.dev` for dev, `deploy/.env` for Docker — 单一配置文件，`deploy/init.sh` 生成；`COMPOSE_PROFILES` 默认 `app,livekit`，加 `edge` 启用内置 Caddy). Startup loads env files without overriding process env (`process > .env.<env> > .env > defaults`), then `config.Load()` parses into a typed `Config` via `caarlos0/env`, normalizes aliases (e.g. `PostgresSQL` → `PostgreSQL`), validates. 配置已通过 `server/gin.go` 装配时以显式 `config.Config` 注入到各内部包；全局 `config.Current()` 当前代码库无任何包使用，保留仅为历史兼容占位，新代码不要依赖它。

### Database

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_TYPE` | `SQLite` | `SQLite` / `PostgreSQL` / `MySQL`（`PostgresSQL` / `MYSQL` 为兼容别名）|
| `DB_HOST` | `localhost` | DB host (PostgreSQL/MySQL) |
| `DB_PORT` | — | DB port |
| `DB_USER` | — | DB user |
| `DB_PASSWORD` | — | DB password |
| `DB_PATH` | `db/app.db` | SQLite file path |
| `DB_DSN` | — | Custom DSN (overrides the field-by-field settings) |
| `DB_WAL` | `false` | SQLite WAL mode（并发读建议开启）|
| `TURSO_DATABASE_URL` / `TURSO_AUTH_TOKEN` | — | Turso (libSQL) 远程库；`TURSO_DATABASE_URL` 优先于 `DB_DSN`，token 亦可用 `DB_DSN?authToken=` 传入 |

### JWT

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_KEY` | `default-secret` | HMAC signing key (change in production) |
| `JWT_KEY_TTL` | `24h` | Signing-key rotation interval (NATS KV) |
| `JWT_ACCESS_TTL` | `15m` | Access token TTL（login/refresh 响应 `expires_in` 同源） |
| `JWT_REFRESH_TTL` | `168h` | Refresh token TTL |
| `BCRYPT_COST` | `12` | 密码哈希工作因子 |

### Auth Cookies（HttpOnly Cookie 承载 token）

access/refresh token 均以 HttpOnly Cookie 下发（HTTP 与 WS 握手统一走 cookie 鉴权）：

| Variable | Default | Description |
|----------|---------|-------------|
| `ACCESS_COOKIE_NAME` | `gospeak_token` | access token cookie 名 |
| `REFRESH_COOKIE_NAME` | `gospeak_refresh_token` | refresh token cookie 名 |
| `COOKIE_DOMAIN` / `COOKIE_PATH` | — / `/` | Cookie 作用域 |
| `COOKIE_SECURE` | `auto` | `auto`\|`true`\|`false` |
| `COOKIE_SAMESITE` | `lax` | `lax`\|`strict`\|`none` |

### SFU (LiveKit)

`SFU_PROVIDER` 固定为 `livekit`。SFU 配置：

| Variable | Default | Description |
|----------|---------|-------------|
| `LIVEKIT_HOST` / `LIVEKIT_KEY` / `LIVEKIT_SECRET` | — | LiveKit server URL, API key/secret |


> Base SFU config is loaded from env; persistent SFU config is managed through `/api/v1/sfu/*` and resolved at runtime by `factory.NewDynamicProvider(...)`.

### NATS / Bus (Embedded)

| Variable | Default | Description |
|----------|---------|-------------|
| `NATS_SUBJECT_PREFIX` | `gospeak` | NATS subject prefix |
| `NATS_NAME` | — | Instance name (auto = hostname-pid) |
| `NATS_CONNECT_TIMEOUT` | `2s` | Connection timeout |
| `NATS_EMBEDDED_PORT` | — | Embedded NATS port (random if empty) |
| `NATS_WAL_PATH` | — | 断线事件磁盘 WAL 路径（空 = 禁用持久化） |

### Email / SMTP (optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `EMAIL_ENABLED` | `false` | Enable email verification |
| `SMTP_HOST` / `SMTP_PORT` | — / `587` | SMTP server & port |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | — | SMTP credentials |
| `SMTP_FROM` / `SMTP_FROM_NAME` | — / `GoSpeak` | Sender address / name |
| `EMAIL_CODE_TTL` | `10m` | Verification-code TTL |
| `EMAIL_SEND_COOLDOWN` | `60s` | Per-email send cooldown |
| `EMAIL_CODE_SECRET` | — | Code signing secret (required when email enabled) |

### Object Storage

| Variable | Default | Description |
|----------|---------|-------------|
| `STORAGE_TYPE` | `local` | `local` / `s3` |
| `STORAGE_ENDPOINT` | — | S3-compatible endpoint (RustFS / R2) |
| `STORAGE_BUCKET` | — | S3 bucket |
| `STORAGE_REGION` | — | S3 region |
| `STORAGE_ACCESS_KEY` / `STORAGE_SECRET_KEY` | — | S3 credentials |
| `STORAGE_PUBLIC_BASE_URL` | — | Public base URL (CDN / custom domain) |
| `STORAGE_PATH_PREFIX` | `uploads/` | S3 对象键前缀（本地存储忽略）|
| `STORAGE_LOCAL_DIR` | `uploads` | 本地存储落盘目录，与 `/uploads` 静态服务同源 |
| `STORAGE_ENCRYPT_KEY` | — | Encryption key for secrets at rest |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8998` | HTTP listen port |
| `STATIC_DIR` | — | Frontend static dir (SPA hosting in prod; 回退 `/app/static` > `./static` > go:embed) |
| `GIN_MODE` | — | Gin mode（空 = gin 默认 `debug`，prod 强制 `release`） |
| `CORS_ORIGIN` | `*` | CORS allowed origin（逗号分隔白名单，`*` 放行任意来源） |
| `WS_ALLOWED_ORIGINS` | — | WebSocket origin whitelist (empty = same-origin) |
| `TRUSTED_PROXIES` | — | 信任的代理 IP/CIDR（逗号分隔），用于 ClientIP 解析 |
| `TLS_CERT` / `TLS_KEY` | — | 同时配置时直跑 HTTPS + HTTP/2 |
| `OAUTH_AUTO_CREATE_USER` | `true` | OAuth 回调时是否自动创建未绑定用户（关闭后仅允许已绑定账号登录） |

### Metrics

| Variable | Default | Description |
|----------|---------|-------------|
| `METRICS_TOKEN` | — | Metrics 访问令牌 |

### Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | dev=`debug` / prod=`info` | `trace` / `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | dev=`text` / prod=`json` | `text` / `json` |
| `LOG_OUTPUT` | `stdout` | `stdout` / `stderr` / `file` / `both` |
| `LOG_FILE` | `logs/app.log` | path when output is `file`/`both` |
| `LOG_CALLER` | `false` | print caller file:line |

---

## SFU Abstraction Layer

Standalone package at `internal/sfu/`. Provides a provider interface so multiple SFU backends can be plugged in and resolved dynamically at runtime.

### Provider Interface (`internal/sfu/provider.go`)

```go
type Provider interface {
    ProviderName() string
    Capabilities() Capabilities
    GenerateToken(room, identity string) (string, error)
    ListRooms() ([]RoomSummary, error)
    ListParticipants(room string) ([]ParticipantSummary, error)
    MuteParticipant(room, identity, trackSid string, muted bool) error
    RemoveParticipant(room, identity string) error
    DeleteRoom(room string) error
    GetHost() string
}

// StreamProvider — stream-based addressing (WHIP/WHEP)
type StreamProvider interface {
    Provider
    StreamName(room, identity string) string
    StreamInfo(room, identity string) (stream, token string, err error)
}

// ClientInfoProvider — exposes provider-specific connection metadata to frontend
type ClientInfoProvider interface {
    Provider
    ClientInfo() map[string]interface{}
}

// PublishControlProvider — 由支持发布控制的 Provider 实现：
// 签发 token 时显式声明是否允许发布媒体流（访客说权限等场景）
type PublishControlProvider interface {
    GenerateTokenWithPublish(room, identity string, canPublish bool) (string, error)
}

// TimedMuteProvider — 支持带 TTL 的定时禁言；Hub 优先于 MuteParticipant 使用
type TimedMuteProvider interface {
    Provider
    MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error
}
```

### Capabilities & Enforcement Levels (`internal/sfu/types.go`)

Each provider declares its media-layer enforcement capability matrix:

```go
type Capabilities struct {
    ServerMute  bool   // true when MuteLevel is hard or degraded
    ServerKick  bool   // true when KickLevel is hard or degraded
    DeleteRoom  bool
    ListRooms   bool
    ListMembers bool
    MuteLevel   string  // hard | degraded | soft | none
    KickLevel   string
    DeleteLevel string
    ListLevel   string
}
```

**Enforcement levels:**

| Level | Meaning |
|-------|---------|
| `hard` | Native SFU media force (e.g. LiveKit server mute) |
| `degraded` | Substitute media force (e.g. 信令层禁推) |
| `soft` | Signal/policy + client cooperation only |
| `none` | Capability absent |

`sfu.AllProviderCapabilities()` returns the capability matrix for the registered provider (`livekit`). `CapabilitiesFor(name)` returns a single provider's matrix.

### Provider Capability Summary

| Provider | ServerMute | ServerKick | MuteLevel | KickLevel |
|----------|-----------|-----------|-----------|-----------|
| LiveKit | ✅ | ✅ | hard | hard |

### Factory (`internal/sfu/factory/`)

`factory.NewProvider(cfg)` reads `cfg.SFUProvider` and returns the matching implementation. Registered providers: `livekit`.

`factory.NewDynamicProvider(resolve)` is the runtime entry used by server wiring. It resolves config via `SFUConfigService.ResolveConfig()`（配置缓存 TTL 2s）and delegates each call to the current provider；另提供 `SetRoomRegistry` / `SetStreamRoomResolver` / `SetMuteRuleStore` 等注入点。

---

## AuthState Module

Package `internal/authstate/` manages JWT auth state (blacklist, refresh-family replay protection, signing-key rotation) without Redis. Multi-instance state is stored in NATS JetStream KV (`bus.AuthStore`); single-process fallback is in-memory or static `JWT_KEY`.

### Token Blacklist

Logout marks the token's JTI revoked with TTL = remaining token lifetime. Middleware checks blacklist via `authstate.IsBlacklistedErr(jti)`. 无共享后端时写入**进程内内存黑名单**（单机生效、跨实例不共享），非 no-op。

### JWT Key Rotation

`authstate.GetSigningKey()` returns the current signing key:
- **NATS KV available**: key stored in the shared auth bucket; `JWT_KEY_TTL` (default 24h) triggers automatic rotation.
- **No shared backend**: falls back to static `JWT_KEY` env var (default `"default-secret"`); production requires an explicit `JWT_KEY`.

---

## Bus Module (Embedded Event Bus)

In-process embedded NATS event bus and shared state at `internal/bus/`. The bus **always runs embedded** (`NATS_URL` 不再支持)：单进程启动内嵌 nats-server（含 JetStream），承担内部事件、缓存失效、KV 状态与异步任务队列。

### Capabilities

| Capability | Backend | Purpose |
|------------|---------|---------|
| EventBus | Embedded NATS | Internal events（缓存失效/跨模块通知）+ 本机 WebSocket fanout 桥接 |
| MembershipStore | NATS KV | Room member / stream mapping |
| MuteRuleStore | NATS KV → memory | Degraded mute cache |
| AuthStore | NATS KV | JWT blacklist + signing key rotation |
| JobQueue | NATS JetStream | Chat persistence / SFU cleanup async tasks |

### Init API

```go
bus, cleanup, err := bus.Init(bus.InitConfig{
    Prefix:         "gospeak",
    Name:           instanceID,
    ConnectTimeout: 2 * time.Second,
    EmbeddedPort:   0,             // 0 = random port
    WALPath:        walPath,       // 断线事件磁盘持久化（空 = 禁用）
    Deliverer:      wsDeliverer,
    RemoteHook:     onRemoteEvent,
})
defer cleanup()
```

### Resolution Order

- `MuteRule`: final fallback is in-memory (never blocks startup)
- `Auth`: NATS KV first; static `JWT_KEY` last

### WSDeliverer

`internal/bus/ws_deliverer.go` bridges NATS fanout into the local WebSocket server.

---

## WebSocket Module (`internal/ws/`)

Pure WebSocket infrastructure layer using `nhooyr.io/websocket`.

### Key files

| File | Role |
|------|------|
| `client.go` | Client struct: read loop, goroutine-safe write queue (64 chan), close |
| `fanout.go` | Fanout Broadcaster: room-based broadcast, namespace broadcast, per-room iteration |
| `broadcaster.go` | `Broadcaster` 接口（Fanout 的抽象，含 `CloseAll`） |
| `messenger.go` | `ClientMessenger` / `StatefulMessenger` 接口 |
| `upgrader.go` | HTTP→WS upgrade: origin check + **access cookie 鉴权** + client creation |
| `handler.go` | HandlerRegistry: event dispatch + panic recover + ACK/ErrorACK |
| `types.go` | Message / ACK / ACKError |

### WS Client Lifecycle

```
HTTP GET /ws
  → Upgrader.originAllowed()      (checks Origin header)
  → accessTokenFromCookie(r)      (HttpOnly access cookie，空则 401)
  → VerifyToken()                 (middleware.VerifyToken shared logic + 必须为 access token 类型)
  → NewClient(conn, id, claims)
  → Fanout.Add(client)
  → client.StartReadLoop(HandlerRegistry.Dispatch)
    → on close: Fanout.Remove → Hub.OnDisconnect (room cleanup)
```

### WS 握手鉴权

- **只读 HttpOnly access cookie**（名字取自 `ACCESS_COOKIE_NAME`，默认 `gospeak_token`）；缺失直接 401。不再支持 Authorization header / subprotocol token。
- `Sec-WebSocket-Protocol` 只接受协议名 `gospeak`，不再承载 token。
- token 必须为 access 类型（refresh 类型拒绝）。

### Fanout Broadcaster

Room names use compound keys `signal.roomKey(domainUUID, roomName)` for Domain namespace isolation.

```go
// Fanout implements ws.Broadcaster
fanout.Add(client)                    // register client
fanout.Remove(clientID)               // deregister, returns affected rooms
fanout.Join(room, clientID)           // client joins room
fanout.Leave(room, clientID)          // client leaves room
fanout.BroadcastToRoom(room, event, data)
fanout.BroadcastToNamespace(event, data)
fanout.ForEach(room, fn)              // iterate room members
fanout.RoomExists(room) bool
fanout.GetClient(clientID) ClientMessenger
```

---

## Signal (WebSocket) Module (`internal/signal/`)

WebSocket signaling hub built on `internal/ws/`.

### Key files

| File | Role |
|------|------|
| `events.go` | 46 event name constants |
| `types.go` | `RoomRequest`, `MemberInfo`, `RoomInfo` structs |
| `hub.go` | Hub (global room registry + all event handlers + Domain state sync) |
| `state_sync.go` | Domain namespace isolation + cross-instance state synchronization |
| `message_bridge.go` | Message events bridge |
| `private_bridge.go` | Private message bridge |

### Hub struct

```go
type Hub struct {
    sfuProvider      sfu.Provider
    participantCleanup ParticipantCleanupHandler
    // ...
}
```

### Hub 房间查询方法

| 方法 | 返回 |
|------|------|
| `GetSFURooms()` | 仅内存活跃房间（有 WS 连接的），供 `room:list` 广播 |
| `GetRooms()` | DB 持久化房间 + 内存活跃房间合并 |
| `GetRoomMembers(room)` | 指定房间的在线成员 |

### OnRoomKick SFU dispatch

信令层始终先处理（删 Members + 广播），随后由 `Hub.removeParticipantSafe` 调用 `sfuProvider.RemoveParticipant(room, identity)`。Hub 不硬编码 provider 名，仅在返回 `pkg.ErrSFUNotSupported` 时静默跳过：

| provider | SFU RemoveParticipant | 状态 |
|----------|----------------------|------|
| livekit | ✅ LiveKit gRPC | 完整实现 |

### Mute/ListParticipants 分工

| 操作 | 信令层 | SFU 层 | 说明 |
|------|--------|--------|------|
| `RemoveParticipant` | ✅ 删 Members + 广播 | ✅ 按 provider 能力 | |
| `ListRooms` + `ListParticipants` | 失败时返回 `[]` | ✅ 有则返回 | SFU 媒体状态 ≠ 信令层在线状态，不 fallback |
| `MuteParticipant` | `BroadcastMute` 广播 | ✅ 按 provider 能力调用 | hard/degraded 媒体强制；不支持时 soft 兜底并依赖前端停推流 |

### 发言检测

LiveKit 由 SFU 原生事件驱动，同时保留前端上报链路作为兜底：
- 前端 `onLocalSpeakingChange` 上报「自身」本地麦克风音量状态，经 `member:speaking` 发往信令层
- `Hub.OnMemberSpeaking` 按房间聚合 `Room.Speaking`，广播 `room:active-speakers`（identities 列表）
- 成员离开 / 断连 / 被踢时清发言态；原本在发言则广播最新列表以重置高亮

---

## OAuth Module

Provides generic OAuth2 third-party login. Providers are configured in the `oauth_providers` table and managed through the admin API; `github`, `google`, and `qq` are seeded presets, and arbitrary OpenID/OAuth2 providers are supported via custom endpoint URLs plus JSON field mappings.

### Provider Abstraction (`internal/pkg/oauth/`)

Defines `Provider` interface for the three-step OAuth2 flow:

```go
type Provider interface {
    GetAuthURL(state string) string        // Step 1: build auth URL
    ExchangeToken(code string) (string, error) // Step 2: code → access_token
    GetUserInfo(accessToken string) (*UserInfo, error) // Step 3: get user info
}
```

Built-in providers: `GitHubProvider`, `GoogleProvider`, `QQProvider`. Factory: `oauth.NewProvider(name, cfg)`.

### Data Layer

| Model | Table | Description |
|-------|-------|-------------|
| `OAuthProvider` | `oauth_providers` | Platform config (name, display_name, icon_url, client_id, secret, endpoints, field mappings, enabled) |
| `OAuthAccount` | `oauth_accounts` | User ↔ platform binding (user_id, provider, provider_uid) |

---

## Domain (语音服务器)

GOSpeak 支持多 Server（类 Discord 服务器）架构，即多租户语音域（Domain）架构。每个 `Domain` 是房间、成员、角色的顶层归属容器。

### 数据模型

| 表 | 说明 |
|----|------|
| `domains` | 语音服务器：UUID、名称、IconURL、描述、Owner、邀请码、公开/私有、访客能力开关（AllowGuest / GuestCanListen / GuestCanSpeak / GuestCanMessage / GuestLimit） |
| `domain_members` | 用户-Domain 多对多关系：Nickname、RoleName (owner/admin/member/guest)、JoinedAt |
| `domain_roles` | 域内角色（唯一约束 domain+name），系统角色 IsSystem |
| `domain_role_permissions` | 域角色 → 域权限码（domain_permcode）关联 |
| `domain_guest_bans` | 域级访客封禁（domain+user 唯一，支持过期时间） |

### Domain 角色层级

```
owner (4) > admin (3) > member (2) > guest (1)
```

系统角色权限由 `SeedDefaultDomainRoles` 播种（owner=全部域权限；admin/member/guest 各自收敛），域角色权限码定义在 `internal/permcode/domain_permcode.go`。

### Room 归属

`Room.DomainUUID` 外键关联到 Domain，**not null**（`room` 表 Name 唯一约束为 domain+name 复合）。
新增房间必须归属特定 Domain；存量 `domain_uuid` 为空的房间由启动迁移归入 "Default Server"。

### Signal 命名空间隔离

Signal Hub 中使用 `roomKey(domainUUID, roomName)` 复合键隔离不同 Domain 的同名房间。

### 中间件

- `RequireDomainMember()` / `RequireDomainMemberIfProvided()` — 校验当前用户是目标 Domain 的成员（后者仅在请求携带 domain 参数时校验）。
- `RequirePlatformAdminOrDomainMember()` / `...IfProvided()` — 平台管理员或域成员放行（room create/list 使用）。
- `IsDomainMember` / `IsPlatformAdmin` — 供 handler/service 层复用的判定函数。

### 迁移策略

- 启动时若不存在任何 Domain，自动创建 "Default Server" 并将存量 `domain_uuid` 为空的房间归入其中。
- 创建 Domain 时播种系统角色（`SeedDefaultDomainRoles`）。
- 启动时执行 `BackfillDomainRoleDefaults`（幂等）为所有存量域补播系统角色，覆盖域角色体系上线前创建的域。

---

## Middleware (`internal/middleware/`)

| Function | Description |
|----------|-------------|
| `VerifyToken(tokenStr)` | Shared token verification: signature → token 类型白名单 → UserUUID 非空 → expiry → blacklist → version. Returns `*Claims, ErrCode`. Used by both HTTP and WS paths. |
| `JWTAuth()` | Validates Bearer token（缺失时回退 HttpOnly access cookie）, injects context |
| `RequireRole(roles ...string)` | Legacy role check against the `role` claim |
| `RequirePermission(permCode)` | Permission-based gate. Users map `role` → permissions |
| `RequireOwnerOrPermission(ownerContextKey, permCode)` | Allows if caller owns the resource or holds the permission |
| `PermissionGranted(claims, role, permCode, checker)` | Unified permission check for HTTP and WS paths |
| `BanCheck()` | Blocks any user whose `role` is `ban`; returns `USER_BANNED` (1015) |
| `GuestGuard()` / `GuestGuardWith(whitelist)` | 访客守卫：白名单外接口对 `is_guest` 用户返回 1013（挂在 protected 组全局生效） |
| `RequireDomainMember()`（及 IfProvided 变体） | 校验当前用户是目标 Domain 成员 |
| `RequirePlatformAdminOrDomainMember()`（及 IfProvided 变体） | 平台管理员或域成员放行 |
| `RateLimit(max, window)` | 固定窗口限流（登录/注册/验证码/guest 签发等） |
| `RequestID()` | 请求 ID 注入 |
| `CORS(origins []string)` | 基于显式白名单的 CORS（`*` 放行任意来源），处理 OPTIONS preflight |

### JWTAuth check order

1. `Authorization` header；缺失时回退 HttpOnly access cookie（默认 `gospeak_token`）→ 都没有则 `TOKEN_NOT_EXIST` (1001)
2. 签名合法 → `TOKEN_WRONG` (1002)
3. token 类型在白名单内（access）且 UserUUID 非空 → 否则 `TOKEN_WRONG` (1002)
4. 未过期 → `TOKEN_EXPIRED` (1003)
5. JTI 不在黑名单 → `TOKEN_REVOKED` (1014)
6. `TokenVersion` 匹配当前用户 → 否则 `TOKEN_REVOKED` (1014)
8. Inject `username`, `display_name`, `user_uuid`, `role`, `claims`, `auth_type` into context

---

## RBAC / Permissions

Access control is **permission-based**, layered on top of roles.

### Permission Codes

| Code | Description |
|------|-------------|
| `room:create` | 创建房间 |
| `room:read` | 查看房间列表和详情 |
| `room:update` | 修改房间 |
| `room:delete` | 删除房间 |
| `domain:create` | 创建语音服务器 |
| `domain:read` | 查看语音服务器 |
| `domain:manage` | 修改语音服务器设置 |
| `domain:delete` | 删除语音服务器 |
| `domain:invite` | 管理邀请码 |
| `domain:kick` | 将成员移出语音服务器 |
| `domain:role:manage` | 管理语音服务器内角色 |
| `user:read` | 查看用户 |
| `user:update` | 编辑用户 |
| `user:delete` | 删除用户 |
| `role:read` | 查看角色 |
| `role:manage` | 管理角色 |
| `signal:kick` | 将用户从语音房间中踢出 |
| `mute:manage` | 对用户进行全局禁言 |
| `sfu:manage` | 查看和修改 SFU 配置 |
| `email_config:read` | 查看 SMTP 配置 |
| `email_config:manage` | 修改 SMTP 配置 |
| `storage:read` | 查看存储配置 |
| `storage:manage` | 修改存储配置 |
| `storage:delete` | 删除存储对象 |
| `oauth:read` | 查看 OAuth 配置 |
| `oauth:manage` | 管理 OAuth 配置 |
| `message:send` | 发送消息 |
| `message:read` | 查看历史消息 |
| `message:delete_others` | 删除他人消息 |
| `audit:read` | 查询审计日志（注意：未进入默认权限种子，需手工授予） |

### Default Role Permissions

| Role | Permissions |
|------|------------|
| `admin` | 显式枚举全部平台权限（不含 `audit:read`） |
| `user` | `room:create`, `room:read`, `domain:create`, `user:read`, `role:read`, `message:send`, `message:read` |
| `ban` | (none — intercepted by `BanCheck()`) |

### Default admin account

First boot seeds user `admin` / `admin123` when missing. Login returns `need_change_password=true` while still on the default password.

### Token versioning

`User.TokenVersion` is embedded in the JWT. Changing a password / resetting it bumps the version, so all previously issued tokens are rejected (`TOKEN_REVOKED`).

---

## Models

| Model | Table | Key Fields |
|-------|-------|------------|
| `User` | `users` | ID, UUID (auto-gen), Name, DisplayName, Avatar, Email, EmailVerified, IsBot, IsGuest, Status (`active`/`banned`/`disabled`), Password (`json:"-"`), Role, TokenVersion, timestamps |
| `Room` | `room` | ID, UUID (auto-gen), Name（domain 内唯一）, Password (`json:"-"`), Description, Limit, AudioOnly, AllowAudience, Type (`voice`/`text`), CreatedBy, DomainUUID (not null), timestamps |
| `Role` | `roles` | ID, Name (seeds: `admin` / `user` / `ban`), timestamps |
| `Permission` | `permissions` | ID, Code (unique `permcode`), Name, Description, timestamps |
| `RolePermission` | `role_permissions` | ID, RoleName, PermissionID |
| `Mute` | `mutes` | ID, UUID, UserID, MuterID, Duration, Permanent, ExpiresAt, Reason, timestamps |
| `Domain` | `domains` | UUID, Name, IconURL, Description, OwnerUUID, InviteCode, IsPublic, AllowGuest, GuestCanListen/Speak/Message, GuestLimit, timestamps |
| `DomainMember` | `domain_members` | DomainUUID, UserUUID, Nickname, RoleName, JoinedAt |
| `DomainRole` | `domain_roles` | DomainUUID, Name（domain 内唯一）, IsSystem, timestamps |
| `DomainRolePermission` | `domain_role_permissions` | DomainUUID, RoleName, PermissionCode |
| `DomainGuestBan` | `domain_guest_bans` | DomainUUID, UserUUID（联合唯一）, Reason, BannedBy, ExpiresAt, timestamps |
| `AuditLog` | `audit_logs` | UUID, ActorID/ActorUUID/ActorName, Action, TargetType/TargetID, Detail, IP, Success, timestamps |
| `UserGroup` | `user_groups` | UserID, GroupName（联合唯一）, timestamps |
| `Message` | `messages` | UUID, RoomUUID, AuthorID, AuthorUUID, Content, ReplyTo, ConversationType (`room`/`private`), ConversationID, TargetIdentity, EditedAt, Status (`active`/`deleted`), DeletedAt (软删), timestamps |
| `MessageMention` | `message_mentions` | MessageUUID, UserID |
| `MessageReaction` | `message_reactions` | MessageUUID, UserID, Emoji (unique per user+message+emoji) |
| `ConversationParticipant` | `conversation_participants` | ConversationID (PK), IdentityA/IdentityB（字典序）, LastMessageID/LastContent/LastSenderIdentity/LastMessageAt, UnreadCountA/B, timestamps |
| `OAuthProvider` | `oauth_providers` | ID, Name, DisplayName, IconURL, ClientID, ClientSecret, AuthURL, TokenURL, UserInfoURL, RedirectURL, Scopes, field mappings, Enabled, timestamps |
| `OAuthAccount` | `oauth_accounts` | ID, UserID, Provider, ProviderUID; AccessToken & RefreshToken stored but hidden (`json:"-"`), timestamps |
| `EmailConfig` | `email_configs` | ID, Enabled, SMTP host/port/user/pass/from/name, EmailCodeTTL, EmailSendCooldown, EmailCodeSecret (hidden, `json:"-"`), timestamps |
| `EmailVerificationCode` | `email_verification_codes` | ID, Email, Scene, CodeHash (hidden), UserID, IPAddress, ExpiresAt, UsedAt, AttemptCount, timestamps |
| `StorageConfig` | `storage_configs` | ID, ProviderType (`local`/`s3`), Endpoint, Bucket, Region, AccessKey & SecretKey (hidden), PublicBaseURL, PathPrefix, MaxFileSize, AllowedTypes, timestamps |
| `SFUConfig` | `sfu_configs` | Provider (PK), per-provider config fields, timestamps |
| `SFUActiveProvider` | `sfu_active_provider` | ID, Provider |

> Auto-migration (`repository/db_migrations.go`) syncs all of the above on startup.

---

## Frontend Architecture

前端已从 SolidJS 迁移到 React 19：React 19 + react-dom 19，`@tanstack/react-router` 文件路由（`src/pages` → `routeTree.gen.ts`，`vite.config.ts` 内联 `tanstackRouter({ target: "react" })`），`@tanstack/react-query` 管理服务端状态，zustand 管理客户端状态（`useXxxStore` hook + `xxxStore` 命令式门面，组件用 selector 订阅）。UI 组件来自 workspace 包 `packages/components`（Radix 无头组件 + 自有主题 token，shadcn 风格，`theme.css`），配 Tailwind CSS 4；图标 `lucide-react`，toast `sonner`，表单 `react-hook-form` + `zod`，测试 vitest 4 + `@testing-library/react`。构建为 rolldown-vite 7（`@vitejs/plugin-react`、`@tanstack/router-plugin/vite`、`@tailwindcss/vite`、`@tanstack/devtools-vite`），dev 端口 3000，`/api/v1` 与 `/ws` 代理到后端 8998（原生 WebSocket，无 socket.io）。`src/_solid-legacy/` 为不参与构建的迁移暂存旧代码。前端已建立三层错误边界与统一上报：组件级 `ErrorBoundary`、页面级 `PageErrorBoundary`、路由级 `RouteErrorComponent`（HOC `withErrorBoundary()` 便捷包裹）；`hooks/useErrorHandler.ts` 把 Promise.catch / 事件回调里的异步错误重抛到渲染阶段交给最近边界；`utils/errorReport.ts` 统一上报（`registerErrorReporter` 可接 Sentry 等外部日志），并由 `installGlobalErrorHandlers()` 兜底 `window.onerror` 与 `unhandledrejection`，`main.tsx` 注册默认错误组件与全局监听（重试时 `router.invalidate()`）。

```
app/web/src/
├── api/                # apiClient (axios wrapper) + typed API modules
│   ├── apiClient.ts    # Axios instance with auth interceptor（401 时触发会话续期）
│   ├── authTransport.ts# 会话惰性续期：401 拦截 + refresh + expires_in 学习
│   ├── auth.ts         # Login/register/refresh/logout
│   ├── user.ts         # Profile/info/update
│   ├── room.ts         # Room CRUD
│   ├── sfu.ts          # SFU config + token fetch
│   ├── sfuProfiles.ts  # SFU provider 能力画像
│   ├── ws.ts           # WS endpoint（握手鉴权由 HttpOnly access cookie 自动携带）
│   ├── domain.ts       # Domain CRUD + join/leave/kick/roles
│   ├── guest.ts        # 访客加入/续约/管理
│   ├── message.ts      # Message send/edit/delete/react
│   ├── conversation.ts # Conversation list/messages/mark-read/send
│   ├── mute.ts         # Mute create/cancel/status/list
│   ├── oauth.ts        # OAuth login
│   ├── permission.ts   # Role/permission APIs
│   ├── role.ts         # 平台角色管理
│   ├── storage.ts      # Storage presign/confirm
│   ├── email.ts        # Email send/verify
│   ├── userGroup.ts    # 用户分组
├── socket/             # WebSocket client
│   ├── wsClient.ts     # WebSocket connection manager (reconnect, ACK, fire-and-forget)
│   ├── events.ts       # EVENTS 常量对象（42 个事件键）
│   ├── socketEvents.ts # 服务端推送事件绑定
│   ├── types.ts        # RoomInfo, MemberInfo, MuteEvent, etc.
│   ├── roomState.ts    # Room list/members state merge helpers
│   ├── providerReload.ts   # SFU hot-swap handler (disconnect + reload)
│   ├── tabLock.ts      # Single-tab socket owner (BroadcastChannel)
│   └── *.test.ts       # Unit tests
├── stores/             # zustand stores (useXxxStore hook + xxxStore 门面；IndexedDB 持久化)
│   ├── socketStore.ts  # WebSocket connection + room/member state + SFU events
│   ├── userStore.ts    # Auth state (persisted to IndexedDB)
│   ├── guestStore.ts   # 访客能力（listen/speak/message）
│   ├── themeStore.ts   # Light/dark theme (localStorage)
│   ├── audioDeviceStore.ts  # Audio device enumeration (IndexedDB)
│   ├── voiceChatStore.ts    # Mute/volume state (IndexedDB)
│   ├── chatStore.ts    # Chat/conversation state (IndexedDB)
│   └── domainStore.ts # Domain state (内存 store)
├── pages/              # TanStack Router 文件路由：(app)/ index、chat、discover、domain、link、manage、profile；login/、register/、guest/、invite/
├── hooks/              # React hooks
│   ├── media.ts        # Audio device enumeration utility
│   ├── useAuthParallax.ts
│   ├── useUpload.ts
│   └── useErrorHandler.ts
├── components/
│   ├── common/         # Shared UI (Avatar, Modal, Divider, Toast, etc.) + 错误边界 (errorBoundary.tsx: ErrorBoundary / PageErrorBoundary / RouteErrorComponent / withErrorBoundary；errorFallback.tsx: ErrorFallback)
│   ├── form/           # Form / PasswordChangeForm
│   ├── room/
│   │   ├── roomList.tsx   # Room list page
│   │   ├── roomDetail.tsx # Room detail page
│   │   ├── components/    # memberItemButton, passwordModal, roomItem, userInfoPopover, memberSidebar
│   │   ├── hooks/         # useVoiceSession, useRoomAudioBridge, useRoomSounds
│   │   ├── services/      # loadSfuClient（provider chunk 预热/动态加载）
│   │   └── session/       # providers.ts, runVoiceJoin.ts, voiceSessionTypes.ts, voiceSessionUi.ts
│   ├── domain/            # CreateDomainModal, DomainIcon, InviteShareModal, GuestAccessSettings 等
│   ├── chat/              # chatPage, chatWindow, conversationList, memberSidebar；micControl / speakerControl 共用 audioControl.tsx 通用壳
│   ├── discover/          # 发现页（DiscoverDomainCard/Grid/JoinPanel/MyDomains）
│   ├── dashboard/         # Dashboard components
│   ├── manage/            # Admin management views
│   ├── modal/             # modals (search, settings with tabs)
│   ├── oauth/             # OAuth login views + ProviderIcon（品牌图标）
│   ├── profile/           # Profile views
│   ├── textRoom/          # Text room components
│   ├── home/              # Home page
│   └── userBar.tsx        # User status bar with audio controls
├── layouts/
│   ├── layout.tsx         # Root layout
│   └── common/            # header, main, sidebar, domainWorkspace
├── handler_audio/         # Audio handling
│   ├── index.ts           # 音频事件装配入口
│   ├── speakingStore.ts   # Speaking identities state
│   └── notificationSounds.ts # 通知音效
├── protocol/              # WS 协议类型（ws.ts, conversation.ts）
├── types/                 # Shared TypeScript types（message.ts）
├── utils/                 # Utility functions（errorReport.ts 统一错误上报：normalizeError / registerErrorReporter / reportError / errorSummary / installGlobalErrorHandlers）
├── main.tsx               # App entry point
├── styles.css             # Root CSS
└── routeTree.gen.ts       # TanStack Router generated route tree
```

**图标约定**：一律使用 `lucide-react` 按需导入；品牌图标（GitHub/Google/QQ 等）用 `components/oauth/ProviderIcon.tsx`（lucide 不提供品牌图标）。旧的雪碧图组件 `svgIcon.tsx` / `assets/svg.tsx` 已删除。

**圆角 token 约定**：圆角唯一来源是 `packages/components/src/theme.css` 的 `@theme` 圆角块（`--radius` 为基准 8px，其余档位由它派生）。业务代码只允许用 `rounded-*` 工具类，禁止 `rounded-[4px]` / `rounded-[var(--radius)]` 这类任意值写法，也禁止在 CSS 里写 `border-radius: 4px` 字面量；`pnpm lint:radius`（pre-push 已挂）会拦下违规。

| 档位 | 值 | 用途 |
|------|-----|------|
| `rounded-xs` | 2px | 极细：缩略图、分隔条、微型指示器 |
| `rounded-sm` | 4px | 控件：输入框、小按钮、徽章、列表内按钮 |
| `rounded-md` | 6px | 紧凑块：列表行、内嵌小面板 |
| `rounded-lg` | 8px | 默认：按钮、卡片、下拉面板、对话框内容块 |
| `rounded-xl` | 12px | 区块：管理页 section、设置面板 |
| `rounded-2xl` | 16px | 大容器：弹窗、页面级浮层 |
| `rounded-3xl` | 24px | 特大容器（极少使用） |
| `rounded-full` | 9999px | 胶囊：头像、状态点、标签、圆形按钮 |

语义别名与档位等值，按「用途」选型时优先用：`rounded-field`(=sm)、`rounded-card`(=lg)、`rounded-panel`(=xl)、`rounded-overlay`(=2xl)、`rounded-pill`(=full)。

### Web / 桌面能力边界与条件编译

桌面能力 SDK 在 `packages/desktop`（`@gospeak/desktop`）：桥接 MyGo 宿主（`window.mygo`）提供窗口/托盘/通知/自启/深链/对话框/更新/覆盖层等原生能力，每个能力都带浏览器降级实现。**能力边界规范**：

- **桌面独享能力**（窗口控制、托盘、自启、覆盖层、深链、原生对话框、更新器等）只允许通过 `@gospeak/desktop` 的封装调用，禁止在业务代码里直接摸 `window.mygo`。
- **共享能力**（网络、房间、消息、音频会话等）一律写成平台无关实现，Web 与桌面共用；SDK 内部负责降级，业务代码不感知差异。
- 桌面宿主（`app/desktop`，Go + MyGo）内嵌 Web 前端产物；**同一套前端代码，两种构建目标**：`web`（默认，桌面独享模块剔除）与 `desktop`（`GOSPEAK_TARGET=desktop`，桌面独享模块进包）。

**条件编译规范**（工具集在 `packages/desktop/src/conditional.ts`，详细用法见其 README）：

| 场景 | 用法 | 要求 |
|------|------|------|
| 桌面独享模块**不进 Web 包**（模块级裁剪） | 调用处直接写 token：`__GOSPEAK_DESKTOP__ ? A : B` 或 `if (__GOSPEAK_DESKTOP__) { ... }` | token 由 vite `define` 注入字面量，死分支连同 import 被摇树剔除；动态导入的 `import()` 必须写在死分支内，chunk 才不会产出 |
| UI 条件渲染 / 运行时行为分支 | `inDesktop()`（桌面构建恒 true；Web 构建回退运行时 `isDesktop()` 检测 `window.mygo`） | 兼容「Web 构建被桌面宿主打开」的 dev 场景 |
| 开发期守卫 | `assertDesktop(feature)` | Web 构建误用桌面能力时抛可读错误 |
| 分支取值 / 动态导入兜底 | `ifDesktop(d, w)` / `desktopOnly(d, fallback?)` / `webOnly(w, fallback?)` / `importDesktop(loader)` / `importWeb(loader)` | 死分支不执行（运行时安全）；但**不保证**死分支代码不进包，模块级裁剪必须用 token |
| 常量读取（非打包环境） | `BUILD_TARGET` / `DESKTOP_BUILD` / `WEB_BUILD` | tsc / vitest / Node 直跑源码时按 `web` 兜底 |

硬性规则：

1. **禁止再新增裸 `isDesktop()` 调用点**（历史调用点已迁移为 `inDesktop()`）；SDK 内部（`capabilities/*` 的浏览器降级）是唯一例外。
2. token 类型声明在 `packages/desktop/src/globals.d.ts` 与 `app/web/types/gospeak-build.d.ts`，两处必须保持一致；`app/web/vite.config.ts` 的 `define` 是唯一注入点（`GOSPEAK_TARGET` 环境变量驱动，默认 `web`）。
3. 桌面构建命令：`pnpm --filter @gospeak/web build:desktop`（构建）/ `dev:desktop`（dev）；desktop 包测试双目标跑：`pnpm --filter @gospeak/desktop test` + `GOSPEAK_TARGET=desktop pnpm --filter @gospeak/desktop test`。
4. 新增桌面独享模块时：模块本身放 `packages/desktop/src/capabilities/`（接口 + native + webFallback），业务侧用 token 裁剪引用点，确保 Web 构建产物中不出现该模块。
5. 桥接命令名必须是 `<Struct>.<Method>`（与 `mygo.Bind` 的类型名和方法名逐字一致，首字母大写），用 `@gospeak/desktop` 的 `command()` 构造，禁止手写字符串；新增或改名绑定方法后跑 `node scripts/check-desktop-commands.mjs`，CI 会拦下不匹配的调用。

**桌面窗口外壳（自定义标题栏）：**

- 主窗口用 `TitleBarStyle: mygo.TitleBarHidden`（`app/desktop/windows.go`，栏高 28px = Web 构建 `Header` 的高度）：系统窗口按钮保留（macOS 红绿灯 / Windows 11 按钮 / GTK 标题按钮），标题栏由 `app/web/src/components/titleBar/` 自绘，挂在根路由上，登录 / 注册 / 访客 / 邀请 / 应用内全部覆盖。
- 页面只允许通过 MyGo 注入的 `--mygo-titlebar-height` / `--mygo-titlebar-inset-left` / `--mygo-titlebar-inset-right` 给按钮让位，用 `--app-region: drag` 声明拖拽区；**禁止按平台写分支**，也禁止在业务页面里再画一条顶栏（Web 构建的 `Header` 已用 `__GOSPEAK_DESKTOP__` 排除）。
- 原生窗口标题固定为 `GOSpeak`（`Page.OnPageTitleUpdated` 阻止页面 `<title>` 改写），任务栏 / Mission Control 不跟随路由；路由标题只出现在应用内标题栏。
- 主题切换时调用 `createDesktop().window.setAppearance("light" | "dark")`，让原生按钮与菜单跟随；新增标题栏相关样式只加 `--mygo-titlebar-*` / `--app-region` 两类钩子。

### socketStore (`src/stores/socketStore.ts`)

zustand store（`useSocketStore` hook + `socketStore` 命令式门面），管理 WebSocket 连接、房间/成员状态与 SFU 事件订阅。组件通过 selector 订阅所需字段（如 `useSocketStore((s) => s.connected)`），避免无关重渲染。

**State 字段（selector 订阅）:**

| 字段 | Type | Description |
|--------|------|-------------|
| `connected` | `boolean` | WebSocket connection status |
| `connecting` | `boolean` | Connection in progress |
| `rooms` | `RoomInfo[]` | Live room list |
| `currentRoom` | `string \| null` | Room this client has joined |
| `members` | `MemberInfo[]` | Derived from rooms[currentRoom].members |
| `selectedRoomInfo` | `RoomInfo \| null` | Selected room info |
| `activeSFUProvider` | `SFUProvider \| undefined` | Current SFU provider |
| `speechRestricted` | `boolean` | User is muted by server |
| `speechRestrictionInfo` | `MuteInfo \| null` | Mute reason/expiry |

**Room APIs:**

| Method | Description |
|--------|-------------|
| `connect()` | Open WS connection (single-tab enforced via BroadcastChannel tabLock) |
| `disconnect()` | Close connection |
| `createRoom(name, password?)` | Emit `room:create` |
| `joinRoom(room, identity, password?, domain_uuid?)` | Emit `room:join` (await ack) |
| `joinRoomSFU(room, identity, stream?, domain_uuid?)` | Emit `room:join:sfu` (await ack) |
| `leaveRoom(room, domain_uuid?)` | Emit `room:leave` (await ack) |
| `listRooms()` | Emit `room:list` |
| `kickMember(room, targetIdentity)` | Emit `room:kick` |

**Event subscriptions:**

| Method | Description |
|--------|-------------|
| `onActivity(cb)` | Activity events (room_joined, member_joined, etc.) |
| `onPresence(cb)` | Presence events (member_joined, member_left) |
| `onRoomKicked(cb)` | Room kick events |
| `setPrivateNewHandler(cb)` | 私聊新消息推送处理 |

### WS Client (`src/socket/wsClient.ts`)

- 握手鉴权由 HttpOnly access cookie 自动携带；子协议只传协议名 `["gospeak"]`，不携带 token
- Auto-reconnect with exponential backoff + jitter
- Two emit modes: `emitFireAndForget` (no ack) and `emitAck` (returns Promise, 10s timeout)
- Server push events: `onServerEvent(event, cb)` returns unsubscribe function

### Voice Session Architecture

The voice session is coordinated through `providers.ts` / `runVoiceJoin.ts` / `voiceSessionTypes.ts`:

- `VoiceProviderAdapter` declares the join strategy:
  - `interactiveAfterMedia`: whether UI becomes interactive after media join
  - `signalJoinMode`: signaling join mode; LiveKit uses `await`
  - `serializeJoins`: whether same stream must be sequential
- `useVoiceSession.ts` is the **unified orchestrator** — do not branch per SFU in this file
- `loadSfuClient.ts` preloads the SFU client module
- `session/providers.ts` resolves provider → connect target; `session/runVoiceJoin.ts` owns the join/teardown lifecycle

### VoiceChat 加载时机

- VoiceChat 可交互展示的确认点 = **本端 media join 成功**（`client.joinRoom` 完成）
- 不要等 `room:join` / `room:join:sfu` 信令全完成才允许加载 VoiceChat
- LiveKit 通过 adapter 的 `interactiveAfterMedia` 声明该时机

---

## Testing

三层测试：

```bash
# Go 单元测试（app/server 内 pnpm test 即 go test ./...）
cd app/server && pnpm test
# 或从 monorepo 根：
pnpm test:server:go

# API 集成测试（顶层 test/，Vitest，先启动 server）
pnpm test:server        # 等价于 pnpm test:integration

# Web 单元测试（Vitest，当前 33 文件 / 200 用例全通过）
cd app/web && pnpm test
```

API 集成测试发送真实 HTTP 请求并校验响应，新增测试文件放 `test/<module>/<module>.test.ts`；Web 单测与源码同目录（`*.test.ts` / `*.spec.ts`）。

---

## Common Commands

根 package.json 的 `dev` / `build` / `test` / `lint` / `typecheck` / `check` 全部走 turbo 编排：

```bash
# Dev（turbo 并行起 server + web）
pnpm dev                     # = turbo run dev --filter=@gospeak/server --filter=@gospeak/web
pnpm dev:all                 # 起全部 dev 任务（含 components/sfu-client 的 tsc --watch）
pnpm dev:server              # 只起后端（air 热重载）
pnpm dev:web                 # 只起前端（vite）

# Build
pnpm build                   # turbo 全量构建，带缓存（命中时 FULL TURBO）
pnpm build:web               # = turbo run build --filter=@gospeak/web（含依赖包）
pnpm build:server            # Go 二进制，输出 app/server/gospeak
# 桌面构建（条件编译，GOSPEAK_TARGET=desktop；见「Web / 桌面能力边界与条件编译」）
pnpm --filter @gospeak/web build:desktop

# Test
pnpm test                    # turbo 跑全部单测（web / sfu-client / components / 集成）
pnpm test:web                # web 单测
pnpm test:server             # API 集成测试（test/，自动拉起临时 server）
pnpm test:server:go          # Go 单测（go test ./...）

# 质量
pnpm lint                    # turbo lint（biome check + go vet）
pnpm check                   # turbo check（biome check，不自动修复）
pnpm typecheck               # turbo typecheck（tsc --noEmit，按包并行）
pnpm format                  # biome format（递归）
pnpm changelog               # 生成 changelog
```

### Turbo 配置要点

- 根 `turbo.json` 只放公共任务定义；包级差异用 `app/*/turbo.json`、`packages/*/turbo.json`、`test/turbo.json` 的 `extends: ["//"]` 覆盖。
- `build` 走 `dependsOn: ["^build"]`，`@gospeak/web` 会先构建 `components` / `sfu-client`。
- `dev` 为 `persistent` + `cache: false`，且 `interruptible: true`（否则 `turbo watch` 无法重启）；因此其他任务不得 `dependsOn` `dev`。
- 各包 `outputs` 按实际产物声明：web `dist/**`、docs `.vitepress/dist/**`、server `gospeak`、sfu-client `dist/**`、components `[]`（`tsc` 为 `noEmit`，无产物，避免误报 "no output files found"）。
- `test` 无产物，`outputs` 为空；`test/turbo.json` 把 `app/server/**/*.go` 纳入 inputs（集成测试会自行编译 Go server），改后端代码会正确失效缓存。
- `globalDependencies` 用 `app/*/tsconfig*.json` 这类浅层 glob，不要用 `**/tsconfig*.json`：后者会扫进 `.worktrees/`，导致同一仓库的多 worktree 互相污染缓存。
- 包级 `turbo.json` 同样被 biome 扫描，缩进必须与仓库一致（tab）。

---

## Code Style Guidelines

1. **No comments in code** unless absolutely necessary for complex logic
2. **No emoji in code** — only in documentation files
3. **Error handling**: Always return `*AppError` from service layer, let handlers decide response format
4. **Naming**: Go files use `snake_case` (e.g. `auth_service.go`), Go types use `PascalCase`
5. **Imports**: Group standard library, third-party, internal packages with blank lines
6. **Handler methods**: Accept `*gin.Context`, return nothing
7. **Service methods**: Return `(result, error)` — error is always `*AppError`
8. **Repository methods**: Return `(result, error)` — error may be `gorm.ErrRecordNotFound`

---

## Mute vs Restrict Speech

- **`禁言`** means **user-level speech restriction**. The restricted user may still listen, but must not publish a local audio track.
- **`静音`** (client-local playback mute) is out of scope for server state modeling.
- There is **no room-level mute** concept.
- User-level restriction is represented by `user:muted` / `user:unmuted`, backend mute records, and join/publish checks.
- The enforcement level (`hard` / `degraded` / `soft`) determines how forcefully the media layer is stopped.

---

## Adding a New Feature (Example Flow)

1. Define model in `internal/model/`
2. Add repository methods in `internal/repository/`
3. Add business logic in `internal/service/`
4. Add HTTP handler in `internal/handler/`
5. Create route file in `internal/router/routes/<module>/routes.go`
6. Register route group in `internal/router/router.go`
7. Wire dependencies in `server/gin.go`
8. Add tests（Go 单测放 app/server 同包 `*_test.go`，API 集成测试放 `test/<module>/<module>.test.ts`）
9. Regenerate Swagger docs（`app/server/docs/`）

---

## Adding a New SFU Backend

1. Create `internal/sfu/providers/<provider>/` implementing `sfu.Provider`
2. Add `CapabilitiesFor("<provider>")` case in `internal/sfu/capabilities.go`
3. Add a case in `internal/sfu/factory/factory.go` for the new `SFU_PROVIDER` value
4. Add provider-specific config fields to `internal/config/config.go` and `model.SFUConfig`
5. Implement `ProviderName()`, `Capabilities()`, and all interface methods
   - Unsupported operations MUST return `pkg.NewErrSFUNotSupported()`
   - Missing configuration should return `SFU_NOT_CONFIGURED`
6. Add `VoiceProviderAdapter` in the frontend `providers.ts` for UI integration
7. Set `SFU_PROVIDER="<provider>"` in `.env.dev` or update via `/api/v1/sfu/config`

---

## Test Logging

当 agent 被命令进行测试时，必须将测试总结的结果以 Markdown 格式保存到 `agent_test_logs` 文件夹。

### 命名规范

文件名格式：`{测试内容}-{时间}.md`

示例：
- `api-auth-test-2026-05-26.md` - 认证 API 测试
- `role-permission-test-2026-05-26.md` - 角色权限测试
- `signal-websocket-test-2026-05-26.md` - WebSocket 信令测试

### 测试状态标识

- ✅ 通过 - 测试成功
- ❌ 失败 - 测试失败
- ⚠️ 警告 - 测试通过但存在问题
- ⏭️ 跳过 - 测试被跳过

### 保存位置

```
GOSpeak/
├── agent_test_logs/           # 测试日志目录
│   ├── AGENTS.md              # 测试规范
│   ├── api-auth-test-2026-05-26.md
│   └── ...
└── ...
```


---

## Guest Access（访客访问）

- 设计规格：`docs/superpowers/specs/2026-08-25-guest-access-permissions-design.md`
- 实现计划：`docs/superpowers/plans/2026-08-25-guest-access-permissions.md`
- 访客 = `users.is_guest=true` + `DomainMember(role=guest)`；能力开关落 `domains` 表（听/说默认开，发消息默认关）；封禁表 `domain_guest_bans`
- 守卫：`middleware.GuestGuard()` 挂在 protected 组（JWT 之后），白名单外接口对访客返回 1013
