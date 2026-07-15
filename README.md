<div align="center">
  <h1>🎙️ GOSpeak</h1>
  <p><strong>开箱即用的游戏语音平台 · 自部署 Discord 语音平替</strong></p>

  <p>
    <a href="https://go.dev" target="_blank"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go" alt="Go" /></a>
    <a href="https://www.solidjs.com" target="_blank"><img src="https://img.shields.io/badge/SolidJS-1.9-2C4F7C?style=flat&logo=solid" alt="SolidJS" /></a>
    <a href="https://www.apache.org/licenses/LICENSE-2.0" target="_blank"><img src="https://img.shields.io/badge/License-Apache%202.0-blue?style=flat" alt="License" /></a>
    <a href="./docs/sfu-provider-maturity.md"><img src="https://img.shields.io/badge/SFU-LiveKit%20%7C%20SRS%20%7C%20MediaSoup%20%7C%20Agora%20%7C%20Daily-blue?style=flat" alt="SFU" /></a>
  </p>

  <p>
    <a href="#特性">特性</a> ·
    <a href="#快速开始">快速开始</a> ·
    <a href="#部署">部署</a> ·
    <a href="#技术栈">技术栈</a> ·
    <a href="#架构">架构</a> ·
    <a href="#语音功能">语音功能</a> ·
    <a href="#faq">FAQ</a>
  </p>

  <br />
</div>

---

**GOSpeak** 是一个自托管的游戏语音平台。支持多种 SFU 后端（LiveKit / SRS / MediaSoup / Agora / Daily），运行时一键切换。提供房间管理、发言检测、成员音量独立调节、权限控制等游戏语音场景功能。

---

## 特性

| 特性 | 说明 |
|------|------|
| 🎮 游戏语音频道 | 创建/加入语音房间，类 Discord 频道体验 |
| 🗣️ 发言检测 | 实时语音活动指示，谁在说话一目了然 |
| 🔊 独立音量控制 | 每个成员单独调节音量 |
| 🔄 多 SFU 切换 | LiveKit / SRS / MediaSoup / Agora / Daily 运行时切换 |
| 🔐 房间权限 | 密码保护、创建者/管理员踢人、角色权限体系 |
| 🔌 多认证方式 | JWT + OAuth2（GitHub / Google / QQ）|
| 🗄️ 渐进式数据库 | 单文件 SQLite → PostgreSQL → PostgreSQL + Redis |

---

## 快速开始

### 前置依赖

- **二进制运行**：无需任何依赖，下载即用
- **源码开发**：Go 1.24+、Node.js 20+、pnpm 10+
- Docker & Docker Compose（可选，也可直接用 SFU 云服务）

### 1 分钟跑起来（单二进制）

