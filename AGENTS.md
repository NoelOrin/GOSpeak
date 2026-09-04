# GOSpeak Monorepo — AI Agent Guide

## Project Overview

GOSpeak 是一个自托管的**游戏语音平台**，类似自部署版 Discord 语音房间。基于 WebRTC，pnpm monorepo：Go 后端 + SolidJS 前端，多 SFU Provider 抽象。

**为什么自建？** 游戏语音数据不经第三方、自定义语音路由策略、无用户数限制、完全控制部署架构。

关键能力：
- 语音房间管理（创建/加入/密码保护/踢人）
- 实时发言检测 + 成员独立音量控制
- 多 SFU 运行时切换（LiveKit / SRS / Agora / Cloudflare）
- 渐进式数据库（SQLite 开箱即用 → PostgreSQL → MySQL → Turso 远程库）
- JWT + OAuth2 三端登录（GitHub / Google / QQ），HttpOnly Cookie 会话 + 惰性续期
- Domain（语音服务器）多租户隔离 + 访客访问体系
- 文字房间消息（房间消息 + 私聊）
- 插件系统（内置 Bot + 外部插件）
- 多实例部署（NATS 事件总线 + JetStream KV 状态共享）
- 集群编排（Agent 控制面 / Worker 数据面，节点注册与心跳）

