# GOSpeak 部署/运维手册

> 版本: 1.0 · 更新: 2026-07-03

GOSpeak 实时音视频平台的部署文档。覆盖本地开发、Docker 部署、生产上线全流程。

---

## 1. 环境要求

| 组件 | 版本 | 用途 |
|------|------|------|
| Go | 1.24+ | 后端编译 (需要 CGO, 因 SQLite) |
| Node.js | 22+ | 前端构建 (Vite) |
| pnpm | 10+ | monorepo 包管理 |
| Docker | 24+ | 容器化部署 |
| Docker Compose | v2 | 多服务编排 |

可选:
- Redis 7 — Token 黑名单、JWT 密钥轮换（缺失则优雅降级）
- PostgreSQL / MySQL — 生产数据库（默认 SQLite）

---

## 2. 快速开始（本地开发）

### 2.1 安装依赖

```bash
pnpm install
```

### 2.2 启动依赖服务

```bash
docker compose -f deploy/docker-compose.example.yml up -d
```

默认启动: LiveKit + Redis + MinIO。MediaSoup/SRS 见文件内注释启用。

### 2.3 启动后端

```bash
pnpm dev:server
# 等价: cd app/server && air
# air 会热重载，环境由 -e dev 指定 (读 .env.dev)
```

### 2.4 启动前端

```bash
pnpm dev:web
# 等价: cd app/web && vite --force
```

或一键同时启:
```bash
pnpm start:dev
```

访问: http://localhost:5173 (Vite 默认)

---

## 3. 环境变量说明

配置完全通过环境变量驱动，无独立配置文件。开发用 `.env.dev`，生产用 `.env.prod`。

`app/server/main.go` 启动时 `godotenv.Load()` 读取 `.env`（当前工作目录）。`server -e dev|prod` 切换环境枚举。

### 3.1 数据库

| 变量 | 说明 | 示例 |
|------|------|------|
| `DB_TYPE` | `SQLite` / `PostgresSQL` / `MYSQL` | `SQLite` |
| `DB_HOST` | DB 主机 (非 SQLite) | `localhost` |
| `DB_PORT` | DB 端口 | `5432` |
| `DB_USER` | DB 用户 | `gospeak` |
| `DB_PASSWORD` | DB 密码 | `***` |
| `DB_PATH` | SQLite 文件路径 | `db/gospeak.db` |

### 3.2 Redis（可选）

留空 `REDIS_HOST` 则跳过 Redis 连接，JWT 用静态密钥。

| 变量 | 说明 | 示例 |
|------|------|------|
| `REDIS_HOST` | Redis 主机，空则禁用 | `localhost` |
| `REDIS_PORT` | 端口 | `6379` |
| `REDIS_PASSWORD` | 密码 | `""` |
| `REDIS_DB` | DB 编号 | `0` |
| `JWT_KEY_TTL` | JWT 密钥轮换周期，需 Redis | `24h` |

### 3.3 SFU Provider

通过 `SFU_PROVIDER` 选择音视频后端。三选一。

**LiveKit（默认）:**

| 变量 | 说明 |
|------|------|
| `SFU_PROVIDER` | `livekit` |
| `LIVEKIT_HOST` | `wss://<id>.livekit.cloud` (Cloud) 或 `ws://localhost:7880` (自部署) |
| `LIVEKIT_KEY` | API Key |
| `LIVEKIT_SECRET` | API Secret |

**MediaSoup:**

| 变量 | 说明 |
|------|------|
| `SFU_PROVIDER` | `mediasoup` |
| `MEDIASOUP_BRIDGE_URL` | `http://localhost:3001` |
| `MEDIASOUP_HOST` | `localhost:3001` |

**SRS:** 见 `sfu-provider-maturity.md` 及对应客户端文档。

### 3.4 对象存储

| 变量 | 说明 | 示例 |
|------|------|------|
| `STORAGE_TYPE` | `local` / `s3` | `local` |
| `STORAGE_ENDPOINT` | S3 端点 (MinIO/R2) | `http://localhost:9000` |
| `STORAGE_BUCKET` | 桶名 | `gospeak` |
| `STORAGE_REGION` | 区域 | `us-east-1` |
| `STORAGE_ACCESS_KEY` | 访问密钥 | `minioadmin` |
| `STORAGE_SECRET_KEY` | 密钥 | `minioadmin` |
| `STORAGE_PATH_PREFIX` | 路径前缀 | `uploads/` |
| `STORAGE_PUBLIC_BASE_URL` | 公开域名 (CDN) | `https://cdn.example.com` |
| `STORAGE_ENCRYPT_KEY` | 凭证加密密钥，64 位 hex（生产必设） | `a1b2...` |

⚠️ `STORAGE_ENCRYPT_KEY` 生产环境必须设置，否则凭证明文存储。

---

## 4. Docker 部署

### 4.1 依赖服务 — 渐进式数据库三档

`deploy/docker-compose.example.yml` 用 Compose `profiles` 提供三档组合，按规模选择：

| 档 | 命令 | 应用 `.env` 关键项 |
|----|------|---------------------|
| A 单 SQLite | `docker compose -f deploy/docker-compose.example.yml up -d` | `DB_TYPE=SQLite`, `REDIS_HOST=""` |
| B 单 PostgreSQL | `docker compose -f deploy/docker-compose.example.yml --profile postgres up -d` | `DB_TYPE=PostgresSQL`, `DB_HOST=postgres`, `REDIS_HOST=""` |
| C PG + Redis | `docker compose -f deploy/docker-compose.example.yml --profile postgres-redis up -d` | `DB_TYPE=PostgresSQL`, `DB_HOST=postgres`, `REDIS_HOST=redis` |

