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
└── agent.md             # ← This file
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
│   ├── config/             # Config reading from env
│   ├── model/              # Data models (GORM entities)
│   ├── repository/         # DAO layer, direct DB access
│   ├── service/            # Business logic layer
│   ├── handler/            # HTTP controller (Gin handlers)
│   ├── middleware/         # JWT auth, CORS
│   ├── router/             # Route registration
│   ├── livekit/            # LiveKit integration (独立)
│   ├── signal/             # WebSocket/Socket.IO signaling (独立)
│   └── pkg/                # Shared utilities
│       ├── errors.go       # Business error codes + AppError
│       ├── response.go     # Unified JSON response
│       └── jwt.go          # JWT token generation/parsing
├── test/                   # API integration tests (Node.js)
├── docs/                   # Swagger generated docs
├── db/                     # SQLite database storage
├── .env.dev / .env.prod    # Environment config
└── go.mod
```

### Layered Call Flow

```
Request → Router → Middleware → Handler → Service → Repository → DB
                                          ↓
                                       LiveKit (独立)
                                          ↓
                                       Signal/WS (独立)
```

Each layer communicates **only with the layer directly below it**:
- **Handler** receives HTTP request, validates input, calls Service
- **Service** implements business logic, calls Repository and LiveKit
- **Repository** is pure data access, returns GORM errors
- **LiveKit** is a standalone package wrapping LiveKit API
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
| 2001 | `INVALID_PARAMS` | invalid parameters |
| 3001 | `NOT_FOUND` | resource not found |
| 3002 | `ALREADY_EXISTS` | resource already exists |
| 5001 | `INTERNAL_ERROR` | internal server error |
| 6001 | `LIVEKIT_NOT_CONFIGURED` | livekit not configured |
| 6002 | `LIVEKIT_ERROR` | livekit error |

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
| POST | `/api/v1/auth/logout` | Yes | AuthHandler.Logout |
| POST | `/api/v1/auth/refresh` | Yes | AuthHandler.RefreshToken |
| POST | `/api/v1/signal/token` | No | SignalHandler.GetJoinToken |
| POST | `/api/v1/signal/signal` | No | SignalHandler.Signal |
| GET | `/api/v1/signal/rooms` | No | SignalHandler.ListRooms |
| GET | `/api/v1/signal/participants` | No | SignalHandler.ListParticipants |
| GET | `/api/v1/user/profile` | Yes | UserHandler.GetProfile |
| GET | `/api/v1/user/list` | Yes | UserHandler.List |
| GET | `/api/v1/user/:id` | Yes | UserHandler.GetByID |
| DELETE | `/api/v1/user/:id` | Yes | UserHandler.Delete |
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

- Auto-migration is enabled — models are synced on startup in [db.go](file:///Users/noelorin/GOSpeak/packages/server/internal/repository/db.go)

---

## LiveKit Module

Standalone package at `internal/livekit/`. Handles all LiveKit interactions:

- `GenerateToken(room, identity)` — Generate room join token
- `GenerateAdminToken()` — Admin token for room management
- `ListRooms()` / `ListParticipants(room)` — Room queries
- `MuteParticipant()` / `RemoveParticipant()` / `DeleteRoom()` — Room control

Configure via env: `LIVEKIT_HOST`, `LIVEKIT_KEY`, `LIVEKIT_SECRET`

---

## Signal (WebSocket) Module

Standalone package at `internal/signal/`. Handles WebRTC signaling via Socket.IO:

- **Events**: `room:join`, `room:leave`, `room:joined`, `room:left`
- **Connection**: auto on `/socket.io/`
- Uses `googollee/go-socket.io` library

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
5. Register route in `internal/router/router.go`
6. Wire dependencies in `server/gin.go`
7. Add tests in `test/`
8. Regenerate Swagger docs