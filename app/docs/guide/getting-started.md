# 快速开始

## 前置依赖

| 工具 | 最低版本 | 说明 |
|------|---------|------|
| Go | 1.24+ | 后端编译 |
| Node.js | 20+ | 前端构建 |
| pnpm | 10+ | monorepo 包管理 |
| Docker & Compose | 最新 | 可选，用于部署 SFU 或全栈 |
| GCC / musl-dev | — | SQLite 需要 CGO |

## 5 分钟本地开发

### 1. 克隆并安装依赖

```bash
git clone https://github.com/<your>/GOSpeak.git
cd GOSpeak

pnpm install
```

### 2. 配置环境变量

```bash
cp app/server/.env.dev app/server/.env.dev
# 编辑 app/server/.env.dev，至少填一个 SFU 凭证
```

最小配置（SRS 自建，推荐本地开发）：

```env
DB_TYPE="SQLite"
SFU_PROVIDER="srs"
SRS_HOST="localhost"
SRS_API_PORT="1985"
SRS_SECRET="your-random-hex-secret"
SRS_WHIP_URL="/rtc/v1/whip/"
```

或最小配置（LiveKit Cloud）：

```env
DB_TYPE="SQLite"
SFU_PROVIDER="livekit"
LIVEKIT_HOST="wss://your-instance.livekit.cloud"
LIVEKIT_KEY="your-api-key"
LIVEKIT_SECRET="your-api-secret"
```

### 3. 启动后端依赖（SFU + 可选服务）

```bash
# SRS 模式：启动 SRS 和 MinIO（可选）
docker compose -f deploy/docker-compose.yml --profile srs up -d

# 或 LiveKit 模式：启动 LiveKit
docker compose -f deploy/docker-compose.yml --profile livekit up -d
```

> 如果宿主机已有 SRS / LiveKit 实例，可跳过此步，直接配置环境变量指向它即可。

### 4. 启动开发服务

```bash
# 同时启动前后端（后端 air 热重载 + 前端 vite）
pnpm start:dev
```

- 后端 API: `http://localhost:8098`（或 `SERVER_PORT` 指定的端口）
- 前端页面: `http://localhost:5173`（vite dev server）
- Swagger 文档: `http://localhost:8098/swagger/index.html`
- 健康检查: `http://localhost:8098/ping`

### 5. 注册用户并登录

1. 打开前端页面
2. 点击「注册」，创建第一个用户
3. 注册完成后自动登录，进入主界面
4. 创建房间 → 加入语音频道或文字聊天

## 目录结构概览

```
GOSpeak/
├── app/
│   ├── server/              # Go 后端
│   │   ├── main.go          # 入口点
│   │   ├── cmd/             # Cobra CLI
│   │   ├── server/gin.go    # DI 容器
│   │   ├── internal/
│   │   │   ├── config/      # 环境配置读取
│   │   │   ├── handler/     # HTTP 控制器
│   │   │   ├── service/     # 业务逻辑
│   │   │   ├── repository/  # DAO 数据访问
│   │   │   ├── model/       # GORM 数据模型
│   │   │   ├── router/      # 路由注册
│   │   │   ├── middleware/  # JWT/CORS/权限 RBAC
│   │   │   ├── sfu/         # SFU 抽象 + 工厂 + 多 Provider
│   │   │   ├── signal/      # WebSocket 信令 Hub
│   │   │   ├── jobs/        # 异步任务
│   │   │   ├── storage/     # 对象存储抽象
│   │   │   ├── authstate/   # JWT 认证状态 (NATS KV)
│   │   │   ├── permcode/    # 权限码常量
│   │   │   ├── plugin/      # 插件系统
│   │   │   └── pkg/         # 工具包 (JWT, OAuth, errors)
│   │   └── test/            # API 集成测试
│   ├── web/                 # SolidJS 前端
├── deploy/                  # Docker Compose 编排
├── packages/
│   └── sfu-client/          # 前端 SFU 客户端抽象
└── docs/                    # 项目设计文档
```

## 常用命令

```bash
pnpm install              # 安装依赖
pnpm start:dev            # 同时启动前后端（开发模式）
pnpm dev:server           # 仅启动后端（air 热重载）
pnpm dev:web              # 仅启动前端（vite）
pnpm build:server         # 构建 Go 二进制
pnpm test:server          # 运行后端集成测试
docker build -t gospeak . # 构建 Docker 镜像
docker pull ghcr.io/noelorin/gospeak:1  # 拉取官方发布镜像（或本地 docker build）
```

## 下一步

- [SFU 配置 →](/sfu/) 了解不同音视频引擎的配置方式
- [Docker Compose 部署 →](/deployment/docker-compose) 一键部署到服务器
- [环境变量参考 →](/guide/configuration) 完整配置项说明
