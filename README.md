<div align="center">
  <h1>🎙️ GOSpeak</h1>
  <p><strong>开箱即用的游戏语音平台 · 自部署 Discord 语音平替</strong></p>

  <p>
    <a href="https://go.dev" target="_blank"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go" alt="Go" /></a>
    <a href="https://react.dev" target="_blank"><img src="https://img.shields.io/badge/React-19-61DAFB?style=flat&logo=react&logoColor=black" alt="React 19" /></a>
    <a href="https://www.apache.org/licenses/LICENSE-2.0" target="_blank"><img src="https://img.shields.io/badge/License-Apache%202.0-blue?style=flat" alt="License" /></a>
    <a href="./docs/sfu-provider-maturity.md"><img src="https://img.shields.io/badge/SFU-LiveKit-blue?style=flat" alt="SFU" /></a>
  </p>

  <p>
    <a href="#特性">特性</a> ·
    <a href="#快速开始">快速开始</a> ·
    <a href="#发布流程">发布流程</a> ·
    <a href="#部署">部署</a> ·
    <a href="#技术栈">技术栈</a> ·
    <a href="#架构">架构</a> ·
    <a href="#语音功能">语音功能</a> ·
    <a href="#faq">FAQ</a>
  </p>

  <br />
</div>

---

**GOSpeak** 是一个自托管的游戏语音平台。基于 LiveKit 提供音视频能力，提供房间管理、发言检测、成员音量独立调节、权限控制等游戏语音场景功能。

---

## 特性

| 特性 | 说明 |
|------|------|
| 🎮 游戏语音频道 | 创建/加入语音房间，类 Discord 频道体验 |
| 🗣️ 发言检测 | 实时语音活动指示，谁在说话一目了然 |
| 🔊 独立音量控制 | 每个成员单独调节音量 |
| 🎧 音视频引擎 | 基于 LiveKit 的 SFU 架构 |
| 🔐 房间权限 | 密码保护、创建者/管理员踢人、角色权限体系 |
| 🔌 多认证方式 | JWT + OAuth2（GitHub / Google / QQ）|
| 🗄️ 渐进式数据库 | 单文件 SQLite → PostgreSQL → MySQL |
| 🛡️ 前端错误边界与全局错误上报（组件/页面/路由三层兜底） | 组件级 ErrorBoundary / 页面级 PageErrorBoundary / 路由级 RouteErrorComponent + 全局兜底与统一上报 |
| 🖥️ 原生桌面客户端 | 托盘常驻、开机自启、全局快捷键、系统通知、游戏内覆盖层；自定义标题栏，与网页版共用一套前端 |

---

## 快速开始

### 前置依赖

- **二进制运行**：无需任何依赖，下载即用
- **源码开发**：Go 1.26+、Node.js 20+、pnpm 10+
- Docker & Docker Compose（可选，也可直接用 SFU 云服务）

### 1 分钟跑起来（单二进制）

