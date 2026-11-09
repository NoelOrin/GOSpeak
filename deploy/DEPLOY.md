# GOSpeak 统一部署清单

> 入口: `deploy/docker-compose.yml`  
> 配置: `deploy/.env`（唯一配置文件，compose 插值与应用 env 共用）  
> 镜像: 仓库根 `Dockerfile` (Go API + 前端 SPA)

## 1. 快速开始

```bash
cd deploy
# 生成 .env（随机密钥）+ 归一 livekit.yaml + 校验两者一致
# 生产务必带上浏览器侧地址（必须是浏览器可达的公网地址）：
./init.sh --livekit-url wss://lk.example.com

docker compose up -d --build  # 档位由 .env 的 COMPOSE_PROFILES 决定，无需 --profile
curl -s http://localhost:8998/ping
```

`init.sh` 做三件事，保证 `.env` 与 `livekit.yaml` 不会脱节：

1. 以 `.env.example` 为模板生成 `.env`，随机填充 `JWT_KEY` / `STORAGE_ENCRYPT_KEY` /
   `POSTGRES_PASSWORD` / `LIVEKIT_KEY` / `LIVEKIT_SECRET`
2. 归一 `livekit/livekit.yaml`：剥离硬编码 `keys` 与 `webhook.api_key`，
   密钥统一由 compose 从 `.env` 注入
3. 校验一致性，不一致直接失败退出

常用命令：

| 命令 | 作用 |
|------|------|
| `./init.sh` | 生成配置（`.env` 已存在则拒绝） |
| `./init.sh --force` | 重新生成（保留已填的 `LIVEKIT_PUBLIC_URL`） |
| `./init.sh --check` | 只校验不写文件，CI 也用它 |
| `./init.sh --livekit-url wss://lk.example.com` | 同时设置浏览器侧地址 |

手动方式: `cp .env.example .env` 后至少修改 `JWT_KEY` / `LIVEKIT_KEY` / `LIVEKIT_SECRET`，
并确保 `livekit/livekit.yaml` 的 `keys` 为 `{}`（跑 `./init.sh --check` 可自检）。

## 2. 发布前 Checklist

- [ ] 运行 `./init.sh --livekit-url wss://lk.example.com`（或复制 `.env.example` → `.env`）
- [ ] 确认 `JWT_KEY` / `LIVEKIT_KEY` / `LIVEKIT_SECRET` / `STORAGE_ENCRYPT_KEY` 已是随机值（init.sh 自动生成）
- [ ] `./init.sh --check` 通过（确认 `.env` 与 `livekit.yaml` 一致、无硬编码密钥）
- [ ] `LIVEKIT_PUBLIC_URL` 已设为**浏览器可达**的地址（不能填容器名 `livekit`），见第 5 节
- [ ] 叠加 postgres 时确认 `DB_TYPE=PostgreSQL` 已打开且 `DB_PASSWORD` 非默认
- [ ] 防火墙开放:
  - [ ] 80/443 (若前置自建 Nginx / TLS 终结)
  - [ ] 8998 (应用 API / SPA / WebSocket)
  - [ ] 7880 (LiveKit 信令 WS)、7881 / 7882 (媒体)、3478/udp (TURN)
- [ ] HTTPS: 在反向代理前加 TLS 终结或启用 443 server
- [ ] 备份策略: `/app/db` volume / postgres dump

## 3. 档位组合

档位写在 `deploy/.env` 的 `COMPOSE_PROFILES`（改一次，之后命令都是裸 `docker compose ...`）：

| 档位 | COMPOSE_PROFILES | 说明 |
|------|------------------|------|
| LiveKit（默认）| `livekit,app` | LiveKit + 应用；密钥由 compose 从 `.env` 注入 |
| + PostgreSQL | 追加 `postgres` 并打开 DB 段的 PostgreSQL 配置 | |
| + 对象存储 | 追加 `rustfs` | |
| + 可观测 | 追加 `observability` | Prometheus/Grafana/Alertmanager/Loki |

临时换档（不改编译在 .env 里的值）可用进程环境覆盖：

```bash
# 本地 pnpm 开发，只起依赖不起应用:
COMPOSE_PROFILES=livekit,rustfs docker compose up -d
pnpm dev:server && pnpm dev:web
```

### 可观测栈

```bash
docker compose --profile observability up -d
```

- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000`（默认 `admin/admin`，预置 GOSpeak Overview Dashboard 与 Prometheus/Loki 数据源）
- Alertmanager: `http://localhost:9093`
- Loki: `http://localhost:3100`；Promtail 自动采集所有 Docker 容器 stdout 日志
- 应用暴露 `GET /metrics`；设置 `METRICS_TOKEN` 后需要 `Authorization: Bearer <token>`，生产环境建议同时用防火墙/反代限制访问
- 告警规则见 `deploy/observability/prometheus/rules.yml`，通知接收器在 `deploy/observability/alertmanager/alertmanager.yml` 配置

> ⚠️ MinIO → RustFS 迁移提示：v1.x 起对象存储由 MinIO 换为 RustFS，volume 由 `minio-data` 更名为 `rustfs-data`。升级后旧数据不会自动迁移：如需保留旧文件请先从 `minio-data` 导出拷贝到 `rustfs-data`；确认无误后执行 `docker volume rm gospeak_minio-data` 清理。生产环境必须设置 `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`（勿用默认 rustfsadmin/rustfsadmin），Web 控制台在 9001 端口。

## 4. 端口表

| 端口 | 服务 | 说明 |
|------|------|------|
| 80 | nginx | 公网入口 |
| 8998 | gospeak | API + SPA + WebSocket |
| 7880-7882 | livekit | LiveKit 控制/媒体 |
| 5432 | postgres | DB |
| 9000/9001 | rustfs | 对象存储 |
| 9090 | prometheus | 指标采集 |
| 9093 | alertmanager | 告警 |
| 3000 | grafana | 可视化 |
| 3100 | loki | 日志存储 |

## 5. 环境变量关键项

完整变量见 `deploy/.env.example`（分区注释）。关键项：

| 变量 | 作用 |
|------|------|
| `COMPOSE_PROFILES` | 档位选择，改一次永久生效（默认 `livekit,app`）|
| `SFU_PROVIDER` | 固定 `livekit` |
| `LIVEKIT_HOST` | 后端访问 LiveKit 的地址（admin API），容器内填 `ws://livekit:7880` |
| `LIVEKIT_PUBLIC_URL` | **下发给浏览器的**连接地址，必须浏览器可达，如 `wss://lk.example.com`；留空回退到 `LIVEKIT_HOST` |
| `LIVEKIT_KEY` / `LIVEKIT_SECRET` | 唯一来源；compose 注入给 LiveKit 容器，`livekit.yaml` 不再存密钥 |
| `METRICS_TOKEN` | 可选，/metrics 的 Bearer token；留空表示不鉴权 |

### LiveKit 地址与密钥

**两个地址必须分清**，这是容器部署最容易踩的坑：

- `LIVEKIT_HOST`：后端调用 LiveKit admin API（房间管理/踢人）用，走容器内网
  `ws://livekit:7880`。
- `LIVEKIT_PUBLIC_URL`：`/api/v1/signal/join` 返回给前端的 `serverUrl`，
  浏览器用它连 `livekit-client`。**必须是浏览器能解析的公网地址**，
  填容器名会导致浏览器连不上（浏览器无法解析 `livekit` 这个容器服务名）。

**密钥只有一份**：`deploy/.env` 的 `LIVEKIT_KEY` / `LIVEKIT_SECRET`。
compose 以 `LIVEKIT_KEYS` / `LIVEKIT_WEBHOOK_API_KEY` 环境变量注入 LiveKit 容器，
`livekit/livekit.yaml` 内 `keys: {}` 不再硬编码任何凭据，因此不存在
「.env 与 yaml 两侧 key 不一致」的问题。

LiveKit 1.13 起 `webhook.api_key` 必须显式指定且必须存在于 keys 中，
否则容器启动即失败（`api_key is required to use webhooks`）。compose 已自动注入。

## 6. 验证

```bash
# 应用健康检查
curl -s http://localhost:8998/ping
# 进房 token (需先登录拿 JWT, 或先注册)
# POST /api/v1/signal/token 应返回 LiveKit 进房所需信息
```

## 7. 升级

```bash
git pull                       # 或 GOSPEAK_IMAGE 指向新版本 tag 后 docker compose pull
cd deploy
docker compose up -d --build
```

## 8. 停止

```bash
docker compose down
# 清数据 volume:
# docker compose down -v
```

## NATS 信号事件总线（内嵌）

- gospeak 进程固定使用**内嵌 nats-server**（默认随机本机端口；可用 `NATS_EMBEDDED_PORT` 固定，如 `4222`），单二进制零依赖，不再支持外部 `NATS_URL`。
- 房间 membership/stream 存储走 NATS JetStream KV，内嵌 NATS 随进程启动，NATS 不可用时启动失败（fail-fast）。
- **监控**：SSE health 含 `eventbus_mode`（固定 `embedded`）、`eventbus_connected`。