```
GOSpeak/
├── app/
│   ├── server/          # Go 后端 (Gin + GORM + multi-provider SFU abstraction)
│   ├── web/             # SolidJS 前端 (TypeScript + Vite + TanStack Router)
│   └── docs/            # 文档站 (architecture / deployment / guide / sfu)
├── packages/
│   ├── bot/             # Bot 运行时与插件框架
│   ├── mediasoup-worker/ # MediaSoup Worker 独立进程（保留，当前未接入）
│   └── sfu-client/      # 共享 SFU 客户端类型与工厂
├── test/                # API 集成测试 (Vitest + Node)
├── docs/                # 设计/运维文档（design、review、superpowers specs/plans）
├── deploy/              # docker-compose、nginx、livekit/srs/mediasoup 配置、observability
├── scripts/             # changelog 生成等辅助脚本
├── .github/             # CI workflows（build/ci/release-please 等）+ dependabot
├── agent_test_logs/     # Agent 测试日志输出目录
├── pnpm-workspace.yaml  # Workspace 配置
├── turbo.json           # 任务编排（dev/build/lint 走 turbo）
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
│   └── control_plane_bootstrap.go # Agent 控制面启动装配（种子、fence、插件等）
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
│   │   └── providers/      # Per-provider implementations
│   │       ├── livekit/    # LiveKit gRPC SDK
│   │       ├── srs/        # SRS REST + WHIP/WHEP
│   │       ├── agora/      # Agora REST
│   │       └── cloudflare/ # Cloudflare Realtime (WHIP/WHEP)
│   ├── signal/             # WebSocket signaling hub
│   │   ├── events.go       # 46 event name constants
│   │   ├── types.go        # RoomRequest, MemberInfo, RoomInfo
│   │   ├── hub.go          # Hub (room registry + all event handlers)
│   │   ├── state_sync.go   # Domain 命名空间 + 跨实例状态同步
│   │   ├── bot_bridge.go   # Bot 消息桥接
│   │   └── message_bridge.go # 消息事件桥接
│   ├── ws/                 # WebSocket 基础设施（nhooyr.io/websocket）
│   │   ├── client.go       # Client (read loop, goroutine-safe write)
│   │   ├── fanout.go       # Fanout Broadcaster (room-based fan-out)
│   │   ├── upgrader.go     # HTTP→WS upgrade + access cookie 鉴权 + origin 校验
│   │   └── handler.go      # HandlerRegistry (event dispatch + panic recover)
│   ├── bus/                # Multi-instance event bus (NATS embedded/external)
│   │   ├── bus.go          # EventBus interface + Envelope
│   │   ├── embedded.go     # Embedded NATS server
│   │   ├── nats_bus.go     # External NATS
│   │   ├── ws_deliverer.go # NATS → local WS bridge
│   │   ├── membership_store.go  # Room member 跨实例共享
│   │   ├── mute_rule_store.go   # Agora kicking-rule + 降级 mute cache
│   │   ├── auth_store.go   # JWT blacklist + signing key rotation
│   │   └── factory.go      # Init() resolves embedded/external
│   ├── authstate/          # JWT auth state (blacklist, key rotation; NATS KV/memory)
│   ├── logger/             # Unified logrus wrapper (level/format/output + gin middleware)
│   ├── plugin/             # Plugin system (registry, host, builtin)
│   │   ├── types.go        # Plugin/Host/Meta/Configurable interfaces
│   │   ├── registry.go     # Plugin lifecycle management
│   │   ├── host.go         # Host implementation (DB, config, HTTP registration)
│   │   └── builtin/        # Built-in plugins (botbase)
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
- **Bus** is a multi-instance event bus (NATS) for cross-instance room fanout, membership, mute rules, and auth stores

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
- Grouped by module: `auth`, `guest`, `user`, `user-group`, `signal`, `oauth`, `role`, `permission`, `mute`, `room`, `message`(挂 `/room`), `storage`, `bot`, `email`, `email_config`, `sfu`, `srs`, `system`, `domain`, `conversation`, `cluster`, `audit`, `plugin`
- The `protected` group applies `middleware.JWTAuth()` + `middleware.BanCheck()` + `middleware.GuestGuard()` to every route；配置了 Agent 写面 fence 时（外置总线）再挂 `middleware.RequireAgentFence()`
- Permission-gated routes additionally use `middleware.RequirePermission(permcode.X)` (permission-based RBAC)
- Public routes (login/register/refresh_token/reset_password、guest 签发、email 验证码、oauth、signal exchange、srs callback) are outside the protected group；登录/注册/重置 10 次/分、refresh_token 60 次/分、email 验证码 5 次/分、guest 签发 60 次/时限流
- Bot tokens use `Claims.Permissions` directly（外加 DB `Revoked/ExpiresAt` 双源校验）; users resolve permissions via `role → role_permissions`
- `GOSPEAK_ROLE=worker` 时仅注册 signal protected + system 子集；未注册的业务写路径由 fallback 拦截（写 403 / 读 404）

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
| POST | `/api/v1/user/upload-avatar` | JWT | — | UserHandler.UploadAvatar |
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
| POST | `/api/v1/signal/signal` | No | — | SignalHandler.Signal |
| GET | `/api/v1/signal/rooms` | JWT | — | SignalHandler.ListRooms |
| GET | `/api/v1/signal/participants` | JWT | — | SignalHandler.ListParticipants |
| POST | `/api/v1/signal/webhook` | No | — | SignalHandler.LivekitWebhook |
| POST | `/api/v1/signal/cloudflare/sessions/:sessionId/tracks/new` | JWT | — | CloudflareHandler.AddTracks |
| PUT | `/api/v1/signal/cloudflare/sessions/:sessionId/renegotiate` | JWT | — | CloudflareHandler.Renegotiate |
| PUT | `/api/v1/signal/cloudflare/sessions/:sessionId/tracks/close` | JWT | — | CloudflareHandler.CloseTracks |
| DELETE | `/api/v1/signal/cloudflare/sessions/:sessionId` | JWT | — | CloudflareHandler.DeleteSession |
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
| POST | `/api/v1/room/messages/send` | JWT | — | MessageHandler.Send |
| POST | `/api/v1/room/messages/edit` | JWT | — | MessageHandler.Edit |
| POST | `/api/v1/room/messages/delete` | JWT | — | MessageHandler.Delete |
| POST | `/api/v1/room/messages/react` | JWT | — | MessageHandler.React |
| POST | `/api/v1/room/messages/unreact` | JWT | — | MessageHandler.Unreact |
| POST | `/api/v1/storage/presign` | JWT | — | StorageHandler.PresignUpload |
| POST | `/api/v1/storage/confirm` | JWT | — | StorageHandler.ConfirmUpload |
| POST | `/api/v1/storage/upload` | JWT | — | StorageHandler.Upload |
| POST | `/api/v1/storage/delete` | JWT | `storage:delete` | StorageHandler.DeleteObject |
| POST | `/api/v1/storage/config` | JWT | `storage:read` | StorageHandler.GetConfig |
| POST | `/api/v1/storage/update-config` | JWT | `storage:manage` | StorageHandler.UpdateConfig |
| POST | `/api/v1/storage/test-config` | JWT | `storage:read` | StorageHandler.TestConfig |
| POST | `/api/v1/bot/create` | JWT | `bot:manage` | BotHandler.Create |
| POST | `/api/v1/bot/list` | JWT | `bot:manage` | BotHandler.List |
| POST | `/api/v1/bot/revoke` | JWT | `bot:manage` | BotHandler.Revoke |
| POST | `/api/v1/email/send_code` | No（限流 5/min） | — | EmailVerificationHandler.SendCode |
| POST | `/api/v1/email/verify_code` | No（限流 5/min） | — | EmailVerificationHandler.VerifyCode |
| POST | `/api/v1/email/config` | JWT | `email_config:read` | EmailConfigHandler.Get |
| POST | `/api/v1/email/update-config` | JWT | `email_config:manage` | EmailConfigHandler.Update |
| POST | `/api/v1/sfu/config` | JWT | `sfu:manage` | SFUConfigHandler.Get |
| POST | `/api/v1/sfu/config/:provider` | JWT | `sfu:manage` | SFUConfigHandler.GetProvider |
| POST | `/api/v1/sfu/update-config` | JWT | `sfu:manage` | SFUConfigHandler.Update |
| POST | `/api/v1/sfu/switch-provider` | JWT | `sfu:manage` | SFUConfigHandler.SwitchProvider |
| POST | `/api/v1/sfu/providers` | JWT | `sfu:manage` | SFUConfigHandler.ListProviders |
| POST | `/api/v1/srs/callback` | No | — | SRSCallbackHandler.HandleCallback |
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
| POST | `/api/v1/cluster/nodes/register` | JWT | `cluster:manage` | ClusterHandler.Register |
| POST | `/api/v1/cluster/nodes/heartbeat` | JWT | `cluster:manage` | ClusterHandler.Heartbeat |
| POST | `/api/v1/cluster/nodes/deregister` | JWT | `cluster:manage` | ClusterHandler.Deregister |
| POST | `/api/v1/cluster/nodes/drain` | JWT | `cluster:manage` | ClusterHandler.Drain |
| POST | `/api/v1/cluster/nodes/undrain` | JWT | `cluster:manage` | ClusterHandler.Undrain |
| POST | `/api/v1/cluster/nodes/list` | JWT | `cluster:read` | ClusterHandler.List |
| POST | `/api/v1/cluster/stats` | JWT | `cluster:read` | ClusterHandler.Stats |
| POST | `/api/v1/cluster/servers/scale` | JWT | `cluster:manage` | ClusterHandler.Scale |
| POST | `/api/v1/cluster/servers/resolve` | JWT | `cluster:read` | ClusterHandler.Resolve |
| POST | `/api/v1/cluster/servers/list` | JWT | `cluster:read` | ClusterHandler.ListAssignments |
| POST | `/api/v1/cluster/servers/drain` | JWT | `cluster:manage` | ClusterHandler.DrainServer |
| POST | `/api/v1/cluster/servers/autoscale` | JWT | `cluster:manage` | ClusterHandler.AutoScaleServer |
| POST | `/api/v1/audit/list` | JWT | `audit:read` | AuditHandler.List |
| POST | `/api/v1/plugins/list` | JWT | `plugin:read` | PluginHandler.List |
| POST | `/api/v1/plugins/get` | JWT | `plugin:read` | PluginHandler.Get |
| POST | `/api/v1/plugins/update` | JWT | `plugin:manage` | PluginHandler.Update |
| * | `/api/v1/plugins/:name/*` | JWT | `plugin:manage` | 插件自定义路由（PluginHost.MountRoutes） |
| WS | `/ws` | Cookie（access token） | — | WebSocket signaling（子协议仅传协议名 `gospeak`，不携带 token） |
| GET | `/ping` | No | — | Health check |
| GET | `/readyz` | No | — | Readiness probe（探测 DB 等依赖） |
| GET | `/swagger/*any` | No | — | Swagger UI |
| GET | `/uploads/*filepath` | No | — | Static avatar/uploads (safe path traversal) |

> SPA 托管：`STATIC_DIR` > `/app/static` > `./static` > go:embed 内嵌资源，由 NoRoute fallback 提供。

---

## Configuration

All configuration is injected via environment variables (`APP_ENV` selects dev/prod；`.env.dev` for dev, `deploy/env/app.*.env` for Docker). Startup loads env files without overriding process env (`process > .env.<env> > .env > defaults`), then `config.Load()` parses into a typed `Config` via `caarlos0/env`, normalizes aliases (e.g. `PostgresSQL` → `PostgreSQL`), validates. 配置已通过 `server/gin.go` 装配时以显式 `config.Config` 注入到各内部包；全局 `config.Current()` 当前代码库无任何包使用，保留仅为历史兼容占位，新代码不要依赖它。

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
| `DB_READ_DSN` | — | Worker 只读副本 DSN（优先于 `DB_READ_*` 字段）|
| `DB_READ_HOST` / `DB_READ_PORT` / `DB_READ_USER` / `DB_READ_PASSWORD` / `DB_READ_DBNAME` | 回退主库字段 | Worker 只读副本连接参数 |
| `DB_READ_ONLY` | `true` | Worker 会话强制只读（PG `default_transaction_read_only`，MySQL `transaction_read_only`）|
| `DB_REPLICA_LAG_THRESHOLD` | `5s` | 只读副本延迟降级阈值 |

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

### SFU Provider

`SFU_PROVIDER` selects the active backend: `livekit` (default), `srs`, `agora`, `cloudflare`. Provider-specific config:

| Variable | Default | Description |
|----------|---------|-------------|
| `LIVEKIT_HOST` / `LIVEKIT_KEY` / `LIVEKIT_SECRET` | — | LiveKit server URL, API key/secret |
| `AGORA_APP_ID` / `AGORA_APP_CERTIFICATE` / `AGORA_HOST` / `AGORA_CUSTOMER_ID` / `AGORA_CUSTOMER_SECRET` | — | Agora credentials |
| `SRS_HOST` / `SRS_API_PORT` | `localhost` / `1985` | SRS management API |
| `SRS_WHIP_URL` | `/rtc/v1/whip/` | SRS WHIP endpoint path |
| `SRS_SECRET` | — | SRS stream/room token HMAC key (required) |
| `SRS_PUBLIC_HOST` | — | Browser-side serverUrl prefix |
| `CF_APP_ID` / `CF_APP_SECRET` / `CF_STUN_URL` | — / — / `stun.cloudflare.com:3478` | Cloudflare Realtime credentials |

> MediaSoup / Daily 已禁用保留：实现文件仍在仓库（`MEDIASOUP_BRIDGE_URL`、`MEDIASOUP_HOST`、`DAILY_API_KEY`、`DAILY_DOMAIN`），但 `SFU_PROVIDER` 不再接受这两个值。

> Base SFU config is loaded from env; persistent per-provider config is managed through `/api/v1/sfu/*` and resolved at runtime by `factory.NewDynamicProvider(...)`.

### NATS / Bus (Multi-Instance)

| Variable | Default | Description |
|----------|---------|-------------|
| `NATS_URL` | — | Leave empty to use embedded NATS |
| `NATS_SUBJECT_PREFIX` | `gospeak` | NATS subject prefix |
| `NATS_NAME` | — | Instance name (auto = hostname-pid) |
| `NATS_CONNECT_TIMEOUT` | `2s` | Connection timeout |
| `NATS_EMBEDDED_PORT` | — | Embedded NATS port (random if empty) |
| `NATS_USER` / `NATS_PASSWORD` / `NATS_TOKEN` | — | Auth credentials |
| `NATS_CREDS_FILE` | — | NATS JWT creds file |
| `NATS_TLS` | `false` | Enable TLS |
| `NATS_WAL_PATH` | — | 断线事件磁盘 WAL 路径（空 = 禁用持久化） |
| `STATE_STORE` | `auto` | NATS 层 KV store 按需降级（见下方「Bus 模式」）；Bus 连接模式为硬判定 |

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
| `STORAGE_PATH_PREFIX` | `uploads/` | Upload path prefix |
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

### Cluster（Agent/Worker 编排）

| Variable | Default | Description |
|----------|---------|-------------|
| `GOSPEAK_ROLE` | `all` | `all` / `agent`（控制面）/ `worker`（数据面） |
| `CLUSTER_NODE_ID` / `CLUSTER_ADVERTISE_URL` | — | 节点标识与对外地址 |
| `CLUSTER_AGENT_URL` / `CLUSTER_AGENT_TOKEN` / `CLUSTER_NODE_SECRET` | — | Worker→Agent 上报通道 |
| `CLUSTER_HEARTBEAT_INTERVAL` / `CLUSTER_HEARTBEAT_TIMEOUT` | `5s` / `30s` | 心跳间隔与离线判定 |
| `CLUSTER_MAX_SERVERS` / `CLUSTER_MAX_ROOMS` | `100` / `1000` | 节点容量上限 |
| `CLUSTER_LABELS` / `CLUSTER_ENTRY_URL` | — | 节点标签 / 入口地址 |
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

// StreamProvider — SRS WHIP/WHEP uses stream-based addressing
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
| `degraded` | Substitute media force (e.g. SRS KickByStreams, Agora kicking-rule) |
| `soft` | Signal/policy + client cooperation only |
| `none` | Capability absent |

`sfu.AllProviderCapabilities()` returns the full matrix for all providers (`livekit` / `srs` / `agora` / `cloudflare`，另含保留未接入的 `daily` / `mediasoup`). `CapabilitiesFor(name)` returns a single provider's matrix.

### Provider Capability Summary

| Provider | ServerMute | ServerKick | MuteLevel | KickLevel |
|----------|-----------|-----------|-----------|-----------|
| LiveKit | ✅ | ✅ | hard | hard |
| SRS | ✅ | ✅ | degraded | hard |
| Cloudflare | ✅ | ✅ | degraded | hard |
| Agora | ✅ | ✅ | degraded | degraded |

### Factory (`internal/sfu/factory/`)

`factory.NewProvider(cfg)` reads `cfg.SFUProvider` and returns the matching implementation. Registered providers: `livekit`, `agora`, `srs`, `cloudflare`.

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

## Bus Module (Multi-Instance Event Bus)

Multi-instance event bus and shared state at `internal/bus/`. Provides cross-instance coordination for WebSocket fanout, membership sharing, mute rules, JWT auth, and async job queues.

### Capabilities

| Capability | Backend | Purpose |
|------------|---------|---------|
| EventBus | NATS (embedded/external) | WebSocket cross-instance fanout (room/namespace) + internal events |
| MembershipStore | NATS KV → none | Room member / stream mapping sharing |
| MuteRuleStore | NATS KV → memory | Agora kicking-rule id and degraded mute cache |
| AuthStore | NATS KV | JWT blacklist + signing key rotation |
| JobQueue | NATS JetStream | SFU cleanup and async tasks |

### Init API

```go
bus, cleanup, err := bus.Init(bus.InitConfig{
    URL:            natsURL,       // empty = embedded NATS
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

- `STATE_STORE=auto` 仅表示 NATS 层（Membership/MuteRule/Auth 等 KV store）在无共享后端时按各自 fallback 降级（内存 / 静态 `JWT_KEY`），不影响 Bus 连接模式本身。
- ⚠️ Bus 连接模式为硬判定，不是 graceful degradation：`NATS_URL` 为空才用内嵌 NATS；一旦显式配置 `NATS_URL`，`bus/factory.go` 会先探测，探测失败直接失败（进程 panic 退出），**不会**回退到内嵌 NATS。部署时务必保证外部 NATS 可达，或保持 `NATS_URL` 为空走内嵌路径。
- `MuteRule`: final fallback is in-memory (never blocks startup)
- `Auth`: NATS KV first; static `JWT_KEY` last

### WSDeliverer

`internal/bus/ws_deliverer.go` bridges NATS fanout into the local WebSocket server, so cross-instance messages reach all connected clients.

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
- token 必须为 access 类型（refresh/bot 类型拒绝）。

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
| `bot_bridge.go` | Bot command/message bridge |
| `message_bridge.go` | Message events bridge |
| `private_bridge.go` | Private message bridge |

### Hub struct

```go
type Hub struct {
    sfuProvider      sfu.Provider
    sfuSignalHandler SFUSignalHandler  // optional: provider-specific media negotiation events
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
| srs | ✅ SRS REST `KickByStreams` | 完整实现 |
| agora | ✅ Agora kicking-rule（join_channel，degraded） | 完整实现 |
| cloudflare | ✅ DeleteSession（整会话踢除） | 完整实现 |

### Mute/ListParticipants 分工

| 操作 | 信令层 | SFU 层 | 说明 |
|------|--------|--------|------|
| `RemoveParticipant` | ✅ 删 Members + 广播 | ✅ 按 provider 能力 | |
| `ListRooms` + `ListParticipants` | 失败时返回 `[]` | ✅ 有则返回 | SFU 媒体状态 ≠ 信令层在线状态，不 fallback |
| `MuteParticipant` | `BroadcastMute` 广播 | ✅ 按 provider 能力调用 | hard/degraded 媒体强制；不支持时 soft 兜底并依赖前端停推流 |

### 发言检测

无 SFU 原生 active speaker 的 provider（SRS / Cloudflare）：
- 前端 `onLocalSpeakingChange` 上报「自身」本地麦克风音量状态，经 `member:speaking` 发往信令层
- `Hub.OnMemberSpeaking` 按房间聚合 `Room.Speaking`，广播 `room:active-speakers`（identities 列表）
- LiveKit / Agora 仍由各自 SFU 原生事件驱动，不经此链路
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
| `VerifyToken(tokenStr)` | Shared token verification: signature → token 类型白名单 → UserUUID 非空 → expiry → blacklist → version（→ bot token 额外 DB 校验）. Returns `*Claims, ErrCode`. Used by both HTTP and WS paths. |
| `JWTAuth()` | Validates Bearer token（缺失时回退 HttpOnly access cookie）, injects context |
| `RequireRole(roles ...string)` | Legacy role check against the `role` claim |
| `RequirePermission(permCode)` | Permission-based gate. Bot tokens use `Claims.Permissions`; users map `role` → permissions |
| `RequireOwnerOrPermission(ownerContextKey, permCode)` | Allows if caller owns the resource or holds the permission |
| `PermissionGranted(claims, role, permCode, checker)` | Unified permission check for HTTP and WS paths |
| `BanCheck()` | Blocks any user whose `role` is `ban`; returns `USER_BANNED` (1015) |
| `GuestGuard()` / `GuestGuardWith(whitelist)` | 访客守卫：白名单外接口对 `is_guest` 用户返回 1013（挂在 protected 组全局生效） |
| `RequireDomainMember()`（及 IfProvided 变体） | 校验当前用户是目标 Domain 成员 |
| `RequirePlatformAdminOrDomainMember()`（及 IfProvided 变体） | 平台管理员或域成员放行 |
| `RequireAgentFence(checker)` | Agent 写面 fence：写请求先校验 DB 写面归属（仅外置总线时挂载，防多 Agent 互抢写面） |
| `RateLimit(max, window)` | 固定窗口限流（登录/注册/验证码/guest 签发等） |
| `RequestID()` | 请求 ID 注入 |
| `CORS(origins []string)` | 基于显式白名单的 CORS（`*` 放行任意来源），处理 OPTIONS preflight |

### JWTAuth check order

1. `Authorization` header；缺失时回退 HttpOnly access cookie（默认 `gospeak_token`）→ 都没有则 `TOKEN_NOT_EXIST` (1001)
2. 签名合法 → `TOKEN_WRONG` (1002)
3. token 类型在白名单内（access / bot）且 UserUUID 非空 → 否则 `TOKEN_WRONG` (1002)
4. 未过期 → `TOKEN_EXPIRED` (1003)
5. JTI 不在黑名单 → `TOKEN_REVOKED` (1014)
6. `TokenVersion` 匹配当前用户 → 否则 `TOKEN_REVOKED` (1014)
7. Bot token 额外通过 DB `IsBotTokenValid`（Revoked / ExpiresAt）双源校验 → 失败 `TOKEN_REVOKED` (1014)
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
| `bot:manage` | 创建、查看、吊销 BOT 专用 API Key |
| `email_config:read` | 查看 SMTP 配置 |
| `email_config:manage` | 修改 SMTP 配置 |
| `storage:read` | 查看存储配置 |
| `storage:manage` | 修改存储配置 |
| `storage:delete` | 删除存储对象 |
| `oauth:read` | 查看 OAuth 配置 |
| `oauth:manage` | 管理 OAuth 配置 |
| `plugin:read` | 查看插件列表与配置 |
| `plugin:manage` | 启用/停用插件 |
| `message:send` | 发送消息 |
| `message:read` | 查看历史消息 |
| `message:delete_others` | 删除他人消息 |
| `cluster:read` | 查看集群节点与分配 |
| `cluster:manage` | 集群节点注册/心跳/排水/扩缩容 |
| `audit:read` | 查询审计日志（注意：未进入默认权限种子，需手工授予） |

### Bot Scoped Permissions

Bot tokens carry an explicit `permissions` list in the JWT (`Claims.Permissions`). Only the following are whitelisted for bots:

```go
var BotScopedPermissions = []string{
    "room:read", "user:read", "signal:kick",
    "room:create", "mute:manage",
    "message:send", "message:read",
}
```

### Default Role Permissions

| Role | Permissions |
|------|------------|
| `admin` | 显式枚举全部平台权限（含 `cluster:read` / `cluster:manage`；不含 `audit:read`） |
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
| `ClusterNode` | `cluster_nodes` | NodeID, Role, Status (pending/ready/busy/draining/offline/unhealthy), 心跳与容量字段, timestamps |
| `ServerAssignment` | `server_assignments` | Domain/Server 分配状态（assigned/draining），timestamps |
| `ClusterLeaderFence` | `cluster_leader_fences` | Agent leader 写面 fence 记录 |
| `Message` | `messages` | UUID, RoomUUID, AuthorID, AuthorUUID, Content, ReplyTo, ConversationType (`room`/`private`), ConversationID, TargetIdentity, EditedAt, Status (`active`/`deleted`), DeletedAt (软删), timestamps |
| `MessageMention` | `message_mentions` | MessageUUID, UserID |
| `MessageReaction` | `message_reactions` | MessageUUID, UserID, Emoji (unique per user+message+emoji) |
| `ConversationParticipant` | `conversation_participants` | ConversationID (PK), IdentityA/IdentityB（字典序）, LastMessageID/LastContent/LastSenderIdentity/LastMessageAt, UnreadCountA/B, timestamps |
| `OAuthProvider` | `oauth_providers` | ID, Name, DisplayName, IconURL, ClientID, ClientSecret, AuthURL, TokenURL, UserInfoURL, RedirectURL, Scopes, field mappings, Enabled, timestamps |
| `OAuthAccount` | `oauth_accounts` | ID, UserID, Provider, ProviderUID; AccessToken & RefreshToken stored but hidden (`json:"-"`), timestamps |
| `BotToken` | `bot_tokens` | ID, UUID, Name, UserUUID, Permissions `[]string`, Revoked, ExpiresAt, timestamps |
| `EmailConfig` | `email_configs` | ID, Enabled, SMTP host/port/user/pass/from/name, EmailCodeTTL, EmailSendCooldown, EmailCodeSecret (hidden, `json:"-"`), timestamps |
| `EmailVerificationCode` | `email_verification_codes` | ID, Email, Scene, CodeHash (hidden), UserID, IPAddress, ExpiresAt, UsedAt, AttemptCount, timestamps |
| `StorageConfig` | `storage_configs` | ID, ProviderType (`local`/`s3`), Endpoint, Bucket, Region, AccessKey & SecretKey (hidden), PublicBaseURL, PathPrefix, MaxFileSize, AllowedTypes, timestamps |
| `SFUConfig` | `sfu_configs` | Provider (PK), per-provider config fields, timestamps |
| `SFUActiveProvider` | `sfu_active_provider` | ID, Provider |
| `PluginConfig` | `plugin_configs` | Name, Enabled, Config JSON |

> Auto-migration (`repository/db_migrations.go`) syncs all of the above on startup.

---

## Plugin System (`internal/plugin/`)

Provides a plugin architecture for extending backend functionality.

### Core interfaces

```go
type Plugin interface {
    Meta() Meta
    Init(host Host) error   // inject dependencies, no service start
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type Host interface {
    Logger(component string) *logrus.Entry
    DB() HostDB                                  // 受限接口（Ping/Stats），刻意不暴露 *gorm.DB
    AppConfig() *config.Config
    RegisterHTTP(fn func(r *gin.RouterGroup))      // /api/v1/plugins/:name/*
    StartSideServer(name, addr string, handler http.Handler) (SideServer, error)
    LoadConfig/SaveConfig(pluginName string) (enabled bool, cfg map[string]any, err error)
}

type Configurable interface {
    ValidateConfig(raw map[string]any) (map[string]any, error)
    OnConfigUpdated(cfg map[string]any) error  // optional hot-reload
}
```

### Built-in plugins

- `builtin/botbase`: Base framework for text/voice interaction bots. Provides embed generation, command dispatch, and message sending helpers.

### Registration

Plugins are registered via `plugin.Registry.Register(p Plugin)`. The server initializes all registered plugins in `server/gin.go` after DB setup. Plugins can mount HTTP routes under `/api/v1/plugins/:name/*` via `Host.RegisterHTTP()`（该组统一要求 `plugin:manage` 权限）。

---

## Frontend Architecture

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
│   ├── plugin.ts       # Plugin list/update
│   ├── storage.ts      # Storage presign/confirm
│   ├── email.ts        # Email send/verify
│   ├── userGroup.ts    # 用户分组
│   ├── cluster.ts      # 集群控制面
│   └── apikey.ts       # Bot token management
├── socket/             # WebSocket client
│   ├── wsClient.ts     # WebSocket connection manager (reconnect, ACK, fire-and-forget)
│   ├── events.ts       # EVENTS 常量对象（42 个事件键）
│   ├── socketEvents.ts # 服务端推送事件绑定
│   ├── types.ts        # RoomInfo, MemberInfo, MuteEvent, etc.
│   ├── roomState.ts    # Room list/members state merge helpers
│   ├── providerReload.ts   # SFU hot-swap handler (disconnect + reload)
│   ├── mediasoupSignal.ts  # mediasoup 信令绑定（保留，当前未接入）
│   ├── tabLock.ts      # Single-tab socket owner (BroadcastChannel)
│   └── *.test.ts       # Unit tests
├── stores/             # SolidJS reactive stores (IndexedDB persistence)
│   ├── socketStore.ts  # WebSocket connection + room/member state + SFU events
│   ├── userStore.ts    # Auth state (persisted to IndexedDB)
│   ├── guestStore.ts   # 访客能力（listen/speak/message）
│   ├── themeStore.ts   # Light/dark theme (localStorage)
│   ├── audioDeviceStore.ts  # Audio device enumeration (IndexedDB)
│   ├── voiceChatStore.ts    # Mute/volume state (IndexedDB)
│   ├── chatStore.ts    # Chat/conversation state (IndexedDB)
│   └── domainStore.ts # Domain state (内存 store)
├── pages/              # TanStack Router 文件路由：(app)/ index、chat、discover、domain、link、manage、profile；login/、register/、guest/、invite/
├── hooks/              # Reusable SolidJS hooks
│   ├── media.ts        # Audio device enumeration utility
│   ├── useBreakpoint.ts
│   ├── useTitle.ts
│   └── useUpload.ts
├── components/
│   ├── common/         # Shared UI (Avatar, Modal, Divider, Toast, etc.)
│   ├── form/           # Form / PasswordChangeForm
│   ├── room/
│   │   ├── roomList.tsx   # Room list page
│   │   ├── roomDetail.tsx # Room detail page
│   │   ├── components/    # memberItemButton, passwordModal, roomItem, userInfoPopover, memberSidebar
│   │   ├── hooks/         # useVoiceSession, useRoomAudioBridge, useRoomSounds
│   │   ├── services/      # loadSfuClient, sfuSession
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
│   ├── ErrorComponent.tsx # Error boundary
│   ├── common/            # header, main, sidebar, domainWorkspace
│   └── container/         # ContextProvider (theme)
├── handler_audio/         # Audio handling
│   ├── index.ts           # 音频事件装配入口
│   ├── speakingStore.ts   # Speaking identities state
│   └── notificationSounds.ts # 通知音效
├── protocol/              # WS 协议类型（ws.ts, conversation.ts）
├── types/                 # Shared TypeScript types（message.ts）
├── utils/                 # Utility functions
├── main.tsx               # App entry point
├── styles.css             # Root CSS
└── routeTree.gen.ts       # TanStack Router generated route tree
```

**图标约定**：一律使用 `lucide-solid` 按需导入；`cui-solid-icons` 仅作 cui-solid 的 dist 硬依赖保留，新代码不要直接引用；品牌图标（GitHub/Google/QQ 等）用 `components/oauth/ProviderIcon.tsx`（lucide 不提供品牌图标）。旧的雪碧图组件 `svgIcon.tsx` / `assets/svg.tsx` 已删除。

### socketStore (`src/stores/socketStore.ts`)

Global singleton (createRoot). Manages WebSocket connection, room/member state, and SFU event subscriptions.

**State signals:**

| Signal | Type | Description |
|--------|------|-------------|
| `connected()` | `boolean` | WebSocket connection status |
| `connecting()` | `boolean` | Connection in progress |
| `rooms()` | `RoomInfo[]` | Live room list |
| `currentRoom()` | `string \| null` | Room this client has joined |
| `members()` | `MemberInfo[]` | Derived from rooms()[currentRoom].members |
| `selectedRoomInfo()` | `RoomInfo \| null` | Selected room info |
| `activeSFUProvider()` | `SFUProvider \| undefined` | Current SFU provider |
| `speechRestricted()` | `boolean` | User is muted by server |
| `speechRestrictionInfo()` | `MuteInfo \| null` | Mute reason/expiry |

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

- `VoiceProviderAdapter` declares per-provider join strategy:
  - `interactiveAfterMedia`: UI interactive after media join (SRS WHIP)
  - `signalJoinMode`: `await` (LiveKit) vs `background` (SRS)
  - `serializeJoins`: whether same stream must be sequential
- `useVoiceSession.ts` is the **unified orchestrator** — do not branch per SFU in this file
- `loadSfuClient.ts` preloads the SFU client module
- `sfuSession.ts` manages the SFU client lifecycle

### WHIP/WHEP VoiceChat 加载时机

对 SRS 等 WHIP/WHEP 类 SFU：
- VoiceChat 可交互展示的确认点 = **本端 WHIP publish 成功**（`client.joinRoom` / media join 完成）
- 不要等 `room:join` / `room:join:sfu` 信令全完成才允许加载 VoiceChat
- LiveKit 等非 WHIP provider：以各自 media join 完成点为准；adapter 用 `interactiveAfterMedia` 声明

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

# Web 单元测试（Vitest）
cd app/web && pnpm test
```

API 集成测试发送真实 HTTP 请求并校验响应，新增测试文件放 `test/<module>/<module>.test.ts`；Web 单测与源码同目录（`*.test.ts` / `*.spec.ts`）。

---

## Common Commands

根 package.json 的 `dev` / `build` / `lint` 走 turbo 编排：

```bash
# Dev
cd app/server && pnpm dev    # air 热重载
cd app/web && pnpm dev       # vite --force
# 或同时起前后端：
pnpm start:dev

# Build
pnpm build:server
pnpm build:web

# Test
pnpm test:server             # API 集成测试（test/，vitest）
pnpm test:server:go          # Go 单测（go test ./...）
cd app/web && pnpm test      # web 单测

# 其他
pnpm lint                    # turbo lint（biome + go vet）
pnpm format                  # biome format（递归）
pnpm typecheck               # web/bot/sfu-client tsc --noEmit
pnpm changelog               # 生成 changelog

# From monorepo root
pnpm dev:server
pnpm dev:web
pnpm build
```

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
9. **Bot tokens**: Only grant `BotScopedPermissions` — never expose platform-admin surfaces

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
6. If the provider requires custom signaling semantics, implement `SFUSignalHandler` and wire via `signalHub.SetSFUSignalHandler(...)`
7. Add `VoiceProviderAdapter` in the frontend `providers.ts` for UI integration
8. Set `SFU_PROVIDER="<provider>"` in `.env.dev` or update via `/api/v1/sfu/config`

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
