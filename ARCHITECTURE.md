# GOSpeak 项目架构图

## 整体架构

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              GOSpeak Monorepo                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐                              │
│  │    app/web      │    │   app/server    │                              │
│  │   (React 19)    │    │      (Go)       │                              │
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
│  ┌──────────┐                                                                   │
│  │ LiveKit  │                                                                   │
│  │   SFU    │                                                                   │
│  └──────────┘                                                                   │
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐              │
│ │  Embedded NATS  │    │    Database     │    │     OAuth       │              │
│ │ (Event Bus + KV)│    │  (SQLite/PG/My) │    │  (GitHub/GG/QQ) │              │
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
│   │   │   │   └── providers/  # livekit 实现
│   │   │   ├── signal/      # WebSocket 信令 Hub
│   │   │   ├── authstate/   # JWT 认证状态 (NATS KV)
│   │   │   └── pkg/         # 共享工具 (errors, response, jwt, oauth)
│   │   └── db/              # SQLite 数据库存储
│   ├── web/                 # React 19 前端 (@gospeak/web)
│   │   └── src/
│   │       ├── api/         # apiClient (axios) + auth API
│   │       ├── pages/       # 文件路由 → routeTree.gen.ts
│   │       ├── components/  # 业务组件 (通用 UI 来自 packages/components；common/ 含错误边界)
│   │       ├── hooks/       # React hooks (media / useAuthParallax / useUpload / useErrorHandler)
│   │       ├── layouts/     # 布局 (layout.tsx + common/)
│   │       ├── stores/      # zustand (useXxxStore hook + xxxStore 门面；IndexedDB 持久化)
│   │       └── types/       # TypeScript 类型定义
│   ├── desktop/             # 桌面端宿主 (Go + MyGo，内嵌前端产物)
│   │   ├── main.go          # 入口：mygo.Bind 绑定原生能力服务
│   │   ├── windows.go       # 主窗口 (隐藏标题栏、保留系统按钮)
│   │   ├── proxyserver.go   # 本地回环反代（前端 + API/WS 同源）
│   │   └── overlay.go       # 游戏内覆盖层浮窗
│   └── docs/                # VitePress 文档站 (guide / sfu / deployment / architecture)
├── packages/
│   ├── components/          # UI 组件库 (Radix + 主题 token)
│   ├── sfu-client/          # 前端 SFU 客户端抽象
│   └── desktop/             # 桌面能力 SDK (MyGo 桥接 + 条件编译 + 浏览器兜底)
├── test/                    # API 集成测试 (@gospeak/test, Vitest + Node，仓库根)
├── package.json             # Root scripts
├── pnpm-workspace.yaml      # Workspace config
└── AGENTS.md                # AI Agent 开发指南
```

## 前端架构 (app/web)

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                          React 19 Application                                   │
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
│  │  ┌──────────────┐  ┌──────────────┐                                    │   │
│  │  │ ErrorBoundary│  │ ErrorFallback │ ← 错误边界 / 兜底                  │   │
│  │  └──────────────┘  └──────────────┘                                    │   │
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
- Framework: React 19 + @tanstack/react-router + @tanstack/react-query
- State: zustand (useXxxStore hook + xxxStore 门面) + IndexedDB (idb-keyval)
- UI: packages/components (Radix + shadcn 风格, theme.css) + Tailwind CSS 4；图标 lucide-react；toast sonner；表单 react-hook-form + zod
- Real-time: 原生 WebSocket + packages/sfu-client 前端 SFU 客户端
- HTTP: Axios
- Build: rolldown-vite 7 (@vitejs/plugin-react / @tanstack/router-plugin/vite / @tailwindcss/vite / @tanstack/devtools-vite)
```

### 前端错误处理

前端建立三层错误边界 + 全局兜底 + 统一上报：

