# GOSpeak 部署/运维手册

> 版本: 1.1 · 更新: 2026-07-11
>
> **统一部署入口已迁到 [`deploy/DEPLOY.md`](../deploy/DEPLOY.md) + [`deploy/docker-compose.yml`](../deploy/docker-compose.yml)。**
> 本文保留概念说明；一键命令与公网清单以 `deploy/DEPLOY.md` 为准。

GOSpeak 实时音视频平台的部署文档。覆盖本地开发、Docker 部署、生产上线全流程。

---

## 1. 环境要求

| 组件 | 版本 | 用途 |
|------|------|------|
| Go | 1.26+ | 后端编译 (需要 CGO, 因 SQLite) |
| Node.js | 22+ | 前端构建 (Vite) |
| pnpm | 10+ | monorepo 包管理 |
| Docker | 24+ | 容器化部署 |
| Docker Compose | v2 | 多服务编排 |

可选:
- PostgreSQL / MySQL — 生产数据库（默认 SQLite）

> NATS 事件总线固定为进程内嵌运行（含 JetStream），随 GOSpeak 启动，无需单独部署。

---

## 2. 快速开始（本地开发）

### 2.1 安装依赖

```bash
pnpm install
```

### 2.2 启动依赖服务

```bash
docker compose -f deploy/docker-compose.yml up -d
```

默认启动: LiveKit + 应用 + Nginx（`COMPOSE_PROFILES=livekit,app`，见 `deploy/.env`，由 `deploy/init.sh` 生成）；RustFS 等按 profile 追加启用。

### 2.3 启动后端

```bash
pnpm dev:server
# 等价: cd app/server && air
# air 会热重载，环境由 -e dev 指定（自动加载仓库根的 deploy/.env）
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

访问: http://localhost:3000 (Vite)

### 2.5 默认管理员账号

| 字段 | 值 |
|------|-----|
| 用户名 | `admin` |
| 默认密码 | `admin123` |

首次启动自动 seed；登录后若 `need_change_password=true`，请立即修改默认密码。


---

## 3. 环境变量说明

配置完全通过环境变量驱动，唯一配置文件是仓库根的 **`deploy/.env`**（由 `deploy/.env.example` 复制或 `deploy/init.sh` 生成）。

启动时 `config.LoadEnvFiles()` 按顺序查找：`GOSPEAK_ENV_FILE` 显式指定的文件 → 当前工作目录的 `.env.<env>` / `.env` → 逐级向上找到的 `deploy/.env`。已存在的进程环境变量不会被覆盖，因此容器内以 `env_file` 注入的值优先。

### 3.1 数据库

| 变量 | 说明 | 示例 |
|------|------|------|
| `DB_TYPE` | `PostgreSQL` / `MySQL` / `SQLite` | `SQLite` |
| `DB_HOST` | DB 主机 (非 SQLite) | `localhost` |
| `DB_PORT` | DB 端口 | `5432` |
| `DB_USER` | DB 用户 | `gospeak` |
| `DB_PASSWORD` | DB 密码 | `***` |
| `DB_PATH` | SQLite 文件路径 | `db/gospeak.db` |

### 3.2 NATS（内嵌事件总线）

NATS 总线固定为进程内嵌运行（含 JetStream），承担内部事件、KV 状态（JWT 黑名单/密钥轮换/禁言规则）与异步任务队列，无需也无法配置外部 NATS（不支持 `NATS_URL`）。

| 变量 | 说明 | 示例 |
|------|------|------|
| `NATS_SUBJECT_PREFIX` | NATS subject / KV bucket 前缀 | `gospeak` |
| `NATS_NAME` | 实例名（空 = 自动生成） | `node-1` |
| `NATS_CONNECT_TIMEOUT` | 连接超时 | `2s` |
| `NATS_EMBEDDED_PORT` | 内嵌 NATS 端口（空 = 随机端口） | `4222` |
| `NATS_WAL_PATH` | 断线事件磁盘 WAL 路径（空 = 禁用） | `/data/nats-wal` |
| `JWT_KEY_TTL` | JWT 密钥轮换周期，经 NATS KV 管理 | `24h` |

### 3.3 SFU（LiveKit）

`SFU_PROVIDER` 固定为 `livekit`（唯一支持的音视频后端）。

| 变量 | 说明 |
|------|------|
| `SFU_PROVIDER` | `livekit`（固定值） |
| `LIVEKIT_HOST` | 后端访问 LiveKit 的地址（admin API）。容器部署 `ws://livekit:7880`，Cloud `wss://<id>.livekit.cloud` |
| `LIVEKIT_PUBLIC_URL` | 下发给浏览器的连接地址，必须浏览器可达。留空回退到 `LIVEKIT_HOST` |
| `LIVEKIT_KEY` | API Key |
| `LIVEKIT_SECRET` | API Secret |

### 3.4 对象存储

| 变量 | 说明 | 示例 |
|------|------|------|
| `STORAGE_TYPE` | `local` / `s3` | `local` |
| `STORAGE_ENDPOINT` | S3 端点 (RustFS/R2) | `http://localhost:9000` |
| `STORAGE_BUCKET` | 桶名 | `gospeak` |
| `STORAGE_REGION` | 区域 | `us-east-1` |
| `STORAGE_ACCESS_KEY` | 访问密钥 | `rustfsadmin` |
| `STORAGE_SECRET_KEY` | 密钥 | `rustfsadmin` |
| `STORAGE_PATH_PREFIX` | S3 对象键前缀（本地存储忽略） | `uploads/` |
| `STORAGE_LOCAL_DIR` | 本地存储落盘目录，与 `/uploads` 静态服务同源 | `uploads` |
| `STORAGE_PUBLIC_BASE_URL` | 公开域名 (CDN) | `https://cdn.example.com` |
| `STORAGE_ENCRYPT_KEY` | 凭证加密密钥，64 位 hex（生产必设） | `a1b2...` |

