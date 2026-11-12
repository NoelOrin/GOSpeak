# 部署到服务器

本页说明如何在 Linux 服务器上部署 GOSpeak。前提：一台可 SSH 登录的 Linux 机器，已安装 Docker。

## 部署要点

部署只涉及三个变量：

1. **是否有域名** —— 有域名则填 `GOSPEAK_DOMAIN`，内置 Caddy 自动 HTTPS
2. **是否有现成反向代理** —— 有则不启用 `edge` 档位，反代指向 `8998`
3. **防火墙** —— 放行 80/443 与媒体端口

**只需要一个域名、一个上游。** LiveKit 不需要第二个域名、单独证书或单独的反代规则——信令走应用内置的同源反代 `/livekit`。

## 部署前评估

**1. 用户规模**
- 10 人以内：单机 + SQLite
- 几十人：单机 + PostgreSQL，或将数据库独立部署
- 上百人：LiveKit 独立部署、对象存储、多实例

**2. 语音自建还是云服务？**
自建 LiveKit 成本低但需自行运维；LiveKit Cloud 免运维但按量计费。详见 [语音服务](/sfu/)。

**3. 域名与 HTTPS**
浏览器仅在 HTTPS（或 localhost）下允许使用麦克风。**无 HTTPS 时语音功能不可用。** 有域名时交由内置 Caddy 处理证书。

## 推荐路线：Docker Compose

一条命令启动整套服务，包括应用、LiveKit 与可选的内置 Caddy。

### 第一步：获取代码

```bash
git clone https://github.com/NoelOrin/GOSpeak.git
cd GOSpeak/deploy
```

### 第二步：生成配置

```bash
./init.sh --domain voice.example.com     # 无域名时去掉 --domain
```

`./init.sh --domain voice.example.com` 写入域名并将 `edge` 加入 `COMPOSE_PROFILES`（已有 `app,livekit` 时追加为 `app,livekit,edge`）。执行后直接运行 `docker compose up -d --build`，无需手动调整档位。

该脚本执行以下操作：

- 生成 `deploy/.env`
- 随机生成 JWT 密钥、LiveKit 密钥、存储加密密钥
- 归一 `livekit/livekit.yaml`，使密钥仅保留在 `.env` 一处
- 校验 `.env` / `livekit.yaml` / `Caddyfile` 一致性

密钥统一由 compose 从 `.env` 注入。

**不启用 `edge` 的场景**：已有反向代理（Nginx / Traefik / Dokploy / 宝塔），或纯局域网无域名时，**不传 `--domain`**（或手动将 `edge` 从 `COMPOSE_PROFILES` 中移除），反代指向宿主机 `8998`。

### 第三步：确认档位

上一步传了 `--domain`，`deploy/.env` 中应为：

```ini
COMPOSE_PROFILES=app,livekit,edge
```

未传 `--domain`（已有反代 / 局域网）则为 `app,livekit`，不含 `edge`。

### 第四步：启动

```bash
docker compose up -d --build
```

首次构建镜像需数分钟，之后重启很快。

### 第五步：验证

```bash
curl -s http://localhost:8998/ping      # 应用自身
curl -s https://voice.example.com/ping  # 经 Caddy（有域名时）
```

有返回表示服务已启动。浏览器打开 `https://域名`（或 `http://服务器IP`）应可见界面。

## 三种场景

### 场景一：有域名，使用内置 Caddy（推荐）

```bash
./init.sh --domain voice.example.com
docker compose up -d --build
```

Caddy 自动申请并续期证书、将 HTTP 重定向到 HTTPS、处理 WebSocket 升级。域名 A 记录指向本机公网 IP，防火墙放行 80/443。

### 场景二：已有反向代理

**不传 `--domain`**（或手动将 `edge` 从 `COMPOSE_PROFILES` 移除）：

```bash
./init.sh
```

此时 `.env` 中为：

```ini
COMPOSE_PROFILES=app,livekit
GOSPEAK_DOMAIN=
GOSPEAK_PORT=8998
```

将现有反代指向 `http://<宿主机>:8998` 这**一个上游**，`/api`、`/ws`、`/uploads`、`/livekit` 全部同源经过它。反代侧需保留 `Host` 与 `X-Forwarded-Proto` 头，并放行 WebSocket 升级。

**无需为 LiveKit 编写任何 location。** 详见 [反代与 HTTPS](/deploy/nginx)。

### 场景三：局域网 / 无域名测试

**不传 `--domain`**：

```bash
./init.sh
```

保持 `app,livekit`，浏览器访问 `http://<服务器IP>:8998`。

若使用内置 Caddy 统一到 80 端口，手动将 `edge` 加入档位（未传 `--domain` 时不会被自动追加）：

```ini
COMPOSE_PROFILES=app,livekit,edge
GOSPEAK_DOMAIN=
```

`GOSPEAK_DOMAIN` 留空时 Caddy 仅监听 `:80`，纯 HTTP，不申请证书。浏览器打开 `http://<服务器IP>`。

> 麦克风需要安全上下文：仅 localhost 或 HTTPS 下授权。`http://<局域网IP>` 可进入页面，但无法使用语音。局域网验证语音需通过 SSH 端口转发至 `http://localhost:8998`。

## 档位选择

