# GOSpeak Monorepo — AI Agent Guide

## Project Overview

GOSpeak 是一个自托管的**游戏语音平台**，类似自部署版 Discord 语音频道。基于 WebRTC，pnpm monorepo：Go 后端 + SolidJS 前端，多 SFU Provider 抽象。

**为什么自建？** 游戏语音数据不经第三方、自定义语音路由策略、无用户数限制、完全控制部署架构。

关键能力：
- 🎮 语音房间管理（创建/加入/密码保护/踢人）
- 🗣️ 实时发言检测 + 成员独立音量控制
- 🔄 多 SFU 运行时切换（LiveKit / SRS / MediaSoup / Agora / Daily）
- 🐘 渐进式数据库（SQLite 开箱即用 → PostgreSQL → MySQL）
- 🔌 JWT + OAuth2 三端登录（GitHub / Google / QQ）

```
GOSpeak/
├── app/
│   ├── server/          # Go 后端 (Gin + GORM + multi-provider SFU abstraction)
│   ├── web/             # SolidJS 前端 (TypeScript + Vite + TanStack Router)
│   ├── sfu-client/      # 前端多 SFU 客户端抽象
│   └── mediasoup-worker/# MediaSoup Node 服务
├── package.json         # Root scripts
├── pnpm-workspace.yaml  # Workspace config
├── AGENTS.md            # ← This file
└── agent_test_logs/     # Test log output directory
```

---

## Server Architecture — Enterprise Layered Design

```
app/server/
├── main.go                 # Entry point
├── cmd/
│   └── root.go             # CLI (cobra): `server`, `version` commands
├── server/
│   └── gin.go              # DI container, initializes all layers
├── internal/
│   ├── config/             # Config reading from env (DB, Redis, SFU, JWT)
│   ├── model/              # Data models (GORM entities)
│   ├── repository/         # DAO layer, direct DB access
│   ├── service/            # Business logic layer
│   ├── handler/            # HTTP controller (Gin handlers)
│   ├── middleware/         # JWT auth, CORS, permission-based RBAC, ban check
│   ├── router/             # Route registration (sub-route modules)
│   │   └── routes/         # Per-module route groups (auth, user, signal, oauth, sfu, role, permission, mute, room, storage, bot, email, srs, system)
│   ├── sfu/                # SFU provider abstraction layer
│   ├── livekit/            # LiveKit SFU implementation
│   ├── agora/              # Agora SFU implementation
│   ├── daily/              # Daily SFU implementation
│   ├── mediasoup/          # MediaSoup SFU implementation
│   ├── srs/                # SRS SFU implementation (WHIP/WHEP)
│   ├── cloudflare/         # Cloudflare Realtime SFU implementation
│   ├── permcode/           # Permission code constants
│   ├── signal/             # Socket.IO signaling hub
│   ├── redis/              # Optional Redis client (blacklist, JWT key rotation)
│   ├── logger/             # Unified logrus wrapper (level/format/output + gin middleware)
│   └── pkg/                # Shared utilities
│       ├── errors.go       # Business error codes + AppError
│       ├── response.go     # Unified JSON response
│       ├── jwt.go          # JWT token generation/parsing
│       └── oauth/          # Generic OAuth2 provider abstraction (seeded github/google/qq + custom)
├── test/                   # API integration tests (Node.js)
├── docs/                   # Swagger generated docs
├── db/                     # SQLite database storage
├── .env.dev / .env.prod    # Environment config
└── go.mod
```

### Layered Call Flow

```
Request → Router → Middleware → Handler → Service → Repository → DB
                  (JWT+RBAC)      ↓         ↓         ↓
                               OAuth       SFU      Redis
                            (standalone) (provider) (optional)
                                          ↓
                                       Signal/WS
```

Each layer communicates **only with the layer directly below it**:
- **Handler** receives HTTP request, validates input, calls Service
- **Service** implements business logic, calls Repository and SFU-related modules
- **Repository** is pure data access, returns GORM errors
- **SFU providers** are standalone packages wrapping provider-specific APIs (LiveKit, Agora, MediaSoup)
- **OAuth** is a standalone package for third-party login (GitHub, Google, QQ)
- **Redis** is an optional standalone package for token blacklist and JWT key rotation
- **Signal** is a standalone WebSocket hub for signaling

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
pkg.HandleError(c, err)    // auto-detects *AppError

