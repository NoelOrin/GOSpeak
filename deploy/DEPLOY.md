# GOSpeak 统一部署清单

> 入口: `deploy/docker-compose.yml`  
> 配置: `deploy/.env`（唯一配置文件，compose 插值与应用 env 共用）  
> 镜像: 仓库根 `Dockerfile` (Go API + 前端 SPA)

## 1. 快速开始

```bash
cd deploy
./init.sh                     # 自动生成 .env（含随机密钥），默认 LiveKit 档位
docker compose up -d --build  # 档位由 .env 的 COMPOSE_PROFILES 决定，无需 --profile
curl -s http://localhost:8998/ping
```

手动方式: `cp .env.example .env` 后至少修改 `JWT_KEY` 与 `LIVEKIT_KEY` / `LIVEKIT_SECRET`（`openssl rand -hex 32`）。

## 2. 发布前 Checklist

- [ ] 运行 `./init.sh` 或复制 `.env.example` → `.env`
- [ ] 确认 `JWT_KEY` / `LIVEKIT_KEY` / `LIVEKIT_SECRET` / `STORAGE_ENCRYPT_KEY` 已是随机值（init.sh 自动生成）
- [ ] `deploy/.env` 的 `LIVEKIT_KEY` / `LIVEKIT_SECRET` 与 `livekit/livekit.yaml` 的 `keys` 保持一致（webhook 签名校验依赖此项）
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
| LiveKit（默认）| `livekit,app` | LiveKit + 应用；init.sh 会同步轮换 `livekit/livekit.yaml` 密钥 |
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
| `LIVEKIT_HOST` | Go→LiveKit 地址，compose 内填 `ws://livekit:7880` |
| `LIVEKIT_KEY` / `LIVEKIT_SECRET` | 与 `livekit/livekit.yaml` 的 `keys` 一致，webhook 签名与进房 token 依赖 |
| `METRICS_TOKEN` | 可选，/metrics 的 Bearer token；留空表示不鉴权 |

### LiveKit webhook 与密钥一致性

LiveKit 的房间/参与者事件通过 `livekit/livekit.yaml` 的 `webhook.urls` 回调到 Go 后端
`POST /api/v1/signal/webhook`（compose 内地址为 `http://gospeak:8998/api/v1/signal/webhook`）。
LiveKit 用 `keys` 中第一个 key 对请求签名，Go 侧用 `LIVEKIT_KEY` / `LIVEKIT_SECRET` 校验，
因此 `deploy/.env` 的 `LIVEKIT_KEY` / `LIVEKIT_SECRET` 必须与 `livekit/livekit.yaml` 的 `keys`
完全一致 —— `init.sh` 会同时轮换两侧，手动改 `.env` 时记得同步改 `livekit.yaml`，否则
webhook 校验失败、进房 token 也会异常。

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
