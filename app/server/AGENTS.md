# server 模块

GoRTC 服务端入口，基于 Gin + WebSocket 的 WebRTC 信令服务器。媒体层通过多 provider SFU 抽象接入，支持 LiveKit、SRS、MediaSoup、Agora、Daily、Cloudflare 六种后端，运行时可切换。

## 目录结构

```
server/
├── main.go               # 程序入口，Swagger 注解定义
├── agent.md              # 模块说明
├── cmd/
│   └── root.go           # Cobra CLI 命令定义（server / version）
├── server/
│   └── gin.go            # Gin 引擎启动、依赖组装、WebSocket 集成
├── internal/
│   ├── config/           # 配置管理（环境变量）
│   ├── model/            # GORM 数据模型
│   ├── repository/       # 数据访问层
│   ├── service/          # 业务逻辑层
│   ├── handler/          # HTTP 请求处理层
│   ├── middleware/       # Gin 中间件（JWT、权限 RBAC、封禁检查）
│   ├── router/           # 路由注册（按模块拆分）
│   ├── sfu/              # SFU 抽象与动态 provider 分发
│   │   └── providers/    # 各 SFU 后端实现
│   │       ├── livekit/  # LiveKit 实现
│   │       ├── agora/    # Agora 实现
│   │       ├── daily/    # Daily 实现
│   │       ├── mediasoup/# MediaSoup 实现
│   │       ├── srs/      # SRS 实现（WHIP/WHEP）
│   │       └── cloudflare/ # Cloudflare Realtime 实现
│   ├── bus/              # 多实例事件总线（NATS/Redis）
│   ├── permcode/         # 权限码常量
│   ├── signal/           # WebSocket 信令中心
│   ├── redis/            # 可选 Redis（黑名单、JWT 密钥轮换）
│   └── pkg/              # 公共工具（错误码、JWT、响应、OAuth、permcode）
├── docs/                 # Swagger 文档
├── ../web/               # Web 前端（SolidJS + Vite + TanStack Router）
└── test/                 # 测试用例
```

## 启动方式

```bash
go run main.go server          # 生产模式
go run main.go server -e dev   # 开发模式
```

## 环境变量（关键项）

| 变量 | 默认值 | 说明 |
|------|--------|------|
| SERVER_PORT | `8998` | HTTP 监听端口 |
| SFU_PROVIDER | `livekit` | SFU 类型：`livekit` / `srs` / `mediasoup` / `agora` / `daily` / `cloudflare` |
| LIVEKIT_HOST | — | LiveKit 服务器地址 |
| LIVEKIT_KEY | — | LiveKit API Key |
| LIVEKIT_SECRET | — | LiveKit API Secret |
| AGORA_APP_ID | — | Agora App ID |
| AGORA_APP_CERTIFICATE | — | Agora App Certificate |
| MEDIASOUP_BRIDGE_URL | `http://localhost:3012` | MediaSoup bridge 地址 |
| SRS_HOST / SRS_API_PORT | `localhost` / `1985` | SRS 管理 API |
| SRS_SECRET | — | SRS token HMAC 密钥（必填）|
| DAILY_API_KEY / DAILY_DOMAIN | — | Daily 凭据 |
| CF_APP_ID / CF_APP_SECRET / CF_STUN_URL | — / — / `stun.cloudflare.com:3478` | Cloudflare 凭据 |
| DB_TYPE | `SQLite` | `SQLite` / `PostgresSQL` / `MYSQL` |
| DB_WAL | `false` | SQLite WAL 模式开关（并发读建议开启）|
| REDIS_HOST | — | Redis 主机（留空则不连接）|
| JWT_KEY | `default-secret` | JWT 签名密钥（生产必须修改）|
| EMAIL_ENABLED | `false` | 启用邮箱验证 |
| STORAGE_TYPE | `local` | `local` / `s3` |

完整变量见根目录 `AGENTS.md` 的「Configuration」章节。

## 依赖关系

```
main.go → cmd/ → server/gin.go
                   ├── internal/repository/ → SQLite/PostgreSQL/MySQL
                   ├── internal/sfu/        → SFU provider 抽象 / 动态分发
                   │   └── providers/       → livekit|agora|daily|mediasoup|srs|cloudflare
                   ├── internal/bus/        → 多实例事件总线（NATS/Redis/fanout）
                   ├── internal/service/    → 业务逻辑层
                   ├── internal/handler/    → HTTP 处理层
                   ├── internal/router/     → 路由注册（含 domain/conversation/message/plugin）
                   ├── internal/signal/     → WebSocket 信令
                   └── internal/redis/      → 可选黑名单 / 密钥轮换
```

## 禁言语义

- 服务端只维护**用户级禁言**。
- 用户级禁言：**允许收听，但不允许发布本地音轨**。
- 不要引入"房间级静音/房间级禁言"作为产品语义。
- `静音` = 前端本地远端轨道静音；`禁言` = 服务端用户级发言限制。
