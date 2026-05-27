# GoRTC Monorepo — AI Agent Guide

## Project Overview

GoRTC is a **pnpm monorepo** containing a Go backend server and a React/TypeScript web frontend for real-time communication using WebRTC (LiveKit).

```
GOSpeak/
├── packages/
│   ├── server/          # Go backend (Gin + GORM + LiveKit)
│   └── web/             # React frontend (Vite + TypeScript)
├── package.json         # Root scripts
├── pnpm-workspace.yaml  # Workspace config
├── AGENTS.md            # ← This file
└── agent_test_logs/     # Test log output directory
```

---

## Server Architecture — Enterprise Layered Design

```
packages/server/
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
│   ├── middleware/         # JWT auth, CORS, RBAC (RequireRole)
│   ├── router/             # Route registration (sub-route modules)
│   │   └── routes/         # Per-module route groups (auth, user, signal, oauth, swagger)
│   ├── sfu/                # SFU provider abstraction layer
│   ├── livekit/            # LiveKit SFU implementation
│   ├── signal/             # Socket.IO signaling hub
│   ├── redis/              # Optional Redis client (blacklist, JWT key rotation)
│   └── pkg/                # Shared utilities
│       ├── errors.go       # Business error codes + AppError
│       ├── response.go     # Unified JSON response
│       ├── jwt.go          # JWT token generation/parsing
│       └── oauth/          # OAuth2 provider abstraction (GitHub, Google, QQ)
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
                               OAuth     LiveKit    Redis
                            (standalone) (standalone) (optional)
                                          ↓
                                       Signal/WS
```

Each layer communicates **only with the layer directly below it**:
- **Handler** receives HTTP request, validates input, calls Service
- **Service** implements business logic, calls Repository and LiveKit
- **Repository** is pure data access, returns GORM errors
- **LiveKit** is a standalone package wrapping LiveKit API
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
- Grouped by module: `/api/v1/auth/*`, `/api/v1/user/*`, `/api/v1/signal/*`
- Auth-required routes use `middleware.JWTAuth()` under a `protected` group
- Public routes (login, register, signal token) are outside the protected group

### Current Route Table

| Method | Path | Auth | Handler |
|--------|------|------|---------|
| POST | `/api/v1/auth/login` | No | AuthHandler.Login |
| POST | `/api/v1/auth/register` | No | AuthHandler.Register |
| POST | `/api/v1/auth/refresh_token` | No | AuthHandler.GetRefreshToken |
| POST | `/api/v1/auth/reset_password` | No | AuthHandler.ResetPassword |
| POST | `/api/v1/auth/logout` | Yes | AuthHandler.Logout |
| POST | `/api/v1/auth/refresh` | Yes | AuthHandler.RefreshToken |
| POST | `/api/v1/auth/change_password` | Yes | AuthHandler.ChangePassword |
| POST | `/api/v1/auth/first_change_password` | Yes | AuthHandler.FirstChangePassword |
| GET | `/api/v1/oauth/login/:provider` | No | OAuthHandler.Login (redirect) |
| GET | `/api/v1/oauth/callback/:provider` | No | OAuthHandler.Callback |
| GET | `/api/v1/oauth/admin/providers` | Yes (admin) | OAuthHandler.ListProviders |
| POST | `/api/v1/oauth/admin/providers` | Yes (admin) | OAuthHandler.CreateProvider |
| PUT | `/api/v1/oauth/admin/providers` | Yes (admin) | OAuthHandler.UpdateProvider |
| DELETE | `/api/v1/oauth/admin/providers/:id` | Yes (admin) | OAuthHandler.DeleteProvider |
| POST | `/api/v1/signal/token` | No | SignalHandler.GetJoinToken |
| POST | `/api/v1/signal/signal` | No | SignalHandler.Signal |
| POST | `/api/v1/signal/webhook` | No | SignalHandler.LivekitWebhook |
| GET | `/api/v1/signal/rooms` | No | SignalHandler.ListRooms |
| GET | `/api/v1/signal/participants` | No | SignalHandler.ListParticipants |
| GET | `/api/v1/user/profile` | Yes | UserHandler.GetProfile |
| GET | `/api/v1/user/list` | Yes | UserHandler.List |
| GET | `/api/v1/user/:id` | Yes | UserHandler.GetByID |
| DELETE | `/api/v1/user/:id` | Yes (admin) | UserHandler.Delete |
| PUT | `/api/v1/user/:id/role` | Yes (admin) | UserHandler.UpdateRole |
| GET | `/ping` | No | Health check |
| GET | `/swagger/*any` | No | Swagger UI |
| WS | `/socket.io/*` | No | Socket.IO signaling |

---

## Database

- **Default**: SQLite (zero config, auto-creates `db/app.db`)
- **Supported**: PostgreSQL, MySQL via GORM
- Configure via `.env.dev` / `.env.prod`:

```
DB_TYPE="SQLite"          # SQLite | PostgreSQL | MySQL
DB_HOST=""                # Required for PostgreSQL/MySQL
DB_PORT=""
DB_USER=""
DB_PASSWORD=""
DB_PATH=""                # Leave empty for default (db/app.db)
```

