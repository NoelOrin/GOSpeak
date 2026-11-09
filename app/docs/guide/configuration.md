# 配置项

GOSpeak 全部靠环境变量配置，没有独立的配置文件。改完重启进程生效。

只有一份配置文件：**`deploy/.env`**（由 `deploy/.env.example` 复制或 `deploy/init.sh` 生成）。

| 场景 | 怎么读到 |
|------|---------|
| Docker Compose | compose 变量替换 + 应用容器 `env_file` 注入 |
| 本地开发 `pnpm dev:server` | 启动时自动向上查找仓库根的 `deploy/.env` |
| 直接跑二进制 `server -e prod` | 同上；也可用 `GOSPEAK_ENV_FILE=/path/to/env` 显式指定 |

不需要再维护 `app/server/.env`、`.env.dev`、`.env.prod`——这些已废弃。

## 必填项

这些不填就跑不起来，或者跑起来但功能残废：

| 变量 | 说明 |
|------|------|
| `JWT_KEY` | 签发登录令牌的密钥。生产必须换成随机值：`openssl rand -hex 32` |
| `LIVEKIT_HOST` | 后端访问 LiveKit 的地址（admin API）。容器部署 `ws://livekit:7880`，云服务 `wss://xxx.livekit.cloud` |
| `LIVEKIT_PUBLIC_URL` | 下发给浏览器的连接地址，必须浏览器可达（如 `wss://lk.example.com`）。留空回退到 `LIVEKIT_HOST` |
| `LIVEKIT_KEY` | LiveKit 的 API Key |
| `LIVEKIT_SECRET` | LiveKit 的 API Secret，必须和 LiveKit 那边一致 |
| `STORAGE_ENCRYPT_KEY` | 生产环境必填，64 位十六进制。用来加密存储配置里的敏感字段 |

## 数据库

| 变量 | 默认 | 说明 |
|------|------|------|
| `DB_TYPE` | `SQLite` | `SQLite` / `PostgreSQL` / `MySQL` |
| `DB_PATH` | `db/app.db` | SQLite 文件位置，其他类型忽略 |
| `DB_WAL` | `false` | SQLite 开 WAL 模式，并发读更好，建议开 |
| `DB_HOST` | — | 非 SQLite 必填 |
| `DB_PORT` | — | PostgreSQL 填 5432，MySQL 填 3306 |
| `DB_USER` | — | 数据库用户 |
| `DB_PASSWORD` | — | 数据库密码 |
| `DB_DSN` | — | 想自己写完整连接串就用这个，会覆盖上面几项 |

表结构由 GORM 自动迁移，启动时自动建表、加字段，不用手动跑 SQL。

## 语音

| 变量 | 默认 | 说明 |
|------|------|------|
| `SFU_PROVIDER` | `livekit` | 目前只支持这个值 |
| `LIVEKIT_HOST` | — | 见上 |
| `LIVEKIT_PUBLIC_URL` | — | 见上 |
| `LIVEKIT_KEY` | — | 见上 |
| `LIVEKIT_SECRET` | — | 见上 |

也可以在管理后台的「SFU 管理」里改，改完存数据库，会覆盖环境变量。适合不想重启的场景。

## 存储

上传的头像、附件、表情图存哪儿。

| 变量 | 默认 | 说明 |
|------|------|------|
| `STORAGE_TYPE` | `local` | `local` 存本机磁盘，`s3` 存对象存储 |
| `STORAGE_LOCAL_DIR` | `uploads` | `local` 模式的落盘目录 |
| `STORAGE_ENDPOINT` | — | S3 端点，如 RustFS、R2、MinIO |
| `STORAGE_BUCKET` | — | 桶名 |
| `STORAGE_REGION` | — | 区域，部分服务商需要 |
| `STORAGE_ACCESS_KEY` | — | Access Key |
| `STORAGE_SECRET_KEY` | — | Secret Key，会加密存储 |
| `STORAGE_PATH_PREFIX` | `uploads/` | 对象键前缀 |
| `STORAGE_PUBLIC_BASE_URL` | — | 走 CDN 时填 CDN 地址 |
| `STORAGE_ENCRYPT_KEY` | — | 加密上面那些密钥用，生产必填 |

## 邮件

用于注册验证码、找回密码。不开也能用，只是这些功能不可用。