`deploy/.env` 中的 `COMPOSE_PROFILES` 决定启动哪些组件。修改该行后，后续命令均使用裸 `docker compose`，无需每次附加 `--profile`。

| 场景 | 配置值 |
|------|--------|
| 局域网 / 已有反代 | `app,livekit` |
| 有域名，自动 HTTPS | `app,livekit,edge` |
| 使用 PostgreSQL | `app,livekit,edge,postgres` |
| 使用对象存储 | `app,livekit,edge,postgres,rustfs` |
| 使用监控面板 | `app,livekit,edge,postgres,rustfs,observability` |

从 SQLite 迁移至 PostgreSQL 时，除改档位外还需修改数据库配置：

```ini
DB_TYPE=PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=gospeak
DB_PASSWORD=<强密码>
POSTGRES_PASSWORD=<同一密码>
```

**注意**：切换数据库不会自动迁移数据，SQLite 中已有数据需手动导出导入。

## 端口

| 端口 | 协议 | 服务 | 是否开放公网 |
|------|------|------|-------------|
| 80 / 443 | TCP | Caddy 或自有反代 | 必须 |
| 443 | UDP | Caddy（HTTP/3） | 可选 |
| 7882 | UDP | LiveKit 媒体（主通道） | 必须 |
| 7882 | TCP | LiveKit 媒体（UDP 不通时兜底） | 必须 |
| 3478 | UDP | TURN 中继 | 建议开放 |
| 8998 | TCP | 应用 | 仅自有反代场景需可达，不直接暴露公网 |
| 5432 | TCP | PostgreSQL | 禁止开放 |

**无需开放 7880 / 7881。** 7880 仅绑定宿主机回环地址（`127.0.0.1:7880`）供本机 `pnpm dev` 调试直连，不对公网开放，浏览器经应用同源的 `/livekit` 访问。7881 已不再发布。

防火墙示例（Ubuntu）：

```bash
ufw allow ssh
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 443/udp
ufw allow 7882/tcp
ufw allow 7882/udp
ufw allow 3478/udp
```

**媒体端口未开放是语音不通的最常见原因。** 页面可打开、可登录、可进频道但听不到声音，通常由此导致。云服务器除系统防火墙外还有安全组，两层均需放行。

## 服务器配置

| 人数 | CPU | 内存 | 说明 |
|------|-----|------|------|
| 10 人内 | 1 核 | 1G | 可运行 |
| 几十人 | 2 核 | 4G | 推荐起点 |
| 上百人 | 4 核+ | 8G+ | LiveKit 建议独立部署 |

资源消耗主要来自音频转发。带宽随人数增长——每个说话者约需 40-60 kbps 上行，收听者按人数累加。

## 备份

**SQLite**：整个 `/app/db` 目录即数据库：

```bash
docker run --rm -v gospeak_gospeak-db:/data -v $(pwd):/backup alpine \
  tar czf /backup/db-$(date +%F).tar.gz -C /data .
```

**PostgreSQL**：

```bash
docker compose exec -T postgres pg_dump -U gospeak gospeak | gzip > backup-$(date +%F).sql.gz
```

**上传文件**：`gospeak-uploads` 卷或对象存储桶。

建议配置为定时任务。

## 升级

```bash
cd GOSpeak
git pull
cd deploy
docker compose up -d --build
```

数据库表结构自动迁移，无需手动执行 SQL。升级前先备份。

## 不使用 Docker

可运行单个二进制文件，前端已内嵌：

```bash
# 从 Release 下载对应平台的 gospeak-server-v1.0.0-linux-amd64
chmod +x gospeak-server-v1.0.0-linux-amd64
./gospeak-server-v1.0.0-linux-amd64 server -e prod
```

配合 systemd 实现开机自启与崩溃重启。此方式下 LiveKit 需单独运行。

详见 [单二进制部署](/deploy/binary)。

## 单容器部署

不使用 compose 时，可用 `docker run`：

```bash
docker run -d \
  --name gospeak \
  --restart unless-stopped \
  -p 8998:8998 \
  --env-file .env \
  -v gospeak-db:/app/db \
  -v gospeak-uploads:/app/uploads \
  ghcr.io/noelorin/gospeak:1
```

此方式容器内**不含 LiveKit**，`LIVEKIT_HOST` 须指向一个外部可用的 LiveKit 实例。

详见 [单容器部署](/deploy/docker)。

## 上公网前检查

- [ ] `JWT_KEY` 非默认值
- [ ] `LIVEKIT_KEY` / `LIVEKIT_SECRET` 非默认值
- [ ] `STORAGE_ENCRYPT_KEY` 已设置
- [ ] 管理员密码已修改（默认 `admin123`）
- [ ] 数据库密码已修改
- [ ] HTTPS 已配置（有域名时 `GOSPEAK_DOMAIN` 已填且档位含 `edge`）
- [ ] 已有反代时档位**不含** `edge`
- [ ] 媒体端口已在防火墙放行（7882 tcp/udp、3478/udp）
- [ ] 已配置定时备份
- [ ] `CORS_ORIGIN` 设为实际域名，非 `*`

## 下一步

- [Docker Compose 详解](/deploy/compose) — 各档位说明
- [反代与 HTTPS](/deploy/nginx) — 内置 Caddy 与已有反代
- [监控](/deploy/monitoring) — Prometheus + Grafana
- [故障排查](/deploy/troubleshooting) — 问题诊断