- **A 档**：零外部 DB，SQLite 内嵌，最小起步
- **B 档**：单一 PostgreSQL，中等规模
- **C 档**：PostgreSQL + 应用 Redis，启用 JWT 轮换与 Token 黑名单，生产推荐

### 应用层 Redis 复用规则

- **用 LiveKit 时**（默认）→ LiveKit 自身需要 Redis 服务（下方 `redis`），C 档应用直接复用，设 `REDIS_HOST=redis`。不另起独立 Redis。
- **不用 LiveKit**（纯 mediasoup/srs）又要应用 Redis → `docker compose -f deploy/docker-compose.example.yml --profile redis up -d` 起独立 `app-redis`，设 `REDIS_HOST=app-redis`（端口 6380，避让本机 6379）。

PG/Redis 默认凭证见 compose 文件，生产必改。

### 4.1.1 SFU Provider 切换

与 DB 档正交。默认 LiveKit。

MediaSoup/SRS：编辑 `deploy/docker-compose.example.yml` 取消对应注释 + `.env.prod` 设 `SFU_PROVIDER`。

### 4.2 构建应用镜像

```bash
docker build -t gospeak:latest .
```

三阶段构建（见根 `Dockerfile`）:
1. `go-builder` — Go 后端 (CGO=1, 静态链接)
2. `web-builder` — 前端 SPA (pnpm + Vite)
3. 产出 Alpine 镜像，端口 8998，入口 `/app/gospeak`

### 4.3 运行应用

```bash
docker run -d \
  --name gospeak \
  --env-file app/server/.env.prod \
  -p 8998:8998 \
  gospeak:latest
```

应用读 `.env.prod`，默认 `prod` 环境（无需 `-e` flag）。

---

## 5. 生产部署步骤

### 5.1 发布清单 Checklist

- [ ] `.env.prod` 已配置真实密钥（LiveKit、DB、Redis、Storage）
- [ ] `STORAGE_ENCRYPT_KEY` 已设（64 位 hex）
- [ ] `DB_TYPE` 生产环境用 PostgreSQL/MySQL，非 SQLite
- [ ] `REDIS_HOST` 已设（启用 JWT 轮换）
- [ ] 端口防火墙开放: 8998 (应用) + SFU WebRTC UDP/TCP
- [ ] HTTPS 终结已配置（见 Nginx 反代）
- [ ] 数据库备份策略已就位
- [ ] 日志收集已配置

### 5.2 完整上线流程

```bash
# 1. 克隆 + 配置
git clone <repo> && cd GOSpeak
cp app/server/.env.prod app/server/.env.prod.local
# 编辑 .env.prod.local 填入生产密钥

# 2. 拉取依赖镜像
docker compose -f deploy/docker-compose.example.yml up -d

# 3. 构建应用
docker build -t gospeak:latest .

# 4. 运行
docker run -d \
  --name gospeak \
  --env-file app/server/.env.prod.local \
  -p 8998:8998 \
  --restart unless-stopped \
  gospeak:latest

# 5. 验证
curl http://localhost:8998/api/v1/health  # (若该端点存在)
```

### 5.3 Nginx 反代

见 `deploy/nginx.conf`。WebSocket 升级用于 Socket.IO 信令，`/api/` 代理到 Go 后端。

### 5.4 停止

```bash
docker stop gospeak
docker compose -f deploy/docker-compose.example.yml down
```

---

## 6. 运维

### 6.1 日志

```bash
docker logs -f gospeak
```

应用日志由 `logrus` 输出至 stdout（Text 格式，含时间戳）。

### 6.2 健康检查

应用容器需配 `HEALTHCHECK`（见 Dockerfile 加固项）。依赖服务在 `docker-compose.example.yml` 已配 healthcheck。

### 6.3 数据备份

- **SQLite:** 备份 `app/server/db/*.db` 文件
- **PostgreSQL:** `pg_dump`
- **MinIO:** 桶同步至异地
- **Redis:** RDB 快照（AOF 可选）

### 6.4 升级

```bash
git pull
docker build -t gospeak:latest .
docker stop gospeak && docker rm gospeak
docker run -d ... gospeak:latest  # 同 5.2 step 4
```

---

## 7. 故障排查

| 症状 | 排查 |
|------|------|
| 前端无音视频 | 检查 `SFU_PROVIDER` + LiveKit/SRS 服务状态 + WebRTC UDP 端口开放 |
| 信令连不上 | 检查 Socket.IO WebSocket 升级，Nginx 配置 |
| JWT 无法刷新 | 检查 Redis 连接（`REDIS_HOST`） |
| 上传失败 | 检查 `STORAGE_TYPE` + MinIO/S3 凭证 + `STORAGE_ENCRYPT_KEY` |
| DB 连接失败 | `DB_TYPE` 不匹配，PostgreSQL/MySQL 需 `DB_HOST` 等填全 |

---

## 8. 参考

- 架构总览: `ARCHITECTURE.md`
- Agent 指南: `AGENTS.md`
- 多 SFU 方案: `deploy/docker-compose.example.yml`
- 服务成熟度: `docs/sfu-provider-maturity.md`