### SFU Configuration

```
SFU_PROVIDER="livekit"    # Currently only "livekit" is supported
LIVEKIT_HOST=""           # LiveKit server URL (wss://...)
LIVEKIT_KEY=""            # LiveKit API key
LIVEKIT_SECRET=""         # LiveKit API secret
```

### Redis Configuration (Optional)

```
REDIS_HOST=""             # Leave empty to skip Redis (graceful degradation)
REDIS_PORT="6379"
REDIS_PASSWORD=""
REDIS_DB="0"
JWT_KEY_TTL="24h"         # JWT signing key rotation interval (Redis only)
```

- Auto-migration is enabled — models are synced on startup in `repository/db.go`

---

## SFU Abstraction Layer

Standalone package at `internal/sfu/`. Provides a provider interface so multiple SFU backends can be plugged in.

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

`sfu.NewProvider(cfg)` reads `cfg.SFUProvider` (from `SFU_PROVIDER` env) and returns the matching implementation. Currently only `"livekit"` is supported.

### Usage in handlers / services

All SFU calls go through `sfu.Provider`. To add a new SFU backend, implement `sfu.Provider` and register it in `factory.go`.

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

Provides OAuth2 third-party login (GitHub / Google / QQ). Two layers:

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

`oauth.GetDefaultConfig(name)` returns preset endpoint configs for each platform (ClientID/Secret/RedirectURL must be injected).

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
| `JWTAuth()` | Validates Bearer token: checks header → signature → expiry → blacklist (JTI) |
| `RequireRole(roles ...string)` | Checks `role` claim from JWT against allowed roles. Returns `FORBIDDEN` (1013) on mismatch |
| `CORS()` | Sets `Access-Control-Allow-Origin: *`, handles OPTIONS preflight |

### JWTAuth check order

1. Header exists → `TOKEN_NOT_EXIST` (1001)
2. Signature valid → `TOKEN_WRONG` (1002)
3. Not expired → `TOKEN_EXPIRED` (1003)
4. JTI not blacklisted → `TOKEN_REVOKED` (1014)
5. Inject `username`, `user_uuid`, `role`, `claims` into context

---

## Models

| Model | Table | Key Fields |
|-------|-------|------------|
| `User` | `users` | ID, UUID (auto-gen), Name, Password (`json:"-"`), Role, CreatedAt, UpdatedAt |
| `Room` | `room` | ID, UUID (auto-gen), Name, Limit, CreatedAt, UpdatedAt |
| `UserGroup` | `user_groups` | ID, UserID, GroupName, CreatedAt, UpdatedAt |
| `OAuthProvider` | `oauth_providers` | ID, Name, ClientID, ClientSecret, AuthURL, TokenURL, UserInfoURL, RedirectURL, Scopes, Enabled |
| `OAuthAccount` | `oauth_accounts` | ID, UserID, Provider, ProviderUID, AccessToken, RefreshToken |

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
hub := signal.NewHub()
hub.RegisterHandlers(server)   // registers all Socket.IO events
hub.BroadcastToRoom(namespace, room, event, data)
```

---

## Frontend Architecture

```
packages/web/src/
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
cd packages/server
pnpm test

# Or from monorepo root:
pnpm test:server
```

Tests send real HTTP requests and validate responses. Add new test files under `test/<module>/<module>.test.js`.

---

## Common Commands

```bash
# Start server (dev mode, SQLite)
cd packages/server
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
3. Set `SFU_PROVIDER="<provider>"` in `.env.dev`
4. No handler or router changes needed — all SFU calls go through `sfu.Provider`

---

## Test Logging

当 agent 被命令进行测试时，必须将测试总结的结果以 Markdown 格式保存到 `agent_test_logs` 文件夹。

### 命名规范

文件名格式：`{测试内容}-{时间}.md`

示例：
- `api-auth-test-2026-05-26.md`
- `role-permission-test-2026-05-26.md`
- `user-crud-test-2026-05-26-14-30.md`

### 日志内容模板

```markdown
# {测试标题}

**测试时间**: YYYY-MM-DD HH:MM:SS
**测试环境**: dev / prod
**测试人员**: AI Agent

## 测试概要

简要描述本次测试的目的和范围。

## 测试结果

| 测试项 | 状态 | 说明 |
|--------|------|------|
| 测试项1 | ✅ 通过 | 成功说明 |
| 测试项2 | ❌ 失败 | 失败原因 |

## 详细测试记录

### 1. 测试项名称

**请求**:
```bash
curl -X POST http://localhost:8998/api/v1/xxx
```

**响应**:
```json
{
  "code": 0,
  "msg": "success",
  "data": { ... }
}
```

**结论**: 测试通过/失败，原因说明。

## 问题与建议

列出测试过程中发现的问题和改进建议。

## 总结

整体测试结论和下一步计划。
```

### 保存位置

```
GOSpeak/
├── agent_test_logs/           # 测试日志目录
│   ├── api-auth-test-2026-05-26.md
│   ├── role-permission-test-2026-05-26.md
│   └── ...
└── ...
```
