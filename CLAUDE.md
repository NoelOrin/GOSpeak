# CLAUDE.md — GOSpeak 开发者指南

GOSpeak 是基于 WebRTC 的实时音视频沟通平台。pnpm monorepo：Go 后端 + SolidJS 前端，多 SFU Provider 抽象。

详细架构见 `AGENTS.md` 与 `ARCHITECTURE.md`。本文件是日常开发的速查版。

---

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.26 (实际 1.24+) · Gin · GORM · cobra CLI |
| 前端 | SolidJS · TypeScript · Vite · TanStack Router · Tailwind v4 |
| 信令 | WebSocket (GOSpeak/internal/ws) |
| SFU | LiveKit(主) / SRS / Agora / Cloudflare — 抽象层动态解析 |
| DB | SQLite(默认) / PostgreSQL / MySQL — GORM 自动迁移 |
| 缓存 | Redis(可选，缺失优雅降级) — JWT 轮换 + Token 黑名单 |
| 存储 | local / S3(MinIO/R2) |
| 认证 | JWT + OAuth2(GitHub/Google/QQ) |
| 工具链 | pnpm 10 · Turbo · Biome(lint/format) · Vitest(待用) · Lefthook · commitlint |

---

## 目录结构

```
packages/
├── server/          # Go 后端
│   ├── main.go              # 入口 (cobra)
│   ├── cmd/root.go          # server -e dev|prod, version
│   ├── server/gin.go        # DI 容器，组装全部分层
│   ├── internal/
│   │   ├── config/          # 环境变量读取
│   │   ├── model/           # GORM 实体
│   │   ├── repository/      # 数据访问层
│   │   ├── service/         # 业务逻辑层
│   │   ├── handler/         # HTTP 控制器 (Gin)
│   │   ├── middleware/      # JWTAuth, RequireRole, CORS
│   │   ├── router/routes/  # 按模块分组路由
│   │   ├── sfu/             # SFU Provider 抽象 + 工厂 + DynamicProvider
│   │   ├── livekit/         # LiveKit 实现
│   │   ├── signal/          # WebSocket 信令 Hub (14 事件)
│   │   ├── redis/           # 可选 Redis (黑名单/JWT轮换)
│   │   └── pkg/             # errors/response/jwt + oauth 抽象
│   ├── docs/                # swagger.yaml/json
│   └── test/                # Node.js API 集成测试
├── web/             # SolidJS 前端
│   └── src/{api,components,hooks,layouts,stores,types,utils}
├── sfu-client/      # 前端多 SFU 客户端抽象
└── bot/             # Hono 机器人
deploy/              # docker-compose + 各 SFU 配置
docs/                # design + plans + specs
```

### 分层调用流

```
Request → Router → Middleware(JWT+RBAC) → Handler → Service → Repository → DB
                                                    ↓         ↓
                                                   SFU      Redis(可选)
                                                    ↓
                                                 Signal/WS
```

每层只与正下层通信。Service 返回 `*pkg.AppError`，Handler 用 `pkg.HandleError` 转 JSON。

---

## 常用命令

```bash
# 安装
pnpm install

# 启依赖服务 (LiveKit + Redis + MinIO 默认)
docker compose -f deploy/docker-compose.example.yml up -d

# 开发 (同时启后端+前端)
pnpm start:dev

# 单独
pnpm dev:server   # air 热重载，读 .env.dev
pnpm dev:web      # vite --force

# 生产
pnpm prod:server          # go run main.go server -e prod
pnpm build:server         # 输出 gospeak 二进制
docker build -t gospeak .  # 一体镜像，端口 8998 (见根 Dockerfile)

# 测试
pnpm test:server     # Node.js API 集成测试 (需先启动 server)
cd app/web && pnpm test   # vitest (暂无测试文件)

# Lint / Format (前端)
pnpm --filter @go-rtc/web lint
pnpm --filter @go-rtc/web check
pnpm format          # biome format -r

# CI (手动触发)
# GitHub Actions UI → CI (manual) → Run workflow
```

### 启动参数