⚠️ `STORAGE_ENCRYPT_KEY` 生产环境必须设置，否则凭证明文存储。

---

## 4. Docker 部署

### 4.1 依赖服务 — 渐进式数据库三档

首次部署推荐 `cd deploy && ./init.sh livekit` 自动生成配置。`deploy/docker-compose.yml` 用 Compose `profiles` 提供档位组合，档位写在 `deploy/.env` 的 `COMPOSE_PROFILES`，之后命令都是裸 `docker compose up -d`：

| 档 | `deploy/.env` 关键项 |
|----|----------------------|
| A 单 SQLite | `DB_TYPE=SQLite`（默认） |
| B 单 PostgreSQL | `COMPOSE_PROFILES` 追加 `postgres`；`DB_TYPE=PostgreSQL`, `DB_HOST=postgres`, `DB_PASSWORD=...` |
| C PostgreSQL + 内嵌 NATS | 同 B 档；`JWT_KEY_TTL=24h` 启用密钥轮换 |

- **A 档**：零外部 DB，SQLite 内嵌，最小起步
- **B 档**：单一 PostgreSQL，中等规模
- **C 档**：PostgreSQL + 内嵌 NATS（JWT 轮换、Token 黑名单开箱即用），生产推荐。

NATS 为进程内嵌运行，无需外部部署，也不支持多副本共享外部 NATS。

### 4.1.1 SFU 配置

与 DB 档正交。`./init.sh livekit` 一步完成档位与密钥配置，或直接改 `deploy/.env`：`COMPOSE_PROFILES` 使用 `livekit` + `SFU_PROVIDER=livekit`。

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
  --env-file deploy/.env \
  -p 8998:8998 \
  gospeak:latest
```

应用读 `deploy/.env`，默认 `prod` 环境（无需 `-e` flag）。

---

## 5. 生产部署步骤

### 5.1 发布清单 Checklist

- [ ] `deploy/.env` 已配置真实密钥（LiveKit、DB、Storage）
- [ ] `STORAGE_ENCRYPT_KEY` 已设（64 位 hex）
- [ ] `LIVEKIT_PUBLIC_URL` 已设为浏览器可达地址（容器部署不能填 `ws://livekit:7880`）
- [ ] `DB_TYPE` 生产环境用 PostgreSQL/MySQL，非 SQLite
- [ ] `JWT_KEY` 已修改为强随机值（默认 `default-secret` 不可上生产）
- [ ] 端口防火墙开放: 8998 (应用) + SFU WebRTC UDP/TCP
- [ ] HTTPS 终结已配置（见 Nginx 反代）
- [ ] 数据库备份策略已就位
- [ ] 日志收集已配置

### 5.2 完整上线流程

```bash
# 1. 克隆 + 配置
git clone <repo> && cd GOSpeak
cp deploy/.env.example deploy/.env
# 编辑 deploy/.env 填入生产密钥（或用 deploy/init.sh 自动生成）

# 2. 拉取依赖镜像
docker compose -f deploy/docker-compose.yml up -d

# 3. 构建应用
docker build -t gospeak:latest .

# 4. 运行
docker run -d \
  --name gospeak \
  --env-file deploy/.env \
  -p 8998:8998 \
  --restart unless-stopped \
  gospeak:latest

# 5. 验证
curl http://localhost:8998/api/v1/health  # (若该端点存在)
```

### 5.3 Nginx 反代

见 `deploy/nginx.conf`。WebSocket 升级用于 WebSocket 信令，`/api/` 代理到 Go 后端。

### 5.4 停止

```bash
docker stop gospeak
docker compose -f deploy/docker-compose.yml down
```

---

## 6. 运维

### 6.1 日志

```bash
docker logs -f gospeak
```

应用日志由 `logrus` 输出至 stdout（Text 格式，含时间戳）。

### 6.2 健康检查

应用容器需配 `HEALTHCHECK`（见 Dockerfile 加固项）。依赖服务在 `docker-compose.yml` 已配 healthcheck。

### 6.3 数据备份

- **SQLite:** 备份 `app/server/db/*.db` 文件
- **PostgreSQL:** `pg_dump`
- **RustFS:** 桶同步至异地

### 6.4 升级

```bash
git pull
docker build -t gospeak:latest .
docker stop gospeak && docker rm gospeak
docker run -d ... gospeak:latest  # 同 5.2 step 4
```

> Compose 部署的升级更简单：`cd deploy && docker compose up -d --build`（见 `deploy/DEPLOY.md`）。

---

## 7. 故障排查

| 症状 | 排查 |
|------|------|
| 前端无音视频 | 检查 `SFU_PROVIDER` + LiveKit 服务状态 + WebRTC UDP 端口开放 |
| 信令连不上 | 检查 WebSocket WebSocket 升级，Nginx 配置 |
| JWT 无法刷新 | 检查内嵌 NATS 启动日志（NATS 不可用会 fail-fast）与 `JWT_KEY` 一致性 |
| 上传失败 | 检查 `STORAGE_TYPE` + RustFS/S3 凭证 + `STORAGE_ENCRYPT_KEY` |
| DB 连接失败 | `DB_TYPE` 不匹配，PostgreSQL/MySQL 需 `DB_HOST` 等填全 |

---

## 8. 参考

- 架构总览: `ARCHITECTURE.md`
- Agent 指南: `AGENTS.md`
- 一键编排方案: `deploy/docker-compose.yml`
- 服务成熟度: `docs/sfu-provider-maturity.md`
