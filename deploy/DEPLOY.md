# GOSpeak 统一部署清单

> 入口: `deploy/docker-compose.yml`  
> 镜像: 仓库根 `Dockerfile` (Go API + 前端 SPA)

## 1. 网络架构

```text
浏览器
  ├─ HTTP/WSS :80  ──► nginx
  │                      ├─ /api /ws /  ──► gospeak:8998
  │                      └─ /rtc/v1            ──► srs:1985   (SRS 模式)
  ├─ WebRTC UDP/TCP :8000 ──► srs 直连媒体

gospeak ──HTTP──► srs:1985 | livekit:7880 | cloudflare API
gospeak ──DB────► sqlite volume | postgres
gospeak ──State─► nats (embedded/external, 可选)
```

## 2. 发布前 Checklist

- [ ] 复制 `deploy/.env.example` → `deploy/.env`
- [ ] 复制 `deploy/env/app.srs.env.example` → `deploy/env/app.srs.env` (或 livekit)
- [ ] 改 `JWT_KEY` / `SRS_SECRET` / DB 密码 / MinIO 密码
- [ ] 公网设 `SRS_CANDIDATE=<公网IP>`
- [ ] 公网设 `SRS_PUBLIC_HOST=https://your.domain`
- [ ] 防火墙开放:
  - [ ] 80/443 (nginx)
  - [ ] 8998 (仅内网调试时可开)
  - [ ] 8000/udp+tcp (SRS 媒体)
  - [ ] 7881/7882 (LiveKit 媒体, 若用 LiveKit)
- [ ] HTTPS: 在 nginx 前加 TLS 终结或启用 443 server
- [ ] 备份策略: `/app/db` volume / postgres dump

## 3. 一键命令

### 3.1 SRS + SQLite 最小栈

```bash
cd deploy
cp .env.example .env
cp env/app.srs.env.example env/app.srs.env
# 编辑 env/app.srs.env: SRS_SECRET=$(openssl rand -hex 32)

docker compose --profile srs --profile app up -d --build
curl -s http://localhost/ping
curl -s http://localhost:1985/api/v1/versions
```

访问: `http://localhost`  
媒体 candidate: `.env` 里 `SRS_CANDIDATE`

### 3.2 LiveKit + App

```bash
cd deploy
cp env/app.livekit.env.example env/app.livekit.env
export GOSPEAK_ENV_FILE=./env/app.livekit.env
docker compose --profile livekit --profile app up -d --build
```

### 3.3 PostgreSQL + SRS

```bash
# env 里设:
# DB_TYPE=PostgresSQL DB_HOST=postgres DB_PORT=5432 DB_USER=gospeak DB_PASSWORD=...

docker compose --profile postgres --profile srs --profile app up -d --build
```

### 3.4 Agent + Worker 集群（统一入口 + 多 Worker）

```bash
# 需要一个 admin JWT 作为 worker→agent 控制面鉴权 token
export CLUSTER_AGENT_TOKEN=<admin-jwt>
# 必填：浏览器可达的 Worker HTTP(S) 地址；统一入口模式下可都填同一个公网地址
export CLUSTER_WORKER_ADVERTISE_URL=https://ws.example.com
export CLUSTER_WORKER1_ADVERTISE_URL=https://ws.example.com
export CLUSTER_WORKER2_ADVERTISE_URL=https://ws.example.com
# 可选：统一入口（Agent 会返回 https://<entry>/ws?worker=<node-id>）
export CLUSTER_ENTRY_URL=https://ws.example.com
docker compose -f deploy/docker-compose.yml --profile cluster up -d --build
```

- Compose 内置 `gospeak-worker`（默认节点 `worker-default`）、`gospeak-worker-1`、`gospeak-worker-2` 三个数据面节点；`nginx-cluster` 是统一入口。
- `CLUSTER_ENTRY_URL` 设置后，Agent 返回的 `workerUrl` 形如 `https://ws.example.com/ws?worker=worker-1`，nginx 按 `?worker=` 路由到对应 Worker，实现“域名 → Worker”的应用层粘性会话。未设置时保持旧行为：直接返回节点 `AdvertiseURL`，浏览器直连。
- `CLUSTER_WORKER_ADVERTISE_URL` / `CLUSTER_WORKER1_ADVERTISE_URL` / `CLUSTER_WORKER2_ADVERTISE_URL` 必须填浏览器可达的 HTTP(S) 地址，**不能**填容器服务名；统一入口模式下三者可填同一个入口地址。
- `CLUSTER_WORKER_NODE_ID` 自定义默认 Worker 节点 ID；`worker-1` / `worker-2` 固定由 `gospeak-worker-1` / `gospeak-worker-2` 服务使用。
- 新增 Worker 时需同步扩展 `deploy/nginx-cluster.conf` 的 `map $arg_worker` 与 `upstream`；不要用 round-robin 转发 `/ws`，否则同一域名房间会被拆到不同节点。
- 消息持久化与控制命令走 JetStream JobQueue（`{prefix}_jobs`）。新部署自动创建 `LimitsPolicy` 流并启用角色分流：`chat.*` 任务仅由 Agent 消费，`srs` / `livekit` / `sfu_cleanup` / `cluster.control` 由 Worker 消费；控制命令因此具备持久化与重试，Worker 离线期间不会丢失。
- 旧版 `WorkQueuePolicy` 流只允许单个 consumer，会保持全实例消费的兼容模式（Worker 仍可能参与聊天落库）。要启用严格角色隔离，请升级后重建 `{prefix}_jobs` 流。