// Error with explicit code
pkg.Fail(c, pkg.INVALID_PARAMS, "custom message")  // msg is optional
```

### Business Status Codes

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | `SUCCESS` | success |
| 1001 | `TOKEN_NOT_EXIST` | token does not exist |
| 1002 | `TOKEN_WRONG` | token is wrong |
| 1003 | `TOKEN_EXPIRED` | token has expired |
| 1010 | `INVALID_PASSWORD` | invalid password |
| 1011 | `USER_NOT_FOUND` | user not found |
| 1012 | `USERNAME_EXISTS` | username already exists |
| 1013 | `FORBIDDEN` | forbidden |
| 1014 | `TOKEN_REVOKED` | token has been revoked |
| 2001 | `INVALID_PARAMS` | invalid parameters |
| 3001 | `NOT_FOUND` | resource not found |
| 3002 | `ALREADY_EXISTS` | resource already exists |
| 5001 | `INTERNAL_ERROR` | internal server error |
| 6001 | `SFU_NOT_CONFIGURED` | sfu not configured |
| 6002 | `SFU_ERROR` | sfu error |
| 7001 | `OAUTH_PROVIDER_NOT_FOUND` | oauth provider not found |
| 7002 | `OAUTH_PROVIDER_DISABLED` | oauth provider is disabled |
| 7003 | `OAUTH_TOKEN_EXCHANGE_FAILED` | oauth token exchange failed |
| 7004 | `OAUTH_GET_USER_FAILED` | oauth get user info failed |

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
- Grouped by module: `auth`, `user`, `signal`, `oauth`, `role`, `permission`, `mute`, `room`, `storage`, `bot`, `email`, `sfu`, `srs`, `system`
- The `protected` group applies `middleware.JWTAuth()` + `middleware.BanCheck()` to every route
- Permission-gated routes additionally use `middleware.RequirePermission(permcode.X)` (permission-based RBAC; see Middleware section)
- Public routes (login, register, oauth, signal exchange, srs callback, system stream) are outside the protected group

### Current Route Table

| Method | Path | Auth | Permission | Handler |
|--------|------|------|-----------|---------|
| POST | `/api/v1/auth/login` | No | — | AuthHandler.Login |
| POST | `/api/v1/auth/register` | No | — | AuthHandler.Register |
| POST | `/api/v1/auth/refresh_token` | No | — | AuthHandler.GetRefreshToken |
| POST | `/api/v1/auth/reset_password` | No | — | AuthHandler.ResetPassword |
| POST | `/api/v1/auth/logout` | JWT | — | AuthHandler.Logout |
| POST | `/api/v1/auth/refresh` | JWT | — | AuthHandler.RefreshToken |
| POST | `/api/v1/auth/change_password` | JWT | — | AuthHandler.ChangePassword |
| POST | `/api/v1/auth/first_change_password` | JWT | — | AuthHandler.FirstChangePassword |
| POST | `/api/v1/user/profile` | JWT | — | UserHandler.GetProfile |
| POST | `/api/v1/user/info` | JWT | — | UserHandler.GetByName |
| POST | `/api/v1/user/update-profile` | JWT | — | UserHandler.UpdateProfile |
| POST | `/api/v1/user/upload-avatar` | JWT | — | UserHandler.UploadAvatar |
| POST | `/api/v1/user/list` | JWT | `user:read` | UserHandler.List |
| POST | `/api/v1/user/get` | JWT | `user:read` | UserHandler.GetByID |
| POST | `/api/v1/user/delete` | JWT | `user:delete` | UserHandler.Delete |
| POST | `/api/v1/user/update-role` | JWT | `user:update` | UserHandler.UpdateRole |
| POST | `/api/v1/signal/token` | JWT | — | SignalHandler.GetJoinToken |
| POST | `/api/v1/signal/signal` | No | — | SignalHandler.Signal |
| GET | `/api/v1/signal/rooms` | No | — | SignalHandler.ListRooms |
| GET | `/api/v1/signal/participants` | No | — | SignalHandler.ListParticipants |
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
| POST | `/api/v1/room/create` | JWT | `room:create` | RoomHandler.Create |
| POST | `/api/v1/room/list` | JWT | `room:read` | RoomHandler.List |
| POST | `/api/v1/room/get` | JWT | `room:read` | RoomHandler.Get |
| POST | `/api/v1/room/update` | JWT | `room:update` | RoomHandler.Update |
| POST | `/api/v1/room/delete` | JWT | `room:delete` | RoomHandler.Delete |
| POST | `/api/v1/storage/presign` | JWT | — | StorageHandler.PresignUpload |
| POST | `/api/v1/storage/confirm` | JWT | — | StorageHandler.ConfirmUpload |
| POST | `/api/v1/storage/upload` | JWT | — | StorageHandler.Upload |
| POST | `/api/v1/storage/delete` | JWT | `storage:delete` | StorageHandler.DeleteObject |
| POST | `/api/v1/storage/config` | JWT | `storage:read` | StorageHandler.GetConfig |
| POST | `/api/v1/storage/update-config` | JWT | `storage:manage` | StorageHandler.UpdateConfig |
| POST | `/api/v1/bot/create` | JWT | `bot:manage` | BotHandler.Create |
| POST | `/api/v1/bot/list` | JWT | `bot:manage` | BotHandler.List |
| POST | `/api/v1/bot/revoke` | JWT | `bot:manage` | BotHandler.Revoke |
| POST | `/api/v1/email/send_code` | JWT | — | EmailVerificationHandler.SendCode |
| POST | `/api/v1/email/verify_code` | JWT | — | EmailVerificationHandler.VerifyCode |
| POST | `/api/v1/email/config` | JWT | `email_config:read` | EmailConfigHandler.Get |
| POST | `/api/v1/email/update-config` | JWT | `email_config:manage` | EmailConfigHandler.Update |
| POST | `/api/v1/sfu/config` | JWT | `sfu:manage` | SFUConfigHandler.Get |
| POST | `/api/v1/sfu/config/:provider` | JWT | `sfu:manage` | SFUConfigHandler.GetProvider |
| POST | `/api/v1/sfu/update-config` | JWT | `sfu:manage` | SFUConfigHandler.Update |
| POST | `/api/v1/sfu/switch-provider` | JWT | `sfu:manage` | SFUConfigHandler.SwitchProvider |
| POST | `/api/v1/sfu/providers` | JWT | `sfu:manage` | SFUConfigHandler.ListProviders |
| POST | `/api/v1/srs/callback` | No | — | SRSCallbackHandler.HandleCallback |
| GET | `/api/v1/system/stream` | No | — | MonitorHandler.HealthStream |
| GET | `/ping` | No | — | Health check |
| GET | `/swagger/*any` | No | — | Swagger UI |
| GET | `/uploads/*` | No | — | Static avatar/uploads |
| WS | `/socket.io/*` | No | — | Socket.IO signaling |

---

## Configuration

All configuration is injected via environment variables (`.env.dev` for dev, `deploy/env/app.*.env` for Docker). Startup loads env files without overriding process env (`process > .env.<env> > .env > defaults`), then `config.Load()` parses into a typed `Config` via `caarlos0/env`, normalizes aliases (e.g. `PostgresSQL` → `PostgreSQL`), validates, and exposes `config.Current()` for packages that cannot take an explicit dependency injection.

### Database

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_TYPE` | `SQLite` | `SQLite` / `PostgresSQL` / `MYSQL` |
| `DB_HOST` | `localhost` | DB host (PostgreSQL/MySQL) |
| `DB_PORT` | `5432` | DB port |
| `DB_USER` | — | DB user |
| `DB_PASSWORD` | — | DB password |
| `DB_PATH` | `db/app.db` | SQLite file path |
| `DB_DSN` | — | Custom DSN (overrides the field-by-field settings) |
| `DB_WAL` | `false` | SQLite WAL mode（并发读建议开启）|

### JWT

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_KEY` | `default-secret` | HMAC signing key (change in production) |
| `JWT_KEY_TTL` | `24h` | Signing-key rotation interval (Redis only) |

### SFU Provider

`SFU_PROVIDER` selects the active backend: `livekit` (default), `srs`, `mediasoup`, `agora`, `daily`, `cloudflare`. Provider-specific config:

| Variable | Default | Description |
|----------|---------|-------------|
| `LIVEKIT_HOST` / `LIVEKIT_KEY` / `LIVEKIT_SECRET` | — | LiveKit server URL, API key/secret |
| `AGORA_APP_ID` / `AGORA_APP_CERTIFICATE` / `AGORA_HOST` / `AGORA_CUSTOMER_ID` / `AGORA_CUSTOMER_SECRET` | — | Agora credentials |
| `MEDIASOUP_BRIDGE_URL` | `http://localhost:3012` | MediaSoup worker HTTP bridge |
| `MEDIASOUP_HOST` | `localhost:3012` | Client-facing MediaSoup host |
| `SRS_HOST` / `SRS_API_PORT` | `localhost` / `1985` | SRS management API |
| `SRS_WHIP_URL` | `/rtc/v1/whip/` | SRS WHIP endpoint path |
| `SRS_SECRET` | — | SRS stream/room token HMAC key (required) |
| `SRS_PUBLIC_HOST` | — | Browser-side serverUrl prefix |
| `DAILY_API_KEY` / `DAILY_DOMAIN` | — | Daily credentials |
| `CF_APP_ID` / `CF_APP_SECRET` / `CF_STUN_URL` | — / — / `stun.cloudflare.com:3478` | Cloudflare Realtime credentials |

> Base SFU config is loaded from env; persistent per-provider config is managed through `/api/v1/sfu/*` and resolved at runtime by `sfu.NewDynamicProvider(...)`.

### Redis (optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | — | Leave empty to skip Redis (graceful degradation) |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | — | Redis password |
| `REDIS_DB` | `0` | Redis DB index |

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
| `STORAGE_ENDPOINT` | — | S3-compatible endpoint (MinIO / R2) |
| `STORAGE_BUCKET` | — | S3 bucket |
| `STORAGE_REGION` | — | S3 region |
| `STORAGE_ACCESS_KEY` / `STORAGE_SECRET_KEY` | — | S3 credentials |
| `STORAGE_PUBLIC_BASE_URL` | — | Public base URL (CDN / custom domain) |
| `STORAGE_PATH_PREFIX` | `uploads/` | Upload path prefix |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8998` | HTTP listen port |
| `STATIC_DIR` | — | Frontend static dir (SPA hosting in prod) |
| `GIN_MODE` | `debug` | Gin mode (`release` in prod) |

### Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | dev=`debug` / prod=`info` | `trace` / `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | dev=`text` / prod=`json` | `text` / `json` |
| `LOG_OUTPUT` | `stdout` | `stdout` / `stderr` / `file` / `both` |
| `LOG_FILE` | `logs/app.log` | path when output is `file`/`both` |
| `LOG_CALLER` | `false` | print caller file:line |


- Auto-migration is enabled — all models are synced on startup in `repository/db.go`.

---

## SFU Abstraction Layer

Standalone package at `internal/sfu/`. Provides a provider interface so multiple SFU backends can be plugged in and resolved dynamically at runtime.

### Provider Interface (`internal/sfu/provider.go`)

```go
type Provider interface {
    GenerateToken(room, identity string) (string, error)
    GenerateAdminToken() (string, error)
    ListRooms() (interface{}, error)
    ListParticipants(room string) (interface{}, error)
    MuteParticipant(room, identity, trackSid string, muted bool) error
    RemoveParticipant(room, identity string) error
    DeleteRoom(room string) error
    GetHost() string
}
```

### Factory (`internal/sfu/factory.go`)

`sfu.NewProvider(cfg)` reads `cfg.SFUProvider` and returns the matching implementation. Registered providers are `"livekit"`, `"agora"`, `"mediasoup"`, `"srs"`, `"daily"`, and `"cloudflare"`.

`sfu.NewDynamicProvider(resolve)` is the runtime entry used by server wiring. It resolves config via `SFUConfigService.ResolveConfig()` and delegates each call to the current provider.

### SFU 与信令分工

| 操作 | 信令层 | SFU 层 | 说明 |
|------|--------|--------|------|
| `RemoveParticipant` | ✅ 删 Members + 广播 | ✅ LiveKit/SRS/MediaSoup/Daily 均调（按 `ErrSFUNotSupported` 优雅降级） | 仅 Agora 跳过（返回 `ErrSFUNotSupported`） |
| `ListRooms` + `ListParticipants` | 失败时返回 `[]` | ✅ 有则返回 | SFU 媒体状态 ≠ 信令层在线状态，不 fallback |
| `Mute*` | `BroadcastMute` 广播 | ❌ 不调 | 前端收到事件后自行停推流，仅 LiveKit 有服务端强制能力 |

详见 `internal/signal/AGENTS.md`。

> **SFU 历史密钥安全取舍 (#6)**: 历史密钥集合保留 7 天 (histTTL)，对齐 refresh_token 最大有效期。
> 任一历史密钥泄漏都可在有效期内伪造合法 token。若要缩短 window，减小 `histTTL` 常量即可。


详见 `internal/signal/AGENTS.md`。

> **SFU 历史密钥安全取舍 (#6)**: 历史密钥集合保留 7 天 (histTTL)，对齐 refresh_token 最大有效期。
> 任一历史密钥泄漏都可在有效期内伪造合法 token。若要缩短 window，减小 `histTTL` 常量即可。


### Usage in handlers / services

All SFU calls go through `sfu.Provider`. Current service paths already consume the abstraction in `SignalHandler` and `signal.Hub`. To add a new SFU backend, implement `sfu.Provider` and register it in `factory.go`.

### Current provider maturity

| Provider | Maturity | Notes |
|----------|----------|-------|
| LiveKit | Highest | Full room/token/participant/mute/remove/delete support |
| SRS | High | Token/Room/Delete/RemoveParticipant via REST; WHIP/WHEP media. Mute + ListParticipants not supported |
| Agora | Medium | Token and basic room APIs work; mute/kick/admin flows incomplete |
| Daily | Medium | Token/rooms/participants via REST. Mute/kick not supported |
| MediaSoup | Medium | Uses provider-specific signaling path; generic provider methods return not supported |
| Cloudflare | Medium | Realtime SFU via WHIP/WHEP; room/token via REST, media over WebRTC |

---

## LiveKit Module

Standalone package at `internal/livekit/`. Implements `sfu.Provider` using `github.com/livekit/server-sdk-go/v2`.

Constructor: `livekit.NewService(cfg *config.Config) *Service`

| Method | Description |
|--------|-------------|
| `GenerateToken(room, identity)` | Room join token (JWT) |
| `GenerateAdminToken()` | Admin token for server-side room management |
| `ListRooms()` | List all active rooms via `lksdk.RoomServiceClient` |
| `ListParticipants(room)` | List participants in a room |
| `MuteParticipant(room, identity, trackSid, muted)` | Mute/unmute a track |
| `RemoveParticipant(room, identity)` | Kick a participant |
| `DeleteRoom(room)` | Delete a room |
| `GetHost()` | Return configured LiveKit host URL |

Token response shape returned by `POST /signal/token`:

```json
{
  "token": "eyJ...",
  "serverUrl": "wss://...",
  "room": "room-name",
  "identity": "user-identity"
}
```

---

## Redis Module (Optional)

Standalone package at `internal/redis/`. Provides optional Redis support — gracefully degrades when `REDIS_HOST` is not set.

### Configuration

```
REDIS_HOST=""             # Leave empty to skip Redis connection
REDIS_PORT="6379"
REDIS_PASSWORD=""
REDIS_DB="0"
```

### Key files

| File | Role |
|------|------|
| `redis.go` | Client init, `InitRedis()`, `IsConnected()` |
| `blacklist.go` | JWT token blacklist (logout invalidation) |
| `jwt_key.go` | JWT signing key rotation via Redis TTL |

### Token Blacklist

Logout adds the token's JTI to Redis with TTL = remaining token lifetime. Middleware checks blacklist before granting access.

```go
redis.BlacklistToken(jti, remaining)  // add to blacklist
redis.IsBlacklisted(jti)              // check if blacklisted
```

When Redis is not connected, blacklist operations are no-ops (best-effort strategy).

### JWT Key Rotation

`redis.GetOrRotateSigningKey()` returns the current signing key:
- **Redis connected**: key stored in Redis with configurable TTL (`JWT_KEY_TTL`, default 24h). TTL expiry triggers automatic rotation — all old tokens become invalid.
- **Redis not connected**: falls back to static `JWT_KEY` env var (default `"default-secret"`).

---

## OAuth Module

Provides generic OAuth2 third-party login. Providers are configured in the `oauth_providers` table and managed through the admin API; `github`, `google`, and `qq` are seeded presets, and arbitrary OpenID/OAuth2 providers are supported via custom endpoint URLs plus JSON field mappings. Two layers:

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

`oauth.GetDefaultConfig(name)` returns preset endpoint configs for each platform (ClientID/Secret/RedirectURL must be injected). Built-in presets seed `github` / `google` / `qq`; additional providers are persisted in `oauth_providers` and resolved at runtime by the OAuth service.

### Data Layer

| Model | Table | Description |
|-------|-------|-------------|
| `OAuthProvider` | `oauth_providers` | Platform config (name, client_id, secret, endpoints, enabled) |
| `OAuthAccount` | `oauth_accounts` | User ↔ platform binding (user_id, provider, provider_uid) |

Repositories: `OAuthProviderRepo`, `OAuthAccountRepo` — standard CRUD.

### Handler & Routes

| Method | Path | Auth | Handler |
|--------|------|------|---------|
| GET | `/api/v1/oauth/login/:provider` | No | OAuthHandler.Login (redirect) |
| GET | `/api/v1/oauth/callback/:provider` | No | OAuthHandler.Callback |
| GET | `/api/v1/oauth/admin/providers` | Yes (admin) | OAuthHandler.ListProviders |
| POST | `/api/v1/oauth/admin/providers` | Yes (admin) | OAuthHandler.CreateProvider |
| PUT | `/api/v1/oauth/admin/providers` | Yes (admin) | OAuthHandler.UpdateProvider |
| DELETE | `/api/v1/oauth/admin/providers/:id` | Yes (admin) | OAuthHandler.DeleteProvider |

---

## Middleware

All middleware is in `internal/middleware/auth.go`.

| Function | Description |
|----------|-------------|
| `JWTAuth()` | Validates Bearer token: header → signature → expiry → token version → blacklist (JTI); injects `username`, `display_name`, `user_uuid`, `role`, `permissions`, `claims`, `auth_type` into context |
| `RequireRole(roles ...string)` | Legacy role check against the `role` claim; returns `FORBIDDEN` (1013) on mismatch |
| `RequirePermission(permCode)` | Permission-based gate. Bot tokens use `Claims.Permissions`; users map `role` → permissions via `role_permissions`. Returns `FORBIDDEN` (1013) on missing permission |
| `RequireOwnerOrPermission(ownerContextKey, permCode)` | Allows the request if the caller owns the resource (owner field equals `username`) or holds the permission |
| `BanCheck()` | Blocks any user whose `role` is `ban`; returns `FORBIDDEN` (1013) |
| `CORS()` | Sets `Access-Control-Allow-Origin: *`, handles OPTIONS preflight |

### JWTAuth check order

1. Header exists → `TOKEN_NOT_EXIST` (1001)
2. Signature valid → `TOKEN_WRONG` (1002)
3. Not expired → `TOKEN_EXPIRED` (1003)
4. `TokenVersion` matches current user → `TOKEN_REVOKED` (1014) on mismatch (password change / reset invalidates old tokens)
5. JTI not blacklisted → `TOKEN_REVOKED` (1014)
6. Inject `username`, `display_name`, `user_uuid`, `role`, `permissions`, `claims`, `auth_type` into context

---

## RBAC / Permissions

Access control is **permission-based**, layered on top of roles.

- **Roles** (`roles` table): seeded with `admin`, `user`, `ban`. The `ban` role is intercepted by `BanCheck()` and always gets `FORBIDDEN`.
- **Default admin account**: first boot seeds user `admin` / `admin123` when missing (`service.DefaultAdminPassword`). Login returns `need_change_password=true` while still on the default password; change it immediately in production.
- **Permissions** (`permissions` table): each row holds a `permcode` constant such as `user:read`, `room:create`, `sfu:manage`, `mute:manage`, `bot:manage`, `storage:delete`, `oauth:manage`, `email_config:read`, `role:manage`, `signal:kick`.
- **Role → Permission mapping** (`role_permissions` table): links a role name to permission IDs. `RequirePermission` resolves a user's effective permissions from their `role`.
- **Bot tokens** (`bot_tokens`): carry an explicit `permissions` list in the JWT (`Claims.Permissions`). Bot-scoped permissions are whitelisted via `model.BotScopedPermissions` so bots cannot reach platform-admin surfaces.
- **Token versioning**: `User.TokenVersion` is embedded in the JWT. Changing a password / resetting it bumps the version, so all previously issued tokens are rejected (`TOKEN_REVOKED`).

All permission constants live in `internal/permcode/permcode.go`.

## Models

| Model | Table | Key Fields |
|-------|-------|------------|
| `User` | `users` | ID, UUID (auto-gen), Name, DisplayName, Avatar, Email, EmailVerified, IsBot, Password (`json:"-"`), Role, TokenVersion, timestamps |
| `Room` | `room` | ID, UUID (auto-gen), Name, Password, Description, Limit, AudioOnly, AllowAudience, CreatedBy, timestamps |
| `UserGroup` | `user_groups` | ID, UserID, GroupName, timestamps |
| `Role` | `roles` | ID, Name (seeds: `admin` / `user` / `ban`), timestamps |
| `Permission` | `permissions` | ID, Code (unique `permcode`), Name, Description, timestamps |
| `RolePermission` | `role_permissions` | ID, RoleName, PermissionID |
| `Mute` | `mutes` | ID, UUID, UserID, MuterID, Duration, Permanent, ExpiresAt, Reason, timestamps |
| `OAuthProvider` | `oauth_providers` | ID, Name, DisplayName, IconURL, ClientID, ClientSecret, AuthURL, TokenURL, UserInfoURL, RedirectURL, Scopes, field mappings, Enabled, timestamps |
| `OAuthAccount` | `oauth_accounts` | ID, UserID, Provider, ProviderUID; AccessToken & RefreshToken stored but hidden (`json:"-"`), timestamps |
| `BotToken` | `bot_tokens` | ID, UUID, Name, UserUUID, Permissions `[]string`, Revoked, ExpiresAt, timestamps |
| `EmailConfig` | `email_configs` | ID, Enabled, SMTP host/port/user/pass/from/name, EmailCodeTTL, EmailSendCooldown, EmailCodeSecret (hidden, `json:"-"`), timestamps |
| `EmailVerificationCode` | `email_verification_codes` | ID, Email, Scene, CodeHash (hidden), UserID, IPAddress, ExpiresAt, UsedAt, AttemptCount, timestamps |
| `StorageConfig` | `storage_configs` | ID, ProviderType (`local`/`s3`), Endpoint, Bucket, Region, AccessKey & SecretKey (hidden), PublicBaseURL, PathPrefix, MaxFileSize, AllowedTypes, timestamps |
| `SFUConfig` | `sfu_configs` | Provider (PK), LiveKit*/Agora*/MediaSoup*/SRS*/Daily*/Cloudflare* config fields, timestamps |
| `SFUActiveProvider` | `sfu_active_provider` | ID, Provider |

