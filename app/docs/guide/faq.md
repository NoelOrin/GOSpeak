# 常见问题

## 需要什么级别服务器？

最低 1 核 1G，推荐 2 核 4G。实际需求取决于并发语音房间数和 SFU 类型。

## 用户数有限制吗？

无内置限制。受限于 SFU 后端与服务器带宽。

- SQLite：适合 50 人以内同时在线
- PostgreSQL：适合 200+ 用户

## 如何选择 SFU？

| 场景 | 推荐 SFU |
|------|----------|
| 自建、完全控制 | **SRS** 或 **LiveKit**（成熟度最高）|
| 不想管服务器 | **Agora** 或 **Cloudflare**（云服务按量计费）|
| 本地开发快速起步 | **LiveKit** 或 **SRS**（docker 一键起）|

详见 [SFU 配置对比](/sfu/comparison)。

## SRS 自建：浏览器连不上麦克风？

常见原因排查：

1. **ICE Candidate 不对**：`SRS_CANDIDATE` 必须是客户端真实可达的 IP（不是 docker 内网）
2. **防火墙端口未开放**：需放行 8000/udp 和 8000/tcp
3. **WHIP 404**：Nginx `/rtc/v1/` 反代被 SPA 兜底规则吃掉，检查 location 顺序
4. **on_publish 403**：backend 没先启动，SRS callback 地址不可达

## LiveKit 需要 Redis 吗？

GOSpeak 应用已剥离 Redis。LiveKit 单节点不需要 Redis；多节点 LiveKit 集群如需状态共享，可自行按 LiveKit 文档配置 Redis，不再由 GOSpeak 部署清单管理。

## 如何切换 SFU 后端？

只改环境变量 `SFU_PROVIDER` 及其对应配置项即可，不需改代码：

```bash
# 从 SRS 切换到 LiveKit
export SFU_PROVIDER=livekit
export LIVEKIT_HOST=ws://livekit:7880
export LIVEKIT_KEY=...
export LIVEKIT_SECRET=...
```

重启后端生效。运行时切换请通过 `/api/v1/sfu/update-config` API。

## 数据库从 SQLite 迁移到 PostgreSQL 怎么做？

1. 安装 PostgreSQL，创建数据库和用户
2. 修改环境变量：

```env
DB_TYPE="PostgresSQL"
DB_HOST="your-pg-host"
DB_PORT="5432"
DB_USER="gospeak"
DB_PASSWORD="gospeak"
```

3. 重启后端 — GORM 自动建表，旧数据需手动迁移

## 需要 HTTPS 吗？

生产环境建议加 HTTPS。WebRTC 要求安全上下文（HTTPS 或 localhost）才能使用麦克风。可通过 Nginx/Caddy 终结 TLS，或在 docker-compose 前加反向代理。

## TURN 服务器需要配置吗？

取决于你使用的 SFU 和部署环境：

- **LiveKit 自建**：内置 TURN 服务器（docker-compose 中 `turn.enabled=true`，端口 3478/udp），无需额外部署
- **LiveKit Cloud**：自带 TURN 中继，零配置
- **SRS 自建**：不内置 TURN。大多数非对称 NAT 场景下 SRS 的 8000/udp + TCP 回退已足够；对称 NAT 环境建议自行部署 Coturn
- **Agora / Cloudflare**：云服务自带 ICE 穿透能力

对于大多数非对称 NAT 场景，无需额外 TURN 服务器。

## 如何获取并运行 Docker 镜像？

GOSpeak 的官方发布镜像托管在 GitHub 容器镜像仓库（`ghcr.io`），每个 Release 自动构建并推送 `linux/amd64` 与 `linux/arm64` 双架构镜像。

> 完整部署说明见 [单容器 Docker 部署](/deployment/docker)。

### 镜像源

镜像地址为 `ghcr.io/noelorin/gospeak`，tag 由 Release 版本号推导：

| tag | 含义 | 示例 |
|-----|------|------|
| `x.y.z` | 精确版本 | `ghcr.io/noelorin/gospeak:1.2.3` |
| `x.y` | 次版本（自动跟随小版本更新）| `ghcr.io/noelorin/gospeak:1.2` |
| `x` | 主版本（自动跟随全部更新）| `ghcr.io/noelorin/gospeak:1` |

拉取镜像：

```bash
docker pull ghcr.io/noelorin/gospeak:1
```

> 若网络无法访问 `ghcr.io`，可改用下方的「本地构建」方式，或在 Docker 守护进程配置镜像加速（见下）后再拉取。

### 本地构建（备用镜像源）

需要自定义镜像或无法访问 `ghcr.io` 时，从仓库根目录自行构建一体镜像（Go 后端 + 前端 SPA）：

```bash
docker build -t gospeak .
```

### 镜像加速（registry mirror）

部分网络环境下 `docker pull` 官方源速度慢或不稳定，可在 Docker 守护进程配置镜像加速器作为拉取代理。编辑 `/etc/docker/daemon.json`（不存在则新建）：

```json
{
  "registry-mirrors": [
    "https://<你的镜像加速地址>"
  ]
}
```

重启守护进程后生效：

```bash
sudo systemctl daemon-reload
sudo systemctl restart docker
```

> 镜像加速器只代理 `docker pull` 的镜像拉取、不改变镜像内容；请仅使用你信任的加速源。

### docker run 单容器运行

使用官方镜像启动一个最小可运行实例（SQLite + 内存 NATS），数据通过命名卷持久化：

```bash
docker run -d \
  --name gospeak \
  --restart unless-stopped \
  -p 8998:8998 \
  --env-file deploy/env/app.srs.env \
  -v gospeak-db:/app/db \
  -v gospeak-uploads:/app/uploads \
  ghcr.io/noelorin/gospeak:1
```

- 首次访问：`http://<host>:8998/`；`/ping` 为健康检查端点（容器内已内置 HEALTHCHECK）。
- 容器内数据目录：`/app/db`（SQLite）、`/app/uploads`（对象存储），建议用卷持久化，避免容器重建丢数据。
- 完整配置项见 [环境变量参考](/guide/configuration)；生产部署建议改用 [Docker Compose](/deployment/docker-compose) 一键编排。

若改用本地构建的 `gospeak` 镜像，把最后一行镜像名替换为 `gospeak` 即可：

```bash
docker run -d \
  --name gospeak \
  --restart unless-stopped \
  -p 8998:8998 \
  --env-file deploy/env/app.srs.env \
  -v gospeak-db:/app/db \
  -v gospeak-uploads:/app/uploads \
  gospeak
```

## WebSocket 连接不上？

1. 检查 Nginx 是否反代了 `/ws` 路径
2. 确认后端 `SERVER_PORT` 与 Nginx upstream 一致
3. WebSocket WebSocket 需要 `Upgrade` 和 `Connection` 头
4. Nginx 配置参考 `deploy/nginx-docker.conf`

## 如何备份数据？

- **SQLite**：备份 `db/app.db` 文件
- **PostgreSQL**：`pg_dump -U gospeak gospeak > backup.sql`
- 推荐定时任务 + 异地存储

## 日志在哪里？

- **Docker 部署**：`docker compose logs gospeak`
- **本地开发**：控制台 stdout
- **持久化日志**（Docker）：挂载 `gospeak-logs` volume 到 `/app/logs`