### 3.5 可观测栈（Prometheus + Grafana + Alertmanager + Loki）

```bash
# 先保证 gospeak 应用容器已运行，再启用 observability profile
docker compose -f deploy/docker-compose.yml --profile observability up -d
```

- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000`（默认 `admin/admin`，预置 GOSpeak Overview Dashboard 与 Prometheus/Loki 数据源）
- Alertmanager: `http://localhost:9093`
- Loki: `http://localhost:3100`
- Promtail 自动采集所有 Docker 容器 stdout 日志并推送到 Loki。
- 应用暴露 `GET /metrics`；设置 `METRICS_TOKEN` 后需要 `Authorization: Bearer <token>`，生产环境建议同时用防火墙/反代限制访问。
- 告警规则见 `deploy/observability/prometheus/rules.yml`，通知接收器在 `deploy/observability/alertmanager/alertmanager.yml` 配置。

### 3.6 仅依赖 (本地 pnpm 开发)

```bash
# 旧 dev 依赖栈仍可用 example 文件; 推荐改用 profiles:
docker compose -f deploy/docker-compose.yml --profile srs --profile minio up -d
pnpm dev:server
pnpm dev:web
```

## 4. 端口表

| 端口 | 服务 | 说明 |
|------|------|------|
| 80 | nginx | 公网入口 |
| 8998 | gospeak | API + SPA + WebSocket |
| 1985 | srs | HTTP API / WHIP 源 |
| 8000/udp+tcp | srs | WebRTC 媒体 |
| 7880-7882 | livekit | LiveKit 控制/媒体 |
| 5432 | postgres | DB |
| 9000/9001 | minio | 对象存储 |
| 9090 | prometheus | 指标采集 |
| 9093 | alertmanager | 告警 |
| 3000 | grafana | 可视化 |
| 3100 | loki | 日志存储 |

## 5. 环境变量关键项

| 变量 | 作用 |
|------|------|
| `SFU_PROVIDER` | `srs` / `livekit` / `agora` / `cloudflare` |
| `SRS_HOST` | Go→SRS 管理 API, compose 内填 `srs` |
| `SRS_PUBLIC_HOST` | 浏览器侧 serverUrl 前缀, 如 `https://domain` |
| `SRS_SECRET` | stream/room token HMAC, 必填 |
| `SRS_CANDIDATE` | compose 级 ICE 公网/可达 IP |
| `STATIC_DIR` | SPA 目录, 镜像默认 `/app/static` |
| `CLUSTER_ENTRY_URL` | 集群统一入口，例如 `https://ws.example.com`；设置后按 `?worker=` 路由 |
| `METRICS_TOKEN` | 可选，/metrics 的 Bearer token；留空表示不鉴权 |

## 6. 验证

```bash
# 应用
curl -s http://localhost/ping
# SRS
curl -s http://localhost:1985/api/v1/versions
# 进房 token (需先登录拿 JWT, 或先注册)
# POST /api/v1/signal/token 应含 serverUrl/whipUrl/stream...
```

## 9. SRS 注意事项

> 公网/自托管 SRS 时优先读本节。开发细节见 [`docs/srs-selfhost-runbook.md`](../docs/srs-selfhost-runbook.md)。

### 9.1 信令与媒体必须拆开看

| 路径 | 走什么 | 能否 Nginx 反代 |
|------|--------|-----------------|
| API / WebSocket | `gospeak:8998` | ✅ |
| WHIP / WHEP HTTP 信令 | `srs:1985` 的 `/rtc/v1/*` | ✅ 推荐同源反代 |
| WebRTC 媒体 (DTLS/UDP|TCP) | `srs:8000` | ❌ UDP 不能靠 Nginx 代；需直连 SRS |

架构是 **客户端 ↔ SFU**，不是用户 P2P。没有 TURN 时，用户浏览器必须能直连 `SRS_CANDIDATE:8000`。

### 9.2 必配项