Release 页面下载对应平台的 [`gospeak-*` 单文件](https://github.com/NoelOrin/GOSpeak/releases)，前端已内嵌：

```bash
# 下载后
chmod +x gospeak-linux-amd64
./gospeak-linux-amd64 server -e prod
# 浏览器打开 http://<host>:8998 即可使用
```

单二进制直跑 HTTPS + HTTP/2 时，配置 `TLS_CERT` / `TLS_KEY` 后启动即可；两者需同时配置，未配置时自动保持 HTTP/1.1。详见 [`docs/http2-https.md`](./docs/http2-https.md)。

## 发布流程

版本由 `release-please` 自动管理（GitHub Actions）：

1. 功能合并到 `release` 分支后，CI 自动创建/更新 Release PR，版本号按 Conventional Commits 计算：`feat` 升 minor、`fix` 升 patch、`BREAKING CHANGE` 升 major。
2. Review 并合并 Release PR 后，CI 自动打 `vX.Y.Z` tag、创建 GitHub Release，并触发现有 CI 构建 Docker 镜像、上传跨平台二进制。
3. 首次启用前需在 `release` 分支手动打 `v0.1.0` 锚点 tag（首个自动 Release 会从 `v0.2.0` 开始），之后全部自动。

### 5 分钟开发模式跑起来

```bash
# 1. 克隆
git clone https://github.com/<your>/GOSpeak.git
cd GOSpeak

# 2. 安装依赖
pnpm install

# 3. 生成配置（唯一配置文件：deploy/.env）
cd deploy && ./init.sh && cd ..
# 一般无需再手改：密钥已随机生成，LIVEKIT_PUBLIC_URL 默认留空（见下方配置说明）

# 4. 启动依赖服务（LiveKit / RustFS）
docker compose -f deploy/docker-compose.yml up -d

# 5. 启动开发服务（后端 air 热重载 + 前端 vite）
pnpm start:dev
```

### 最小配置示例（deploy/.env）

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

> 没有 LiveKit？docker-compose 已在本地跑了一个。`LIVEKIT_KEY` / `LIVEKIT_SECRET` 的唯一来源是 `deploy/.env`（由 `./init.sh` 随机生成），compose 会注入 LiveKit 容器，`livekit.yaml` 里不含凭据。
>
> 本地开发时应用跑在宿主机上，要连 `localhost:7880`。compose 里 `livekit` 的 7880 已绑到宿主机回环（`127.0.0.1:7880`），直接能连，无需再改端口映射；它只对本机可见，不对公网开放。

### 默认管理员账号

首次启动会自动 seed 管理员账号（仅当库中不存在 `admin` 时）：

| 字段 | 值 |
|------|-----|
| 用户名 | `admin` |
| 默认密码 | `admin123` |

登录后若仍为默认密码，会提示首次改密（`need_change_password=true`）。**生产环境请立即修改默认密码。**


---

## 部署

完整部署入口见 **[deploy/DEPLOY.md](deploy/DEPLOY.md)**（`deploy/docker-compose.yml`）。

部署只需要一个域名、一个上游。**LiveKit 不需要第二个域名、不需要单独证书、也不需要单独的反向代理规则**——应用内置了同源反代 `/livekit`，浏览器信令走你的站点域名，媒体走 UDP 直连。

### 三条命令跑起来

```bash
cd deploy
./init.sh --domain voice.example.com        # 生成 deploy/.env，写入域名并自动启用 edge
docker compose up -d --build                # 启动，档位由 .env 的 COMPOSE_PROFILES 决定
```

`./init.sh --domain voice.example.com` 会写入域名并自动把 `edge` 加进 `COMPOSE_PROFILES`（已有 `app,livekit` 时追加为 `app,livekit,edge`）。跑完直接 `docker compose up -d --build` 即可，无需手改档位。此时 `.env` 里是：

```env
COMPOSE_PROFILES=app,livekit,edge           # edge = 内置 Caddy，自动申请证书
GOSPEAK_DOMAIN=voice.example.com
```

浏览器打开 `https://voice.example.com` 即可。Caddy 会自动处理 HTTPS 与 WebSocket 升级。

**不开 `edge` 的场景**：已有反代（Nginx / Traefik / Dokploy / 宝塔），或纯局域网无域名时，**不要传 `--domain`**（或手动把 `edge` 从 `COMPOSE_PROFILES` 里去掉），反代指向宿主机 `8998`。

### 三种场景

| 场景 | 怎么初始化 | 说明 |
|------|-----------|------|
| 有域名（推荐） | `./init.sh --domain voice.example.com` | 自动得到 `app,livekit,edge`，内置 Caddy 自动 HTTPS |
| 已有反代（Nginx / Traefik / Dokploy / 宝塔） | `./init.sh`（**不传 `--domain`**） | 保持 `app,livekit`，不要开 `edge`；反代指向 `gospeak:8998` 一个上游即可，**不需要给 LiveKit 写任何路由** |
| 局域网 / 无域名测试 | `./init.sh`（不传 `--domain`） | 保持 `app,livekit`，直接访问 `http://<IP>:8998`；想让 Caddy 统一到 80 端口就手动加 `edge`。浏览器麦克风需要 localhost 或 HTTPS |

### 端口与防火墙

只需放行：`80/443`（Caddy 或自有反代）、`7882/tcp` + `7882/udp`（媒体）、`3478/udp`（TURN）。

**不需要 7880 / 7881**：7880 只绑宿主机回环（`127.0.0.1:7880`）供本机调试，不对公网开放；LiveKit 信令经应用同源的 `/livekit` 反代。

### 数据库与可选档位

```env
# A 档：SQLite（默认，零外部服务）
DB_TYPE="SQLite"

# B 档：PostgreSQL（COMPOSE_PROFILES 追加 postgres）
DB_TYPE="PostgreSQL"  DB_HOST=postgres  DB_PORT=5432  DB_USER=gospeak  DB_PASSWORD=gospeak
```

档位可继续叠加 `rustfs`（对象存储）、`observability`（Prometheus + Grafana + Loki）。

### Docker 生产部署（前端已内嵌）

```bash
# 构建一体镜像（Dockerfile 内构建前端 + go:embed）
docker build -t gospeak .

# 运行
docker run -d \
  --env-file deploy/.env \
  -p 8998:8998 \
  -v gospeak-data:/app/db \
  gospeak
```

这样跑时容器里没有 LiveKit，`LIVEKIT_HOST` 需要指向一个应用容器能访问到的 LiveKit 实例。

### 反代与 HTTPS

有域名就用内置 Caddy（`deploy/Caddyfile`）；已有反代就指向 `gospeak:8998`。详见 [`app/docs/deploy/nginx.md`](./app/docs/deploy/nginx.md)。

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

### SFU 配置

```env
SFU_PROVIDER="livekit"
LIVEKIT_HOST="ws://localhost:7880"
LIVEKIT_KEY="xxx"
LIVEKIT_SECRET="xxx"
```

SFU 成熟度：[`docs/sfu-provider-maturity.md`](./docs/sfu-provider-maturity.md)

> 注意：env 注入的 SFU 配置，仅在初次启动时生效，启动后无法通过 env 修改。

---

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go + Gin + GORM + nhooyr.io/websocket |
| 前端 | React 19 + TypeScript + rolldown-vite 7 + @tanstack/react-router + @tanstack/react-query + zustand + Tailwind 4 + packages/components + lucide-react + sonner + react-hook-form + zod |
| SFU | LiveKit |
| 数据库 | SQLite / PostgreSQL / MySQL |
| 事件总线 | 内嵌 NATS JetStream（黑名单、密钥轮换、房间状态、任务队列）|
| 存储 | Local / S3（RustFS / R2）|
| 认证 | JWT + OAuth2（GitHub / Google / QQ）|
| 桌面端 | Go 1.27 + MyGo 0.8 + 系统 WebView（`app/desktop`，内嵌同一套前端） |

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
│       ├── signal/          # WebSocket 信令 Hub（房间/成员/麦控）
│       ├── sfu/             # SFU Provider 抽象层
│       │   └── providers/   # livekit 实现
│       ├── model/           # GORM 实体
│       └── pkg/             # JWT/响应/错误/OAuth
│── web/                     # React 19 前端（@gospeak/web）
│   └── src/
│       ├── pages/           # 文件路由 → routeTree.gen.ts
│       ├── components/      # 业务组件（common/ 含错误边界 ErrorBoundary / PageErrorBoundary / ErrorFallback）
│       ├── components/room/ # 语音房间（VoiceChat 组件）
│       ├── stores/          # zustand 状态管理
│       ├── _solid-legacy/   # 迁移暂存旧代码，不参与构建
│       └── utils/           # errorReport.ts 统一错误上报（installGlobalErrorHandlers）
│── packages/
│   ├── components/          # UI 组件库（Radix + 主题 token）
│   ├── sfu-client/          # 前端 SFU 客户端
│   └── desktop/             # 桌面能力 SDK（MyGo 桥接 + 条件编译）
└── deploy/                  # Docker Compose + 配置
```

详见 [`ARCHITECTURE.md`](./ARCHITECTURE.md)。

---

## 开发

```bash
pnpm install            # 安装
pnpm dev                # turbo 并行启动前后端（热重载）

pnpm dev:server         # 仅后端（air）
pnpm dev:web            # 仅前端（vite）

# 单平台二进制（前端自动嵌入）；make 目标在 app/server/Makefile，需先 cd app/server
make linux-amd64-bin

# 全平台
make all
docker build -t gospeak .  # Docker 镜像

pnpm build              # turbo 全量构建（web/server/docs/packages，带缓存）
pnpm test               # turbo 全量单测（web/sfu-client/components/集成）
pnpm test:server        # 后端集成测试
pnpm lint               # turbo lint（biome + go vet）
pnpm typecheck          # turbo typecheck（tsc --noEmit）
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

- [文档站](./app/docs/index.md)（[桌面客户端](./app/docs/guide/desktop.md) / [SFU 配置](./app/docs/sfu/index.md) / [部署](./app/docs/deployment/index.md)）
- [部署指南](./docs/deployment-guide.md)（含[单二进制部署](./app/docs/deployment/binary.md)）
- [架构图](./ARCHITECTURE.md)
- [SFU 成熟度](./docs/sfu-provider-maturity.md)
- [API 路由表](./AGENTS.md)
- [项目缺口](./docs/project-gaps.md)

---

## License

Apache 2.0