> Auto-migration (`repository/db.go`) syncs all of the above on startup. New fields/models take effect after a restart — no manual DDL.


---

## Signal (Socket.IO) Module

Standalone package at `internal/signal/`. Handles real-time room signaling via Socket.IO (`googollee/go-socket.io v1.7.0`).

### Key files

| File | Role |
|------|------|
| `events.go` | 14 event name constants |
| `types.go` | `RoomRequest`, `MemberInfo`, `RoomInfo` structs |
| `hub.go` | Hub (global room registry) + Socket.IO event handlers |

### Event Table

| Direction | Event | Payload | Description |
|-----------|-------|---------|-------------|
| Lifecycle | `connection` | — | Client connected |
| Lifecycle | `disconnect` | — | Client disconnected |
| Client → Server | `room:create` | `{room}` | Create a new room |
| Client → Server | `room:join` | `{room, identity}` | Join a room |
| Client → Server | `room:leave` | `{room}` | Leave a room |
| Client → Server | `room:list` | — | Request room list |
| Server → Client | `room:created` | `RoomInfo` | A room was created |
| Server → Client | `room:joined` | `{room, members[]}` | Self joined a room |
| Server → Client | `room:left` | `{room, identity}` | Self left a room |
| Server → Client | `room:updated` | `RoomInfo` | Room member count changed |
| Server → Client | `member:joined` | `MemberInfo` | Another member joined |
| Server → Client | `member:left` | `{identity}` | A member left |
| Server → Client | `member:updated` | `MemberInfo` | Member state updated |
| Server → Client | `room:list:result` | `{rooms[]}` | Response to `room:list` |