Release 页面下载对应平台的 [`gospeak-*` 单文件](https://github.com/NoelOrin/GOSpeak/releases)，前端已内嵌：

```bash
# 下载后
chmod +x gospeak-linux-amd64
./gospeak-linux-amd64 server -e prod
# 浏览器打开 http://<host>:8998 即可使用
```

### 5 分钟开发模式跑起来

```bash
# 1. 克隆
git clone https://github.com/<your>/GOSpeak.git
cd GOSpeak

# 2. 安装依赖
pnpm install

# 3. 复制配置模板
cp app/server/.env.example app/server/.env
# 编辑 app/server/.env，填入 LiveKit 凭证（见下方配置说明）

# 4. 启动依赖服务（LiveKit / Redis / MinIO）
docker compose -f deploy/docker-compose.example.yml up -d

# 5. 启动开发服务（后端 air 热重载 + 前端 vite）
pnpm start:dev
```

### 最小配置示例（app/server/.env）

```env
# 数据库 — SQLite 零依赖
DB_TYPE="SQLite"

# 音视频引擎 — 填你的 LiveKit 凭证
SFU_PROVIDER="livekit"
LIVEKIT_HOST="ws://localhost:7880"
LIVEKIT_KEY="<your-key>"
LIVEKIT_SECRET="<your-secret>"

# 存储（头像/文件）
STORAGE_TYPE="local"
```

> 没有 LiveKit？docker-compose 已在本地跑了一个。`LIVEKIT_KEY` 和 `LIVEKIT_SECRET` 默认值在 `deploy/livekit/livekit.yaml` 中。

### 默认管理员账号

首次启动会自动 seed 管理员账号（仅当库中不存在 `admin` 时）：

| 字段 | 值 |
|------|-----|
| 用户名 | `admin` |
| 默认密码 | `admin123` |

登录后若仍为默认密码，会提示首次改密（`need_change_password=true`）。**生产环境请立即修改默认密码。**


---

## 部署

完整公网/统一编排见 **[deploy/DEPLOY.md](deploy/DEPLOY.md)**（`deploy/docker-compose.yml`）。


### 数据库三档渐进

```bash
# A. SQLite — 开箱即用，零外部服务
docker compose -f deploy/docker-compose.example.yml up -d

# B. PostgreSQL
docker compose -f deploy/docker-compose.example.yml --profile postgres up -d

# C. PostgreSQL + Redis — JWT 密钥轮换 + Token 黑名单
docker compose -f deploy/docker-compose.example.yml --profile postgres-redis up -d
```

对应 `.env` 配置：

```env
# A 档
DB_TYPE="SQLite"

# B 档
DB_TYPE="PostgresSQL"  DB_HOST=postgres  DB_PORT=5432  DB_USER=gospeak  DB_PASSWORD=gospeak

# C 档
DB_TYPE="PostgresSQL"  DB_HOST=postgres  DB_PORT=5432  DB_USER=gospeak  DB_PASSWORD=gospeak
REDIS_HOST=redis
```

### Docker 生产部署（前端已内嵌）

```bash
# 构建一体镜像（Dockerfile 内构建前端 + go:embed）
docker build -t gospeak .

# 运行
docker run -d \
  --env-file app/server/.env.prod \
  -p 8998:8998 \
  -v gospeak-data:/app/db \
  gospeak
```

### nginx 反代

参考 [`deploy/nginx.conf`](./deploy/nginx.conf) 配置 WebSocket 代理和 TLS。

完整部署指南：[`docs/deployment-guide.md`](./docs/deployment-guide.md)

---

## 语音功能

### 频道模型

- **房间**：语音频道单位，支持密码保护、人数上限
- **成员**：加入房间后自动进入语音通道
- **权限**：创建者/管理员可踢人、全员静音

### 语音控制

| 功能 | 说明 |
|------|------|
| 发言检测 | 实时语音活动指示（绿色光环）|
| 麦克风开关 | 按钮控制输入静音 |
| 独立音量 | 每位成员单独调节输出音量（持久化 IndexedDB）|
| 输出静音 | 一键静音所有声音 |
| 管理员控制 | 强制静音/踢出成员 |

### SFU Provider 切换

```env
# SRS
SFU_PROVIDER="srs"

# MediaSoup
SFU_PROVIDER="mediasoup"
MEDIASOUP_BRIDGE_URL="http://localhost:3001"

# Agora Cloud
SFU_PROVIDER="agora"
AGORA_APP_ID="xxx"
AGORA_APP_CERTIFICATE="xxx"

# Daily
SFU_PROVIDER="daily"
```

仅改 `.env`，无需代码改动。SFU Provider 成熟度：[`docs/sfu-provider-maturity.md`](./docs/sfu-provider-maturity.md)

> 注意：env 注入的 SFU 提供商环境变量，仅在初次启动时生效，启动后无法通过 env 切换，需要修改数据库中 `sfu_config` 表的 `provider` 字段。

---

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go + Gin + GORM + go-socket.io |
| 前端 | SolidJS + TypeScript + Vite + TanStack Router + Tailwind v4 |
| SFU | LiveKit（主）/ SRS / MediaSoup / Agora / Daily |
| 数据库 | SQLite / PostgreSQL / MySQL |
| 缓存 | Redis（可选，缺失优雅降级）|
| 存储 | Local / S3（MinIO / R2）|
| 认证 | JWT + OAuth2（GitHub / Google / QQ）|

---

## 架构

### 分层调用

```
请求 → Router → Middleware(JWT+RBAC) → Handler → Service → Repository → DB
                                           ↓         ↓
                                        OAuth       SFU Provider
                                           ↓         ↓
                                       第三方登录   音视频引擎
                                                     ↓
                                                  Signal(WS)
```

### 目录结构

```
app/
├── server/                  # Go 后端
│   ├── main.go              # 入口
│   ├── server/gin.go        # DI 容器
│   └── internal/
│       ├── handler/         # HTTP 控制器
│       ├── service/         # 业务逻辑
│       ├── repository/      # 数据访问
│       ├── signal/          # Socket.IO 信令 Hub（房间/成员/麦控）
│       ├── sfu/             # SFU Provider 抽象层
│       ├── livekit/         # LiveKit 实现
│       ├── model/           # GORM 实体
│       └── pkg/             # JWT/响应/错误/OAuth
│── web/                     # SolidJS 前端
│   └── src/
│       ├── components/room/ # 语音房间（VoiceChat 组件）
│       ├── stores/          # 状态管理
│       └── utils/
│── packages/
│   ├── sfu-client/          # 前端 SFU 客户端抽象
│   └── mediasoup-worker/   # MediaSoup Node 服务
└── deploy/                  # Docker Compose + 配置
```

详见 [`ARCHITECTURE.md`](./ARCHITECTURE.md)。

---

## 开发

```bash
pnpm install            # 安装
pnpm start:dev          # 启动前后端（热重载）

pnpm dev:server         # 仅后端（air）
pnpm dev:web            # 仅前端（vite）

# 单平台二进制（前端自动嵌入）
make linux-amd64-bin

# 全平台
make all
docker build -t gospeak .  # Docker 镜像

pnpm test:server        # 后端集成测试
pnpm format             # 格式化（biome）
```

---

## FAQ

### 需要什么级别服务器？

最低 1 核 1G。推荐 2 核 4G。

### 用户数有限制吗？

无内置限制。受限于 SFU 后端与服务器带宽。理论上2C4G-30Mbps服务器可轻松支持50人以上的语音房间。

### 如何添加新 SFU 后端？

`internal/sfu/` 实现 `Provider` 接口，`factory.go` 注册，改 `SFU_PROVIDER` 环境变量。详见 [`AGENTS.md`](./AGENTS.md)。

---

## 贡献

PR / Issue 欢迎。大改动先开 Issue 讨论。

提交规范：[Conventional Commits](https://www.conventionalcommits.org/)（`feat:` / `fix:` / `refactor:` / `docs:` / `chore:`），commitlint + lefthook 强制。

---

## 相关文档

- [部署指南](./docs/deployment-guide.md)（含[单二进制部署](./app/docs/deployment/binary.md)）
- [架构图](./ARCHITECTURE.md)
- [SFU 成熟度](./docs/sfu-provider-maturity.md)
- [API 路由表](./AGENTS.md)
- [项目缺口](./docs/project-gaps.md)

---

## License

Apache 2.0