| 变量 | 谁用 | 说明 |
|------|------|------|
| `SFU_PROVIDER=srs` | Go | 启用 SRS provider |
| `SRS_SECRET` | Go + callback | **必填非空**。空则 token 退化为明文，中文房间/用户名会炸 `Authorization` latin1 |
| `SRS_HOST` / `SRS_API_PORT` | Go→SRS | compose 内用 `srs` / `1985`；仅管理 API（踢人/列客户端） |
| `SRS_PUBLIC_HOST` | 浏览器 | token 里的 `serverUrl` 前缀。生产填 `https://your.domain`（经 nginx），**不要**填容器内网 `http://srs:1985` |
| `SRS_WHIP_URL` | 浏览器 | 默认相对路径 `/rtc/v1/whip/`，配合同源反代 |
| `SRS_CANDIDATE` | SRS ICE | 浏览器可达的媒体 IP：本机 `127.0.0.1`，局域网填 LAN IP，公网填公网 IP |

### 9.3 Nginx 规则

- 必须有 `location /rtc/v1/` → `srs:1985`，且写在 SPA `location /` **之前**
- 只反代 WHIP/WHEP HTTP；**不要**用 nginx `stream` 转 8000/udp（会改源地址，ICE 易挂）
- 生产 HTTPS 时：页面与 `/rtc/v1` 同域同 scheme，避免 mixed content
- Go 管理 SRS 用内网 `SRS_HOST=srs`，浏览器用 `SRS_PUBLIC_HOST`，两套地址不要混

### 9.4 启动顺序与 callback

- SRS `http_hooks` 会回调 Go：`/api/v1/srs/callback`（on_publish / on_play 等）
- **backend 必须先起来**；hooks 失败时 SRS 常 fail-closed，publish 全 403
- compose 全栈用 `deploy/srs/srs.docker.conf`，callback 指向 `http://gospeak:8998/...`
- 本地 dev（backend 在宿主机）用 `host.docker.internal:8998`（见 `deploy/srs/srs.conf`）

### 9.5 无 TURN 时的网络要求

当前仓库**不下发 iceServers / 不附带 Coturn**。公网要稳，至少：

1. 防火墙放行 `8000/udp` 与 `8000/tcp`（TCP 作不稳定 UDP 环境回退）
2. `SRS_CANDIDATE` 必须是客户端真实可达地址（不是 docker 内网 IP）
3. 云厂商安全组 / 本机防火墙都要开
4. 若大量用户 ICE failed / 无声，优先查 candidate 与 8000 连通，而不是查 WHIP HTTP

### 9.6 常见故障

| 症状 | 优先排查 |
|------|----------|
| WHIP 404 且像前端 HTML | `/rtc/v1/` 没进反代，被 SPA 兜底吃掉 |
| WHIP 502/504 | nginx → srs:1985 不通 |
| ICE failed / 无声 | `SRS_CANDIDATE` 错，或 8000 未对公网开放 |
| on_publish 403 | backend 未起、`SRS_SECRET` 不一致、callback 地址不可达 |
| `non ISO-8859-1 code point` | `SRS_SECRET` 为空 |
| 浏览器连 `localhost:1985` 生产挂 | 应走同源 `/rtc/v1` + `SRS_PUBLIC_HOST` |

### 9.7 安全

- SRS 默认不强制校验 WHIP Bearer；**token 安全依赖网络边界 + 自建 callback 校验**
- `SRS_SECRET` 只放服务端 env，勿进前端
- 管理 API `:1985` 生产尽量不对公网裸奔；浏览器只暴露经 nginx 的 `/rtc/v1`

## 7. 升级

```bash
git pull
cd deploy
docker compose --profile srs --profile app up -d --build
```

## 8. 停止

```bash
docker compose -f deploy/docker-compose.yml --profile srs --profile app down
# 清数据 volume:
# docker compose -f deploy/docker-compose.yml --profile srs --profile app down -v
```

### NATS 信号事件总线

- **默认**：`NATS_URL` 为空时，gospeak 进程内嵌 nats-server（默认随机本机端口；可用 `NATS_EMBEDDED_PORT` 固定，如 `4222`），单二进制零依赖。
- **外部**：`NATS_URL` 非空时先探测可用性（`NATS_CONNECT_TIMEOUT`，默认 2s）。
  - 探测成功 → 连外部，不启内嵌（`eventbus_mode=external`）
  - 探测失败 → **启动失败并 panic**，不回退内嵌
- **阶段二（状态共享）**：房间 membership/stream 存储优先级 **`STATE_STORE=auto` → nats → none**。
  - `nats`：JetStream KV（`{prefix}_membership` / `{prefix}_stream`）；外部 NATS 需 `-js`。
  - `none`：仅本机内存。
  成员变更仍经 `state:room-changed` 内部事件通知对端从存储重算列表。 成员变更后发布内部事件 `state:room-changed`，对端从 KV 重算并**本机**推送 `room:updated` / `room:list:result`（不再跨实例直接 fanout 带人数快照）。
- **多副本**：所有实例必须配置**同一个**外部 `NATS_URL` 且探测成功；否则进程启动失败。
- **监控**：SSE health 含 `eventbus_mode`、`eventbus_connected`、`eventbus_fallback_from_external`。
- **启用外部 NATS 示例**：

```bash
NATS_URL=nats://nats:4222
docker compose --profile nats --profile app up -d
```
