# 环境变量配置

所有配置通过环境变量注入，支持 `.env` 文件。使用 `app/server/.env.dev`（开发）或 `deploy/env/app.*.env`（Docker 部署）。

## 数据库

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DB_TYPE` | `SQLite` | 数据库类型：`SQLite` / `PostgresSQL` / `MYSQL` |
| `DB_HOST` | `localhost` | 数据库主机 |
| `DB_PORT` | `5432` | 数据库端口 |
| `DB_USER` | — | 数据库用户 |
| `DB_PASSWORD` | — | 数据库密码 |
| `DB_PATH` | `app.db` | SQLite 数据库文件路径 |
| `DB_DSN` | — | 自定义 DSN（优先级高于逐字段配置）|

## SFU 音视频引擎

### 通用

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SFU_PROVIDER` | `livekit` | SFU 类型：`livekit` / `srs` / `mediasoup` / `agora` / `daily` / `cloudflare` |

### LiveKit

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LIVEKIT_HOST` | — | LiveKit 服务器 URL（如 `ws://localhost:7880`）|
| `LIVEKIT_KEY` | — | LiveKit API Key |
| `LIVEKIT_SECRET` | — | LiveKit API Secret |

### SRS

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SRS_HOST` | `localhost` | SRS 管理 API 地址（Go→SRS 通信）|
| `SRS_API_PORT` | `1985` | SRS API 端口 |
| `SRS_WHIP_URL` | `/rtc/v1/whip/` | WHIP 端点路径 |
| `SRS_SECRET` | — | stream/room token HMAC 密钥（**必填非空**）|
| `SRS_PUBLIC_HOST` | — | 浏览器侧 serverUrl 前缀（如 `https://your.domain`）|

### MediaSoup

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MEDIASOUP_BRIDGE_URL` | `http://localhost:3012` | MediaSoup worker HTTP bridge 地址 |
| `MEDIASOUP_HOST` | `localhost:3012` | 客户端侧 MediaSoup host |

### Agora

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `AGORA_APP_ID` | — | Agora 应用 ID |
| `AGORA_APP_CERTIFICATE` | — | Agora 应用证书 |
| `AGORA_HOST` | — | Agora 自定义主机 |
| `AGORA_CUSTOMER_ID` | — | Agora 客户 ID |
| `AGORA_CUSTOMER_SECRET` | — | Agora 客户密钥 |

### Daily

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DAILY_API_KEY` | — | Daily API 密钥 |
| `DAILY_DOMAIN` | — | Daily 域名 |

## Redis（可选）

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `REDIS_HOST` | — | Redis 主机（留空则不连接）|
| `REDIS_PORT` | `6379` | Redis 端口 |
| `REDIS_PASSWORD` | — | Redis 密码 |
| `REDIS_DB` | `0` | Redis DB 编号 |

Redis 未连接时，黑名单操作和 JWT 密钥轮换会优雅降级为无操作 / 静态密钥。

## JWT 认证

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `JWT_KEY` | `default-secret` | JWT 签名密钥（生产环境**必须修改**）|
| `JWT_KEY_TTL` | `24h` | JWT 密钥轮换周期（需要 Redis）|

## 邮箱验证（可选）

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `EMAIL_ENABLED` | `false` | 是否启用邮箱验证 |
| `SMTP_HOST` | — | SMTP 服务器 |
| `SMTP_PORT` | `587` | SMTP 端口 |
| `SMTP_USERNAME` | — | SMTP 用户名 |
| `SMTP_PASSWORD` | — | SMTP 密码 |
| `SMTP_FROM` | — | 发件人地址 |
| `SMTP_FROM_NAME` | `GoSpeak` | 发件人名称 |
| `EMAIL_CODE_TTL` | `10m` | 验证码有效期 |
| `EMAIL_SEND_COOLDOWN` | `60s` | 同邮箱发送冷却 |
| `EMAIL_CODE_SECRET` | — | 验证码签名密钥（启用邮箱时必填）|

## 对象存储

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `STORAGE_TYPE` | `local` | 存储类型：`local` / `s3` |
| `STORAGE_ENDPOINT` | — | S3 兼容端点（MinIO / R2）|
| `STORAGE_BUCKET` | — | S3 Bucket |
| `STORAGE_REGION` | — | S3 Region |
| `STORAGE_ACCESS_KEY` | — | S3 访问密钥 |
| `STORAGE_SECRET_KEY` | — | S3 秘密密钥 |
| `STORAGE_PUBLIC_BASE_URL` | — | 公开访问基础 URL（CDN / 自定义域名）|
| `STORAGE_PATH_PREFIX` | `uploads/` | 上传路径前缀 |

## 服务器

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_PORT` | `8098` | HTTP 服务端口 |
| `STATIC_DIR` | — | 前端静态文件目录 |
| `GIN_MODE` | `debug` | Gin 模式，生产设为 `release` |

## 数据库三档配置示例

### A 档 — SQLite（开箱即用）

```env
DB_TYPE="SQLite"
# 无需任何外部服务
```

### B 档 — PostgreSQL

```env
DB_TYPE="PostgresSQL"
DB_HOST="postgres"
DB_PORT="5432"
DB_USER="gospeak"
DB_PASSWORD="gospeak"
# 可选移除 SQLite DB_PATH
```

### C 档 — PostgreSQL + Redis

```env
DB_TYPE="PostgresSQL"
DB_HOST="postgres"
DB_PORT="5432"
DB_USER="gospeak"
DB_PASSWORD="gospeak"
REDIS_HOST="redis"
REDIS_PORT="6379"
JWT_KEY_TTL="24h"
```

## Docker Compose 环境文件

生产部署推荐使用 `deploy/` 目录下的预配环境文件：

| 文件 | 说明 |
|------|------|
| `deploy/env/app.srs.env` | SRS 模式（含 SRS 特有配置）|
| `deploy/env/app.livekit.env` | LiveKit 模式（含 Redis 配置）|
| `deploy/.env` | Compose 级变量（端口、candidate 等）|
