# GOSpeak 统一部署清单

> 入口: `deploy/docker-compose.yml`  
> 镜像: 仓库根 `Dockerfile` (Go API + 前端 SPA)

## 1. 网络架构

```text
浏览器
  ├─ HTTP/WSS :80  ──► nginx
  │                      ├─ /api /socket.io /  ──► gospeak:8998
  │                      └─ /rtc/v1            ──► srs:1985   (SRS 模式)
  ├─ WebRTC UDP/TCP :8000 ──► srs 直连媒体

gospeak ──HTTP──► srs:1985 | livekit:7880 | mediasoup:3012
gospeak ──DB────► sqlite volume | postgres
gospeak ──Redis─► redis (可选)
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

### 3.2 LiveKit + Redis + App

```bash
cd deploy
cp env/app.livekit.env.example env/app.livekit.env
export GOSPEAK_ENV_FILE=./env/app.livekit.env
docker compose --profile livekit --profile redis --profile app up -d --build
```

### 3.3 PostgreSQL + Redis + SRS

```bash
# env 里设:
# DB_TYPE=PostgresSQL DB_HOST=postgres DB_PORT=5432 DB_USER=gospeak DB_PASSWORD=...
# REDIS_HOST=redis

docker compose --profile postgres --profile redis --profile srs --profile app up -d --build
```

### 3.4 仅依赖 (本地 pnpm 开发)

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
| 8998 | gospeak | API + SPA + Socket.IO |
| 1985 | srs | HTTP API / WHIP 源 |
| 8000/udp+tcp | srs | WebRTC 媒体 |
| 7880-7882 | livekit | LiveKit 控制/媒体 |
| 3012 | mediasoup | bridge HTTP |
| 5432 | postgres | DB |
| 6379 | redis | 缓存/JWT |
| 9000/9001 | minio | 对象存储 |

## 5. 环境变量关键项

| 变量 | 作用 |
|------|------|
| `SFU_PROVIDER` | `srs` / `livekit` / `mediasoup` ... |
| `SRS_HOST` | Go→SRS 管理 API, compose 内填 `srs` |
| `SRS_PUBLIC_HOST` | 浏览器侧 serverUrl 前缀, 如 `https://domain` |
| `SRS_SECRET` | stream/room token HMAC, 必填 |
| `SRS_CANDIDATE` | compose 级 ICE 公网/可达 IP |
| `STATIC_DIR` | SPA 目录, 镜像默认 `/app/static` |

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
| API / Socket.IO | `gospeak:8998` | ✅ |
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

- **默认**：`NATS_URL` 为空时，gospeak 进程内嵌 nats-server（随机本机端口），单二进制零依赖。
- **外部优先**：`NATS_URL` 非空时先探测可用性（`NATS_CONNECT_TIMEOUT`，默认 2s）。
  - 探测成功 → 连外部，不启内嵌（`eventbus_mode=external`）
  - 探测失败 → 打 Warn，回退内嵌（`eventbus_mode=embedded`，`eventbus_fallback_from_external=true`），进程不退出
- **多副本**：所有实例必须实际连上**同一个**外部 NATS。若探测失败回退内嵌，则跨实例 fanout 失效（各嵌各的）。
- **监控**：SSE health 含 `eventbus_mode`、`eventbus_connected`、`eventbus_fallback_from_external`。
- **启用外部 NATS 示例**：

```bash
NATS_URL=nats://nats:4222
docker compose --profile nats --profile app up -d
```