### Hub API

```go
hub := signal.NewHub(roomStore, muteStore)
hub.SetServer(server)
hub.SetSFU(provider)    // sets sfuProvider + sfuProviderName
hub.SetupRoutes(server) // registers all Socket.IO events
hub.BroadcastToRoom(namespace, room, event, data)
```

### Hub 房间查询方法

| 方法 | 返回 |
|------|------|
| `GetSFURooms()` | 仅内存活跃房间（有 WS 连接），供 `room:list` 广播 |
| `GetRooms()` | DB 持久化房间 + 内存活跃房间合并 |
| `GetRoomMembers(room)` | 指定房间的在线成员 |

### OnRoomKick SFU dispatch

信令层始终先处理（删 Members + 广播），随后由 `Hub.removeParticipantSafe` 直接调用 `sfuProvider.RemoveParticipant(room, identity)`。Hub **不再硬编码 provider 名**，仅在 provider 返回 `pkg.ErrSFUNotSupported` 时静默跳过，因此「踢人是否真正到达 SFU」由各 provider 自身是否实现 `RemoveParticipant` 决定：

| provider | `sfuProvider.RemoveParticipant` 调用 | 实现状态 |
|----------|------------------------------------|----------|
| livekit | ✅ | 原始完整实现 |
| srs | ✅（`KickParticipant` → `RemoveParticipant` 统一命名） | 原始完整实现 |
| mediasoup | ✅ bridge `CloseParticipant` | 补全实现（历史文档误标为跳过） |
| daily | ✅ list → 按 session id `RemoveParticipant` | 补全实现（历史文档误标为跳过） |
| agora | ❌ 跳过（返回 `ErrSFUNotSupported`，无单用户踢人 REST API） | 未实现，仅 ban 语义 |