| 变量 | 默认 | 说明 |
|------|------|------|
| `EMAIL_ENABLED` | `false` | 总开关 |
| `SMTP_HOST` | — | SMTP 服务器 |
| `SMTP_PORT` | `587` | 端口 |
| `SMTP_USERNAME` | — | 账号 |
| `SMTP_PASSWORD` | — | 密码 |
| `SMTP_FROM` | — | 发件地址 |
| `SMTP_FROM_NAME` | `GoSpeak` | 发件人显示名 |
| `EMAIL_CODE_TTL` | `10m` | 验证码有效期 |
| `EMAIL_SEND_COOLDOWN` | `60s` | 同一邮箱两次发送的间隔 |
| `EMAIL_CODE_SECRET` | — | 验证码签名密钥 |

## 服务

| 变量 | 默认 | 说明 |
|------|------|------|
| `SERVER_PORT` | `8998` | 后端监听端口 |
| `APP_ENV` | — | `dev` / `prod`，影响日志与安全默认值 |
| `CORS_ORIGIN` | `*` | 允许的跨域来源，生产建议填前端域名 |
| `STATIC_DIR` | — | 用外部目录的前端文件覆盖二进制内嵌的那份 |

## 日志

| 变量 | 默认 | 说明 |
|------|------|------|
| `LOG_LEVEL` | — | `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | — | `json` 便于日志系统采集，`text` 便于人看 |
| `LOG_OUTPUT` | `stdout` | `stdout` 或 `file` |
| `LOG_FILE` | `logs/app.log` | `LOG_OUTPUT=file` 时的路径 |
| `LOG_CALLER` | `false` | 日志里带文件名和行号 |

## 事件总线

内部事件、JWT 黑名单、密钥轮换、异步任务都跑在进程内的 NATS 上。**不需要也不能配外部 NATS。**

| 变量 | 默认 | 说明 |
|------|------|------|
| `NATS_SUBJECT_PREFIX` | `gospeak` | 主题与 KV 桶的前缀，多实例共用一个 NATS 时才需要区分 |
| `NATS_CONNECT_TIMEOUT` | `2s` | 连接超时 |

## 令牌

| 变量 | 默认 | 说明 |
|------|------|------|
| `JWT_KEY_TTL` | `24h` | 签名密钥轮换周期 |

## 常见配置组合

### 本地开发，用本机 LiveKit

```ini
APP_ENV="dev"
DB_TYPE="SQLite"
DB_WAL="true"
JWT_KEY="dev-secret"
SFU_PROVIDER="livekit"
LIVEKIT_HOST="ws://localhost:7880"
LIVEKIT_KEY="devkey"
LIVEKIT_SECRET="devsecret"
STORAGE_TYPE="local"
```

### 小团队自建，单机

```ini
APP_ENV="prod"
DB_TYPE="SQLite"
DB_WAL="true"
JWT_KEY="<openssl rand -hex 32>"
SFU_PROVIDER="livekit"
LIVEKIT_HOST="ws://livekit:7880"          # 后端访问，走容器内网
LIVEKIT_PUBLIC_URL="wss://lk.example.com"  # 下发给浏览器，必须公网可达
LIVEKIT_KEY="<随机>"
LIVEKIT_SECRET="<随机>"
STORAGE_TYPE="local"
STORAGE_ENCRYPT_KEY="<64 位 hex>"
CORS_ORIGIN="https://voice.example.com"
```

### 用 LiveKit 云服务

```ini
SFU_PROVIDER="livekit"
# 云服务场景下后端与浏览器访问同一地址，PUBLIC_URL 留空即回退
LIVEKIT_HOST="wss://your-project.livekit.cloud"
LIVEKIT_KEY="APIxxxxxxxx"
LIVEKIT_SECRET="xxxxxxxx"
```

本地不用跑 Docker，只起应用就行。

## 改完不生效？

- Docker 部署：`docker compose up -d` 会重建容器读新环境变量，只 `restart` 不行
- 直接跑二进制：必须重启进程
- 管理后台改的 SFU / 存储配置存在数据库里，优先级高于环境变量，想恢复环境变量配置得去后台清掉

## 下一步

- [域与频道](/guide/domains) — 建域、建频道、访客开关
- [权限](/guide/permissions) — 谁能做什么
- [文件存储](/guide/storage) — 头像和附件存哪
- [部署到服务器](/deploy/) — 生产环境的配置建议
