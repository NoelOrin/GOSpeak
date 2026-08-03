# Docker Compose 渐进式部署

GOSpeak 提供 `deploy/docker-compose.yml` 统一编排，通过 **profile** 按需选择组件。

## Profile 概览

| Profile | 包含组件 | 用途 |
|---------|---------|------|
| `app` | GOSpeak 应用 | 必须。Go 后端 + 前端静态 |
| `srs` | SRS + Nginx | SRS 自建方案 |
| `livekit` | LiveKit | LiveKit 自建方案 |
| `redis` | Redis | JWT 密钥轮换 + Token 黑名单 |
| `postgres` | PostgreSQL | 生产级数据库 |
| `minio` | MinIO 对象存储 | S3 兼容存储 |

## 渐进式方案

### 第一档 — SRS + SQLite 最小生产栈

适用：个人/小团队，单机部署。

```bash
cd deploy
cp .env.example .env
cp env/app.srs.env.example env/app.srs.env
# 编辑 app.srs.env：改 JWT_KEY 和 SRS_SECRET

docker compose --profile srs --profile app up -d --build
```

```bash
# 验证
curl -s http://localhost/ping        # 应用健康
curl -s http://localhost:1985/api/v1/versions  # SRS 运行
```

暴露端口：
| 端口 | 服务 | 说明 |
|------|------|------|
| `:80` | Nginx | 公网入口（API + SPA + WebSocket + WHIP）|
| `:8000/udp+tcp` | SRS | WebRTC 媒体直连 |

### 第二档 — LiveKit + Redis + App

适用：需要完整功能（踢人、禁言、Webhook）的场景。

```bash
cd deploy
cp env/app.livekit.env.example env/app.livekit.env
# 编辑 app.livekit.env：改 JWT_KEY
export GOSPEAK_ENV_FILE=./env/app.livekit.env

docker compose --profile livekit --profile redis --profile app up -d --build
```

暴露端口：
| 端口 | 服务 | 说明 |
|------|------|------|
| `:7880` | LiveKit | HTTP API |
| `:7881-7882` | LiveKit | WebRTC 媒体 |
| `:3478/udp` | LiveKit | TURN 中继 |

### 第三档 — PostgreSQL + Redis + SRS

适用：需要持久化数据库，多于 50 并发用户。

```bash
cd deploy
# 在 app.srs.env 中添加：
# DB_TYPE=PostgresSQL
# DB_HOST=postgres
# DB_PORT=5432
# DB_USER=gospeak
# DB_PASSWORD=gospeak
# REDIS_HOST=redis

docker compose --profile postgres --profile redis --profile srs --profile app up -d --build
```

### 第四档 — 全量栈

适用：生产高可用场景。

```bash
docker compose \
  --profile postgres \
  --profile redis \
  --profile srs \
  --profile minio \
  --profile app \
  up -d --build
```

## 环境文件结构

```
deploy/
├── .env                     # Compose 级变量（端口、candidate）
├── env/
│   ├── app.srs.env          # SRS 模式应用配置
│   └── app.livekit.env      # LiveKit 模式应用配置
```

### `deploy/.env` — Compose 级变量

```env
GOSPEAK_ENV_FILE=./env/app.srs.env   # 使用哪份应用配置
GOSPEAK_PORT=8998
HTTP_PORT=80
SRS_CANDIDATE=127.0.0.1              # 生产改为公网 IP
```

### `deploy/env/app.srs.env` — 应用配置示例

```env
DB_TYPE=SQLite
SFU_PROVIDER=srs
SRS_HOST=srs
SRS_API_PORT=1985
SRS_PUBLIC_HOST=http://localhost
SRS_WHIP_URL=/rtc/v1/whip/
SRS_SECRET=please-set-openssl-rand-hex-32
JWT_KEY=please-change-me
```

## 常用操作

```bash
# 启动
docker compose --profile srs --profile app up -d --build

# 查看日志
docker compose logs -f gospeak

# 停止
docker compose --profile srs --profile app down

# 停止并清除数据卷
docker compose --profile srs --profile app down -v

# 仅启动依赖服务（本地开发时用）
docker compose -f deploy/docker-compose.yml --profile srs up -d
pnpm dev:server   # 本地启动后端
pnpm dev:web      # 本地启动前端
```

## 构建与版本

```bash
# 构建特定版本
docker compose --profile srs --profile app build

# 更新到最新版本
git pull
docker compose --profile srs --profile app up -d --build
```
