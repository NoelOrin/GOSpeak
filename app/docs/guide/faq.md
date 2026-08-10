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

## Docker 镜像如何构建？

```bash
# 从仓库根目录构建一体镜像（Go 后端 + 前端 SPA）
docker build -t gospeak .

# 运行
docker run -d \
  --env-file deploy/env/app.srs.env \
  -p 8998:8998 \
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