`/signal/rooms` 和 `/signal/participants` 失败时返回空列表 `[]`。SFU 媒体节点状态与 WS 在线成员不可互相 fallback。

---

## Frontend Architecture

```
app/web/src/
├── api/                # apiClient (axios wrapper) + auth API
├── assets/             # Static assets (SVG icons, global CSS)
│   ├── styles/         # Global stylesheets
│   └── svg.tsx         # SVG icon sprite definitions
├── components/
│   ├── chat/           # Chat input/output components
│   ├── common/         # Shared UI components (avatar, modal, divider, etc.)
│   ├── form/           # Form components (Form, PasswordChangeForm)
│   ├── home/           # Home page component
│   ├── modal/          # Modals (searchModal, settings)
│   │   └── settting/   # Settings modal (note: 3 t's typo in dir name)
│   │       └── tab_item/  # Settings tabs (audio, general, room)
│   ├── room/           # Room-related views
│   ├── funcButton.tsx  # FAB action button with search modal
│   ├── svgIcon.tsx     # SVG icon component (uses svg sprite)
│   └── userBar.tsx     # User status bar with audio controls
├── hooks/
│   ├── livekit/        # LiveKit hooks (createRoom, roomAction, useToken, useSubcribeTrack)
│   │   ├── index.ts    # Barrel export
│   │   ├── createRoom.ts  # Room creation hook
│   │   ├── roomAction.ts  # Join/leave room actions
│   │   ├── useToken.ts    # Token fetching via TanStack Query
│   │   └── useSubcribeTrack.ts  # Track subscription (empty)
│   ├── media.ts        # Audio device enumeration utility
│   └── useTitle.ts     # Document title hook (empty)
├── layouts/
│   ├── layout.tsx      # Root layout
│   ├── ErrorComponent.tsx  # Error boundary
│   ├── common/         # Layout primitives
│   │   ├── header.tsx  # Header with theme toggle + route title
│   │   ├── footer.tsx  # Footer bar
│   │   ├── main.tsx    # Main content area wrapper
│   │   └── sidebar.tsx # Sidebar navigation (home, channel, settings)
│   └── container/
│       └── ContextProvider.tsx  # Theme context provider
├── stores/
│   ├── socketStore.ts         # Socket.IO global singleton store
│   ├── userStore.ts           # Auth state (user, tokens) with IndexedDB persistence
│   ├── themeStore.ts          # Theme switching (light/dark)
│   ├── audioDeviceStore.ts    # Audio device enumeration with IndexedDB persistence
│   └── voiceChatStore.ts      # Voice chat state (mute, volume) with IndexedDB persistence
├── types/
│   ├── room.ts                # RoomInfo, MemberInfo, RoomItemType, RoomMemberInfoType
│   └── userInfo.ts            # UserInfo, LocalUserInfo, Token types
├── utils/                     # Utility directory (currently empty)
├── main.tsx                   # App entry point
├── styles.css                 # Root CSS
└── routeTree.gen.ts           # TanStack Router generated route tree
```