- `main.go server` — 默认 `prod` 环境
- `main.go server -e dev` — 开发环境 (读 `.env.dev`)
- `main.go version` — 版本号 (v0.1.0)
- 启动时 `godotenv.Load()` 读当前目录 `.env`

---

## 环境变量

完全由 env 驱动，无独立配置文件。详见 `docs/deployment-guide.md` 第 3 节。

关键项:
- `SFU_PROVIDER` = `livekit` | `srs` | `agora` | `cloudflare`
- `DB_TYPE` = `SQLite` | `PostgresSQL` | `MYSQL`
- `REDIS_HOST` 空 → 跳过 Redis，JWT 用静态密钥
- `STORAGE_ENCRYPT_KEY` 生产必设 (64 位 hex)

文件: `.env.dev` / `.env.prod`（`app/server/` 下）

---

## API 规范

### 统一响应

```json
{ "code": 0, "msg": "success", "data": {} }
```

错误时 `data` 恒 `null`。完整状态码表见 `AGENTS.md`（0 成功，1001-1014 认证，2001 参数，3001-3002 资源，5001 系统，6001-6002 SFU，7001-7004 OAuth）。

### 错误处理范式

```go
// Service — 返回 *AppError
return nil, pkg.NewAppError(pkg.NOT_FOUND, "user not found")

// Handler — HandleError 自动映射
pkg.HandleError(c, err)

// 校验 — 直接 Fail
pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
```

### 路由

全部 `/api/v1/` 前缀。完整路由表见 `AGENTS.md`。认证路由在 `protected` 组下走 `middleware.JWTAuth()`。

OpenAPI: `app/server/docs/swagger.yaml`，访问 `/swagger/*any`。

---

## SFU 抽象层

`internal/sfu/`:
- `Provider` 接口 — token/adminToken/rooms/participants/mute/remove/delete/getHost
- `factory.go` — 按 `SFU_PROVIDER` 实例化
- `NewDynamicProvider(resolve)` — 运行时解析，服务层统一调用 `sfu.Provider`，禁止在 handler 泄漏 provider 分支

成熟度: LiveKit(高) > SRS(高) > Agora/Cloudflare(中)

新增 SFU 后端步骤见 `AGENTS.md` §Adding a New SFU Backend。

---

## 代码规范

1. 代码中**不加注释**，除非复杂逻辑必须
2. 代码中**不用 emoji**，仅文档可用
3. Service 永远返回 `*AppError`，Handler 决定响应格式
4. Go 文件 `snake_case`，类型 `PascalCase`
5. import 分三组: 标准库 / 第三方 / 内部，空行分隔
6. Handler 方法接 `*gin.Context`，无返回值
7. Service 方法返回 `(result, error)`，error 必为 `*AppError`
8. Repository 错误可为 `gorm.ErrRecordNotFound`

---

## Git 工作流

- commit 规范: Conventional Commits (`feat:` / `fix:` / `refactor:` / `docs:` / `chore:`), enforced by commitlint + lefthook
- 分支: `main`(生产) / `dev`(开发) / `feature/*`
- PR 前确认 lint + build 通过

---

## 新增功能流程

1. 定义 model → `internal/model/`
2. repository → `internal/repository/`
3. service → `internal/service/`
4. handler → `internal/handler/`
5. 路由 → `internal/router/routes/<module>/routes.go`
6. 注册 → `internal/router/router.go`
7. 依赖注入 → `server/gin.go`
8. 测试 → `test/`
9. 重新生成 Swagger docs

---

## 测试日志

agent 测试需把结果 Markdown 存到 `agent_test_logs/`。命名 `{内容}-{时间}.md`。模板见 `AGENTS.md` §Test Logging。

---

## 关键参考

- 部署: `docs/deployment-guide.md`
- 交付路线: `docs/delivery-roadmap.md`
- 架构图: `ARCHITECTURE.md`
- 完整 Agent 指南(含路由表/事件表/状态码): `AGENTS.md`
- SFU 成熟度: `docs/sfu-provider-maturity.md`
- 功能缺口: `docs/feature-gaps.md`