- **组件级 `ErrorBoundary`**（`components/common/errorBoundary.tsx`）：包裹局部 UI，支持 `fallback`、`resetKeys`、`onReset`、`onError`、`report`，并导出 HOC `withErrorBoundary()`。
- **页面级 `PageErrorBoundary`**（同文件）：全屏兜底，提供重试 / 刷新 / 返回首页 / 折叠错误详情 / 复制错误，生产环境隐藏堆栈。
- **路由级 `RouteErrorComponent`**（同文件）：接入 TanStack Router `errorComponent`，覆盖渲染 / loader / beforeLoad 错误。
- **`useErrorHandler()`**（`hooks/useErrorHandler.ts`）：把 `Promise.catch` / 事件回调里的异步错误重抛到渲染阶段，交给最近边界（React 边界本身捕获不到异步错误）。
- **全局兜底 `installGlobalErrorHandlers()`**（`utils/errorReport.ts`）：监听 `window.onerror` + `unhandledrejection`。
- **统一上报 `errorReport.ts`**：`normalizeError` / `registerErrorReporter`（可接 Sentry）/ `reportError` / `errorSummary`，`ErrorScope = component | page | route | global`。
- **接入点**：`main.tsx` 注册 `defaultErrorComponent` + 全局监听（重试时 `router.invalidate()`）；`pages/__root.tsx` 的 `errorComponent: RouteErrorComponent`；`pages/(app)/route.tsx` 页面级边界包 Layout+Outlet（`resetKeys=[pathname]`）；`dynamicRender.tsx` / `domainWorkspace.tsx` / `layout.tsx` 组件级边界各包一层。

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
- Language: Go 1.26
- HTTP: Gin
- Database: GORM (SQLite/PostgreSQL/MySQL)
- Shared state: Embedded NATS JetStream KV (黑名单、密钥轮换、房间状态)
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
│                                    │                                           │
│                                    ▼                                           │
│  ┌──────────┐                                                                  │
│  │ LiveKit  │                                                                  │
│  │ Highest  │                                                                  │
│  │ Full API │                                                                  │
│  └──────────┘                                                                  │
│  配置: SFU_PROVIDER="livekit"                                                    │
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
│       │                                    └─── Embedded NATS KV ──► State    │
│       │                                          (Blacklist/Key Rotation)      │
│       │                                                                         │
│       ├─── WebSocket (WebSocket) ─────► Go Server (Signal Hub)                 │
│       │                                    │                                    │
│       │                                    ├─── Memory ──► Room Registry       │
│       │                                    │                                    │
│       │                                    └─── SFU Provider ──► SFU Server    │
│       │                                          (LiveKit)                    │
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
│  │                        SFU Backend                                      │   │
│  │                                                                         │   │
│  │  LiveKit: ┌──────────────────┐                                          │   │
│  │           │  livekit-server  │                                          │   │
│  │           │  :7880 (HTTP)    │                                          │   │
│  │           │  :7881 (WebRTC)  │                                          │   │
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
│   ├── POST /token                    # 获取加入房间 Token (JWT)
│   ├── POST /webhook                  # LiveKit Webhook (公开)
│   ├── GET  /rooms                    # 列出房间 (JWT)
│   ├── GET  /participants             # 列出参与者 (JWT)
│   └── GET  /ws-endpoint              # WebSocket 连接地址 (JWT)
│
├── /sfu
│   ├── POST /config                   # 获取 SFU 配置 (sfu:manage)
│   ├── POST /config/:provider         # 指定 provider 配置 (sfu:manage)
│   ├── POST /update-config            # 更新 SFU 配置 (sfu:manage)
│   ├── POST /switch-provider          # 切换激活 provider (sfu:manage)
│   └── POST /providers                # 列出全部 provider 配置 (sfu:manage)
│
├── /user
│   ├── POST /profile                  # 获取个人资料 (需认证)
│   ├── POST /info                     # 按用户名查用户 (需认证)
│   ├── GET/POST /preset-avatars       # 预设头像 (需认证)
│   ├── POST /update-profile           # 更新资料 (需认证)
│   ├── POST /list                     # 列出用户 (user:read)
│   ├── POST /get                      # 获取用户 (user:read)
│   ├── POST /delete                   # 删除用户 (user:delete)
│   └── POST /update-role              # 更新角色 (user:update)
│
├── /user-group                        # 用户分组 CRUD (需认证)
│
├── /guest                             # 访客 (公开签发 60/h + 受保护管理)
│   ├── POST /auth/guest               # 访客签发 (公开)
│   ├── POST /auth/guest/renew         # 访客续约 (guest JWT)
│   └── POST /domain/guest/*           # 配置/封禁/清理 (需认证)
│
├── /domain                            # 语音服务器（多租户）
│   ├── POST /create                   # 创建域 (domain:create, 10/min)
│   ├── POST /list /list-public /my-domains /preview
│   ├── POST /join /leave /kick /members
│   ├── POST /update /delete /reset-invite
│   ├── POST /guest/config|ban|unban|ban-list|cleanup
│   └── POST /roles/* /my-permissions  # 域角色与权限
│
├── /room                              # 房间 CRUD
│   ├── POST /create /list /get /update /delete
│   └── POST /messages/list|search|react  # 文字房间消息
│
├── /conversation                      # 私聊
│   ├── GET  /list                     # 会话列表
│   └── POST /messages /mark-read /send
│
├── /message                           # 消息（挂 /room）
│
├── /mute                              # 全局禁言 (mute:manage)
│   └── POST /create /cancel /status /list
│
├── /role /permission                  # 平台角色与权限 (role:read/manage)
│
├── /storage                           # 对象存储 (presign/confirm/upload/delete/config)
│
├── /email /email_config               # 邮箱验证码 + SMTP 配置
│
├── /audit                             # 审计日志 (audit:read)
│
├── /system
│   └── GET /stream                    # 服务状态流 (admin)
│
├── /readyz                            # 就绪探针
├── /metrics                           # 指标 (METRICS_TOKEN)
├── /ping                              # 健康检查
├── /uploads/*filepath                 # 头像/上传静态服务
└── /swagger/*any                      # Swagger UI
```

> 完整路由表（方法/鉴权/权限门/Handler）见根 `AGENTS.md`「Current Route Table」。

## WebSocket 事件 (WebSocket)

单一事实源在 `internal/protocol`（Catalog 声明事件名/方向/Ack/payload struct），
由 `cmd/gen-protocol` 生成前端 `src/protocol/generated/`；事件名与 payload 类型
禁止在任何其他地方手写。payload 字段一律 snake_case。

```
Client → Server (Ack 事件返回纯 data，错误走 error ACK):    Server → Client:
──────────────────────────────────────────────           ────────────────

Room Management:
├── room:create    { room, domain_uuid? }  ──► ack RoomInfo
├── room:join      { room, domain_uuid?, password? } ──► ack
├── room:join:sfu  { room, stream?, domain_uuid? }   ──► ack
├── room:leave     { room, domain_uuid? }            ──► ack
├── room:list      { domain_uuid? }                  ──► ack { rooms[] }
└── room:kick      { room, target_identity, domain_uuid? }

Member / Presence:
├── member:speaking   { identity, speaking } (C→S 聚合上报)
├── member:mic-state  { room, is_mic_muted, domain_uuid? }
                                            ├── room:updated      RoomInfo
                                            ├── room:active-speakers { identities[] }
                                            ├── member:joined / member:left
                                            ├── member:updated    MemberInfo
                                            ├── member:muted / member:unmuted
                                            └── room:kicked       { target_identity }

Mute / Restriction:
                                            ├── user:muted        MuteInfo
                                            └── user:unmuted      MuteInfo

消息 / 私聊 / 公告：
├── message:send/edit/delete/react/unreact ──► ack
│                                          ├── message:created/updated/deleted
│                                          └── message:reaction
├── private:send     ──► ack                ├── private:new
│                                          └── announcement:new
└── typing           { room, typing }

SFU:
                                            └── sfu:provider-changed { provider }

WS Endpoint: /ws（HttpOnly access cookie 鉴权，子协议仅传 `gospeak`）
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
| 1015 | `USER_BANNED` | 用户已被封禁 |
| 1016 | `USER_MUTED` | 用户已被禁言 |
| 1017 | `RATE_LIMITED` | 请求过于频繁 |
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
| 8001-8010 | `EMAIL_*` | 邮箱验证相关（见 `pkg/errors.go`） |
| 8101-8104 | `STORAGE_*` | 对象存储相关 |

## 技术栈总结

| 层级 | 技术 | 用途 |
|------|------|------|
| **前端框架** | React 19 | 响应式 UI |
| **路由** | @tanstack/react-router | 文件路由 (pages → routeTree.gen.ts) |
| **数据获取** | @tanstack/react-query | 服务端状态管理 |
| **状态管理** | zustand | 客户端状态 (useXxxStore hook + xxxStore 门面) |
| **UI/样式** | packages/components (Radix + shadcn 风格) + Tailwind CSS 4 | 主题 token theme.css |
| **实时通信** | WebSocket | 信令服务器 |
| **WebRTC** | LiveKit | 音视频通信 |
| **后端语言** | Go | 服务器逻辑 |
| **HTTP 框架** | Gin | REST API |
| **ORM** | GORM | 数据库操作 |
| **数据库** | SQLite / PostgreSQL / MySQL | 数据持久化 |
| **事件总线** | 内嵌 NATS JetStream | 内部事件/黑名单/密钥轮换/房间状态 |
| **认证** | JWT + OAuth2 (GitHub/Google/QQ) | 用户认证 |
| **构建工具** | rolldown-vite 7 | 前端打包 |
| **桌面端** | Go 1.27 + MyGo 0.8 + 系统 WebView | 原生宿主，内嵌同一套前端；条件编译裁剪平台模块 |
| **包管理** | pnpm | Monorepo 管理 |
| **容器化** | Docker Compose | 部署编排 |

## 核心设计原则

1. **分层架构**: Handler → Service → Repository，单向依赖
2. **SFU 抽象**: 基于 Provider 接口统一接入 LiveKit
3. **内嵌事件总线**: NATS 固定进程内嵌运行（含 JetStream），无外部消息队列依赖
4. **渐进式数据库**: SQLite 开箱即用 → PostgreSQL/MySQL 可扩展
5. **统一响应**: 所有 API 使用相同 JSON 格式
6. **错误处理**: Service 层返回 `*AppError`，Handler 层统一响应

## 内嵌 NATS 事件总线

GOSpeak 固定以进程内嵌方式运行 NATS（不支持外部 `NATS_URL`）：

- 启动时随进程拉起 `nats-server`（含 JetStream；默认随机端口，`NATS_EMBEDDED_PORT` 可固定）
- Hub 广播（`member:*` / `room:*` / `user:muted` 等）经 WebSocket Deliverer 投递到本机连接
二进制仍为单文件：链接 `nats.go` 客户端 + `nats-server` 库，不附带独立 nats-server 可执行文件。

内部事件使用 `PublishInternal`（subject `{prefix}.internal`），例如权限缓存失效 `cache:permissions-invalidated`，不经 WebSocket Deliverer。

房间人数视图：membership JetStream KV 为进程内共享状态；join/leave 写 KV 后 `PublishInternal(state:room-changed)` 触发本地状态合并与 `room:updated`/`room:list:result` 广播。