### socketStore (`src/stores/socketStore.ts`)

Global singleton created with `createRoot`. Manages the Socket.IO connection lifecycle and reactive state for rooms and members.

**State signals:**

| Signal | Type | Description |
|--------|------|-------------|
| `connected()` | `boolean` | Socket.IO connection status |
| `rooms()` | `RoomInfo[]` | Live room list |
| `currentRoom()` | `string \| null` | Room this client has joined |
| `members()` | `MemberInfo[]` | Members in current room |

**Methods:**

| Method | Description |
|--------|-------------|
| `connect(token?)` | Open Socket.IO connection |
| `disconnect()` | Close connection |
| `createRoom(name)` | Emit `room:create` |
| `joinRoom(room, identity)` | Emit `room:join` |
| `leaveRoom(room)` | Emit `room:leave` |
| `listRooms()` | Emit `room:list` |

### userStore (`src/stores/userStore.ts`)

Manages authentication state with IndexedDB persistence (survives page reload).

| Signal | Type | Description |
|--------|------|-------------|
| `user()` | `UserInfo \| null` | Current user (id, uuid, name, role) |
| `accessToken()` | `string` | JWT access token |
| `refreshToken()` | `string` | JWT refresh token |
| `isLoggedIn()` | `boolean` | Derived: true if accessToken exists |

