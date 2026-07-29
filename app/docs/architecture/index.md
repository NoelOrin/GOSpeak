# 项目架构

## 整体架构

```
┌─────────────────────────────────────────────────────┐
│                   浏览器 (SolidJS)                    │
├──────────┬──────────┬───────────────────────────────┤
│  HTTP    │ WebSocket│        WebRTC                  │
│ (Axios)  │(Socket.IO)│     (LiveKit/SRS SDK)         │
└────┬─────┴────┬─────┴───────────────┬───────────────┘
     │          │                     │
     ▼          ▼                     ▼
┌──────────────────────┐     ┌──────────────┐
│   Go Server :8098     │     │              │
│                       │     │   SFU 服务器  │
│  ┌─────────────────┐  │     │  LiveKit/SRS │
│  │ Handler → Service│  │     │  MediaSoup   │
│  │ → Repository     │  │     │   :7880/1985 │
│  └────────┬────────┘  │     └──────────────┘
│           │           │
│  ┌────────▼────────┐  │
│  │   Signal Hub     │  │
│  │  (Socket.IO)     │  │
│  └─────────────────┘  │
│           │           │
└───────────┼───────────┘
            │
     ┌──────┴──────┐
     │   Database   │
     │ SQLite / PG  │
     └─────────────┘
```

## 技术栈

| 层 | 技术 | 用途 |
|----|------|------|
| **前端框架** | SolidJS | 响应式 UI |
| **路由** | TanStack Router | 客户端路由 |
| **数据获取** | TanStack Query | 服务端状态管理 |
| **实时通信** | Socket.IO | 信令服务器 |
| **音视频** | LiveKit / SRS / etc. | WebRTC SFU |
| **后端** | Go + Gin | REST API |
| **ORM** | GORM | 数据库操作 |
| **数据库** | SQLite / PostgreSQL / MySQL | 持久化 |
| **缓存** | Redis（可选）| JWT 黑名单/密钥轮换 |
| **构建** | Vite | 前端打包 |
| **包管理** | pnpm | Monorepo |
| **容器化** | Docker Compose | 部署编排 |

## 后端分层

```
请求 → Router → Middleware(JWT+RBAC) → Handler → Service → Repository → DB
                                         ↓         ↓
                                      OAuth       SFU Provider
                                      (第三方登录)  (音视频引擎)
                                                    ↓
                                                 Signal Hub (WS)
```

### 层间通信规则

- **Handler** → 接收 HTTP 请求，参数校验，调用 Service
- **Service** → 业务逻辑，调用 Repository 和 SFU
- **Repository** → 纯数据访问，返回 GORM errors
- **SFU Provider** → 通过接口抽象，运行时动态解析
- **Redis** → 可选，优雅降级，用于黑名单和密钥轮换

## 目录结构

```
app/
├── server/              # Go 后端
│   ├── main.go          # 入口
│   ├── cmd/             # Cobra CLI
│   ├── server/gin.go    # DI 容器
│   ├── internal/
│   │   ├── config/      # 环境配置读取
│   │   ├── handler/     # HTTP 控制器
│   │   ├── service/     # 业务逻辑
│   │   ├── repository/  # DAO
│   │   ├── model/       # GORM 实体
│   │   ├── router/      # 路由注册
│   │   ├── middleware/  # JWT/CORS/权限 RBAC/封禁
│   │   ├── sfu/         # SFU 抽象 + 工厂 + 多 Provider
│   │   ├── signal/      # Socket.IO 信令 Hub
│   │   ├── bus/         # 内存事件总线
│   │   ├── jobs/        # 异步任务 (消息持久化等)
│   │   ├── storage/     # 对象存储抽象
│   │   ├── redis/       # Redis 客户端
│   │   ├── permcode/    # 权限码常量
│   │   ├── plugin/      # 插件系统
│   │   ├── logger/      # 日志
│   │   ├── webui/       # 嵌入式前端静态文件
│   │   └── pkg/         # 工具包 (JWT, OAuth, errors)
│   └── test/            # API 集成测试
├── web/                 # SolidJS 前端 (Vite + TanStack Router)
│   └── src/
│       ├── api/         # HTTP 客户端
│       ├── stores/      # 状态管理 (chat/voice/audio/theme/user/socket)
│       ├── components/  # UI 组件 (room/textRoom/manage/modal/chat...)
│       ├── hooks/       # 业务 Hook
│       ├── layouts/     # 布局组件
│       ├── pages/       # 路由页面
│       ├── socket/      # Socket.IO 客户端
│       ├── types/       # TypeScript 类型
│       └── utils/       # 工具函数
│   └── docs/            # VitePress 文档站点
├── packages/
│   ├── sfu-client/      # 前端多 SFU 客户端抽象
│   ├── bot/             # Hono 机器人
│   └── mediasoup-worker/ # MediaSoup Worker
└── docs/                # 项目设计文档
```

## SFU 抽象层

```go
type Provider interface {
    GenerateToken(room, identity string) (string, error)
    GenerateAdminToken() (string, error)
    ListRooms() ([]RoomSummary, error)
    ListParticipants(room string) ([]ParticipantSummary, error)
    MuteParticipant(room, identity, trackSid string, muted bool) error
    RemoveParticipant(room, identity string) error
    DeleteRoom(room string) error
    GetHost() string
}
```

所有 SFU 操作通过此接口统一调用。`sfu.NewDynamicProvider(resolve)` 在运行时动态解析当前使用的 Provider。

详细架构参考项目根目录的 [ARCHITECTURE.md](https://github.com/your/GOSpeak/blob/main/ARCHITECTURE.md)。