| Method | Description |
|--------|-------------|
| `login(user, accessToken, refreshToken)` | Persist auth data to IndexedDB + signals |
| `logout()` | Call server logout API, clear IndexedDB + signals |
| `updateAccessToken(token)` | Update access token (for auto-refresh) |

### themeStore (`src/stores/themeStore.ts`)

Light/dark theme switching. Persists to localStorage. Themes: `acid` (light), `synthwave` (dark).

### audioDeviceStore (`src/stores/audioDeviceStore.ts`)

Audio input/output device enumeration with IndexedDB persistence. Auto-fetches devices on init.

### voiceChatStore (`src/stores/voiceChatStore.ts`)

Manages voice/video chat state with IndexedDB persistence. Uses debounced writes.

**State:**

| Field | Type | Description |
|-------|------|-------------|
| `isInputMute` | `boolean` | Audio input muted |
| `isOutMute` | `boolean` | Audio output muted |
| `isVideoMute` | `boolean` | Video output muted |
| `inputVolume` | `number` | Audio input volume (0-100) |
| `outputVolume` | `number` | Audio output volume (0-100) |
| `videoVolume` | `number` | Video output volume (0-100) |
| `otherMemberState` | `OtherMemberStateType[]` | Per-member voice state |

**Actions:** `setIsInputMute`, `setIsOutMute`, `setIsVideoMute`, `setInputVolume`, `setOutputVolume`, `setVideoVolume`

### LiveKit Hooks (`src/hooks/livekit/`)

| Hook | Description |
|------|-------------|
| `createRoom` | Create LiveKit Room instance |
| `_joinRoom(room, url, token)` | Connect to room + enable microphone |
| `_leaveRoom(room)` | Disconnect from room |
| `useToken()` | Fetch join token via TanStack Query (`POST /api/v1/signal/token`) |
| `useSubcribeTrack` | Track subscription (placeholder, empty) |

### Room join data flow

```
User enters room page
    ├─ socketStore.connect()
    ├─ POST /api/v1/signal/token  →  { token, serverUrl, room, identity }
    ├─ socketStore.joinRoom(room, identity)
    │      ← room:joined { members: [...] }
    ├─ createRoom({ token, url })   // LiveKit instance
    └─ _joinRoom(roomIns, url, token)  // LiveKit connect + enable mic



### WHIP/WHEP VoiceChat 加载时机

对 SRS 等 **WHIP/WHEP** 类 SFU：

- VoiceChat **可交互展示** 的确认点 = **本端 WHIP publish 成功**（`client.joinRoom` / media join 完成）
- **不要** 等 `room:join` / `room:join:sfu` 信令全完成才允许加载 VoiceChat
- 信令仍继续跑（成员列表、WHEP 订阅），但 UI 不得因信令慢而卡住 loading
- LiveKit 等非 WHIP provider：仍以各自 media join 完成点为准；adapter 用 `interactiveAfterMedia` 声明
- 实现落点：`providers.ts` / `runVoiceJoin.ts` / `voiceSessionTypes.ts` / `packages/sfu-client/*`
- `runVoiceJoin` media 成功后必须先 `onClientReady(client)` 再 `onPhase("media_ready")`，否则 UI interactive 但 `session.client` 仍 null
- `useVoiceSession` 只接通用 `onClientReady`（挂 client），**禁止** 为某个 SFU 写分支

### useVoiceSession 锁定规则

`app/web/src/components/room/hooks/useVoiceSession.ts` 是统一进房编排器，**禁止为适配 SFU 而修改**。

- 允许改：provider adapter / `runVoiceJoin` / `packages/sfu-client/*` / `api/sfu.ts`
- 禁止改：`useVoiceSession.ts` 的 join 生命周期、phase、abort/teardown，仅为某个 SFU 分支特殊处理
- LiveKit 回归：先确认上述 adapter/client 层，不要动 `useVoiceSession`

### Multi-SFU frontend note

The backend now returns provider-aware token payloads and supports runtime provider switching. The frontend still contains historical LiveKit-oriented structure, but new work should treat LiveKit as one implementation behind a shared SFU client layer rather than the only runtime.
```

### OAuth login flow

```
User clicks "Login with GitHub"
    ├─ Browser → GET /api/v1/oauth/login/github
    │      ← 302 redirect to GitHub auth page
    ├─ User authorizes on GitHub
    │      ← redirect to /api/v1/oauth/callback/github?code=xxx
    ├─ Server: ExchangeToken(code) → access_token
    ├─ Server: GetUserInfo(access_token) → UserInfo
    ├─ Server: find or create User + OAuthAccount
    └─ Response: { access_token, refresh_token, user }
```

---

## Testing

Node.js-based API integration tests in `test/`:

```bash
# Start the server first, then:
cd app/server
pnpm test

# Or from monorepo root:
pnpm test:server
```

Tests send real HTTP requests and validate responses. Add new test files under `test/<module>/<module>.test.js`.

---

## Common Commands

```bash
# Start server (dev mode, SQLite)
cd app/server
pnpm dev

# Start server (prod mode)
pnpm prod

# Build binary
pnpm build

# Run tests (server must be running)
pnpm test

# From monorepo root
pnpm dev:server
pnpm test:server
pnpm build:server
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

## Mute vs Restrict Speech

- **`静音`** in the current frontend means **local playback mute of a remote audio track**. It is client-local, does not require server state, and should not be modeled as an SFU provider capability.
- **`禁言`** means **user-level speech restriction**. The restricted user may still listen, but must not publish a local audio track.
- There is **no room-level mute** concept in the intended product semantics.
- Existing legacy event names such as `room:mute` / `member:muted` should be treated as deprecated semantics and should not be reintroduced into new frontend work.
- User-level restriction is represented by `user:muted` / `user:unmuted`, backend mute records, and join/publish checks.

---

## Adding a New Feature (Example Flow)

1. Define model in `internal/model/`
2. Add repository methods in `internal/repository/`
3. Add business logic in `internal/service/`
4. Add HTTP handler in `internal/handler/`
5. Create route file in `internal/router/routes/<module>/routes.go`
6. Register route group in `internal/router/router.go`
7. Wire dependencies in `server/gin.go`
8. Add tests in `test/`
9. Regenerate Swagger docs

## Adding a New SFU Backend

1. Create `internal/<provider>/client.go` implementing `sfu.Provider`
2. Add a case in `internal/sfu/factory.go` for the new `SFU_PROVIDER` value
3. Add provider-specific config fields to `internal/config/config.go`, `model.SFUConfig`, and the `/api/v1/sfu/config` management flow when needed
4. Set `SFU_PROVIDER="<provider>"` in `.env.dev` or update config through `/api/v1/sfu/config`
5. If the provider requires custom signaling semantics, wire it in `server/gin.go` via `signalHub.SetSFUSignalHandler(...)`
6. Keep handler/router usage on `sfu.Provider`; avoid leaking provider-specific branching into generic HTTP handlers

---

## Test Logging

当 agent 被命令进行测试时，必须将测试总结的结果以 Markdown 格式保存到 `agent_test_logs` 文件夹。详见 `agent_test_logs/AGENTS.md`。

### 命名规范

文件名格式：`{测试内容}-{时间}.md`

示例：
- `api-auth-test-2026-05-26.md` - 认证 API 测试
- `role-permission-test-2026-05-26.md` - 角色权限测试
- `user-crud-test-2026-05-26-14-30.md` - 用户 CRUD 测试（精确到分钟）
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
