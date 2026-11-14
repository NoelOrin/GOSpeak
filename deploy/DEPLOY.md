# GOSpeak 部署指南

> 唯一权威部署入口。  
> 入口文件：`deploy/docker-compose.yml`  
> 配置文件：`deploy/.env`（唯一配置文件，compose 插值与应用环境变量共用）  
> 镜像：仓库根 `Dockerfile`（Go API + 前端 SPA）

核心结论先放在前面：

- **只需要一个域名。** 填 `GOSPEAK_DOMAIN`，内置 Caddy 自动申请证书、自动 HTTPS。
- **只需要一个上游。** 反代或证书层只面对 `gospeak:8998`，`/api`、`/ws`、`/uploads`、`/livekit` 全在这一个上游里。
- **LiveKit 不需要第二个域名、不需要单独证书、不需要单独反代规则。** 信令经应用内置的同源反代 `/livekit` 转发到内部 LiveKit 的 7880，WebSocket 升级自动处理；浏览器拿到的 `serverUrl` 由应用按请求 Host 自动推导为 `wss://<你的域名>/livekit`。

---

## 1. 快速开始

```bash
cd deploy
./init.sh --domain voice.example.com        # 生成 deploy/.env，写入域名并自动启用 edge
docker compose up -d --build                # 启动，档位由 .env 的 COMPOSE_PROFILES 决定
```

`./init.sh --domain voice.example.com` 会写入域名并自动把 `edge` 加进 `COMPOSE_PROFILES`（已有 `app,livekit` 时追加为 `app,livekit,edge`）。跑完直接 `docker compose up -d --build` 即可，无需手改档位。

`edge` 就是内置 Caddy。填了 `GOSPEAK_DOMAIN` 后它自动申请 Let's Encrypt 证书，把 80 重定向到 443，浏览器打开 `https://voice.example.com` 即可。

**不开 `edge` 的场景**：已有反代（Nginx / Traefik / Dokploy / 宝塔），或纯局域网无域名时，**不要传 `--domain`**（或手动把 `edge` 从 `COMPOSE_PROFILES` 里去掉），反代指向宿主机 `8998` 即可。

`init.sh` 做三件事，保证 `.env` 与 `livekit.yaml` 不会脱节：

1. 以 `.env.example` 为模板生成 `.env`，随机填充 `JWT_KEY` / `STORAGE_ENCRYPT_KEY` / `POSTGRES_PASSWORD` / `LIVEKIT_KEY` / `LIVEKIT_SECRET`
2. 归一 `livekit/livekit.yaml`：剥离硬编码 `keys` 与 `webhook.api_key`，密钥统一由 compose 从 `.env` 注入
3. 校验一致性（含 `deploy/Caddyfile` 存在性与注入表达式），不一致直接失败退出

常用命令：

| 命令 | 作用 |
|------|------|
| `./init.sh` | 生成配置（`.env` 已存在则拒绝） |
| `./init.sh --domain voice.example.com` | 生成配置，写入 `GOSPEAK_DOMAIN` 并自动把 `edge` 追加进 `COMPOSE_PROFILES` |
| `./init.sh --force` | 重新生成（保留已填的 `GOSPEAK_DOMAIN`，`edge` 不重复追加） |
| `./init.sh --check` | 只校验不写文件，CI 也用它；没有 `.env` 也能通过 |

手动方式：`cp .env.example .env` 后至少修改 `JWT_KEY` / `LIVEKIT_KEY` / `LIVEKIT_SECRET` / `STORAGE_ENCRYPT_KEY`，并确保 `livekit/livekit.yaml` 的 `keys` 为 `{}`（跑 `./init.sh --check` 可自检）。

---

## 1.1 简化版（推荐起步）

只需要应用 + LiveKit 时，用 `docker-compose.simple.yml`：无 profiles、无 PostgreSQL、
无 RustFS、无可观测性栈，数据库走 SQLite、存储走本地磁盘，开箱即用。

```bash
cd deploy
./init.sh --domain voice.example.com

# 在仓库根目录执行（注意 -f 路径）
cd ..
docker compose -f deploy/docker-compose.simple.yml up -d --build
docker compose -f deploy/docker-compose.simple.yml logs -f gospeak
```

与完整版的区别：

| | 简化版 | 完整版 |
|---|---|---|
| 服务 | app + livekit | app + livekit + 可选 postgres/rustfs/observability |
| profiles | 无，`up` 即起 | 靠 `COMPOSE_PROFILES` 选档 |
| 数据库 | SQLite（内嵌） | 可选 PostgreSQL |
| 对象存储 | 本地磁盘 | 可选 RustFS/S3 |
| 监控 | 无 | Prometheus/Grafana/Loki/Alertmanager |

两个文件共用同一份 `deploy/.env` 和同名 volume，**可在两者间切换而不丢数据**。
需要 PostgreSQL、对象存储或监控时改用完整版 `docker-compose.yml`。

## 2. 三种场景

按你的实际情况选一个，不要混用。

### 场景一：有域名，用内置 Caddy（推荐）

```bash
cd deploy
./init.sh --domain voice.example.com   # 自动写入域名并把 edge 追加进 COMPOSE_PROFILES
docker compose up -d --build
```

跑完 `deploy/.env` 里就是：

```ini
COMPOSE_PROFILES=app,livekit,edge
GOSPEAK_DOMAIN=voice.example.com
```

- Caddy 自动申请并续期证书，自动把 HTTP 重定向到 HTTPS。
- 域名解析到本机公网 IP，防火墙放行 80/443 即可。
- 不需要额外写任何反代规则，也不需要给 LiveKit 配域名或证书。

### 场景二：已有反向代理（Nginx / Traefik / Dokploy / 宝塔）

**不要开 `edge` 档位，也不要传 `--domain`。** 把现有反代指向 `gospeak:8998` 这一个上游即可：

```bash
cd deploy
./init.sh                      # 不传 --domain，edge 不会被加进来
docker compose up -d --build
```

此时 `deploy/.env` 里保持：

```ini
COMPOSE_PROFILES=app,livekit
GOSPEAK_DOMAIN=
GOSPEAK_PORT=8998
```

反代侧只需要转发到 `http://<宿主机>:8998`，并且：

- 保留 `Host` 与 `X-Forwarded-Proto` 请求头（应用靠 `X-Forwarded-Proto` 判断该推导 `ws://` 还是 `wss://`）
- 为 WebSocket 放行升级（`/ws` 与 `/livekit` 都是 WebSocket 长连接）
- **不需要为 LiveKit 写任何 location**。`/livekit` 由应用内部转发，反代看到的就是普通 HTTP 请求。

Nginx 最小片段见 [反代与 HTTPS](/deploy/nginx)。

> 如果之前误传了 `--domain`，把 `edge` 从 `COMPOSE_PROFILES` 里去掉即可（否则 Caddy 会和你的反代抢 80/443）。

### 场景三：局域网 / 无域名测试

不传 `--domain`，`GOSPEAK_DOMAIN` 保持留空。此时有两种玩法：

**A. 不要入口层，直接访问 8998**（最简单）

```ini
# deploy/.env
COMPOSE_PROFILES=app,livekit
GOSPEAK_DOMAIN=
```

浏览器打开 `http://<服务器IP>:8998`。

**B. 用内置 Caddy 统一到 80 端口**（需要手工加 `edge`，因为没传 `--domain` 不会被自动追加）

```ini
# deploy/.env
COMPOSE_PROFILES=app,livekit,edge
GOSPEAK_DOMAIN=
```

`GOSPEAK_DOMAIN` 留空时，Caddy 的站点地址退化为 `:80`，只提供纯 HTTP，不申请证书也不跳转 HTTPS。浏览器打开 `http://<服务器IP>` 即可。

> 注意：浏览器只在 **localhost 或 HTTPS** 下授予麦克风权限。用 `http://<局域网IP>` 打开时页面能进，但语音功能不可用。局域网内想验证语音，请用 SSH 端口转发到 `http://localhost:8998`，或按场景一配一个真域名。

---

## 3. 端口与防火墙

| 端口 | 协议 | 服务 | 说明 |
|------|------|------|------|
| 80 | TCP | Caddy / 自有反代 | HTTP 入口，有域名时自动 301 到 443 |
| 443 | TCP | Caddy / 自有反代 | HTTPS 入口 |
| 443 | UDP | Caddy | HTTP/3（可选，不开放也能用） |
| 7882 | UDP | LiveKit | 媒体主通道，必须开 |
| 7882 | TCP | LiveKit | UDP 不通时的媒体兜底，必须开 |
| 3478 | UDP | LiveKit | TURN 中继，NAT 严格时靠它 |
| 8998 | TCP | gospeak | 应用端口，仅自有反代场景需要可达（不要直接暴露公网） |

**不需要再开 7880 / 7881。** 7880 是 LiveKit 信令，compose 里只绑到宿主机回环（`127.0.0.1:7880`），供本机 `pnpm dev` 调试直连，**不对公网开放**；浏览器经应用同源的 `/livekit` 访问它。7881 已不再发布。

Ubuntu 防火墙示例：

```bash
ufw allow ssh
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 443/udp
ufw allow 7882/tcp
ufw allow 7882/udp
ufw allow 3478/udp
```

云服务器除了系统防火墙，还有一层云厂商安全组，**两层都要放行**。媒体端口没开是「能进频道但没声音」最常见的原因。

> 媒体是 UDP 直连，不经过 Caddy 或任何 HTTP 反代。所以不能用「只代理 HTTP」的 CDN 来兜媒体流量。

---

## 4. 环境变量关键项

完整变量见 `deploy/.env.example`（分区注释）。部署相关关键项：

| 变量 | 默认 | 作用 |
|------|------|------|
| `COMPOSE_PROFILES` | `app,livekit` | 档位选择，改一次永久生效；加 `edge` 才起内置 Caddy |
| `GOSPEAK_DOMAIN` | 空 | 唯一需要填的域名项。填了 Caddy 自动 HTTPS；留空 Caddy 只听 80 |
| `GOSPEAK_PORT` | `8998` | 宿主机映射端口（自有反代 / Dokploy 指向这里；Caddy 档位可忽略） |
| `LIVEKIT_HOST` | `ws://livekit:7880` | 后端访问 LiveKit 的内网地址，容器部署不要改 |
| `LIVEKIT_PUBLIC_URL` | **空** | 下发给浏览器的 `serverUrl`。留空即按请求自动推导为同源 `/livekit` |
| `LIVEKIT_KEY` / `LIVEKIT_SECRET` | 随机值 | 唯一密钥来源，compose 注入 LiveKit 容器，`livekit.yaml` 不存凭据 |
| `STORAGE_ENCRYPT_KEY` | — | 生产必填，64 位 hex，加密存储配置里的敏感字段 |
| `METRICS_TOKEN` | 空 | 可选，`/metrics` 的 Bearer token；留空表示不鉴权 |

### 4.1 LiveKit 地址：为什么默认留空

这是这次简化最核心的一点，以前最容易踩坑的地方现在不需要操心了。

- `LIVEKIT_HOST`：后端调用 LiveKit admin API（房间管理、踢人）用，走容器内网 `ws://livekit:7880`。**保持默认即可。**
- `LIVEKIT_PUBLIC_URL`：`POST /api/v1/signal/token` 返回给前端的 `serverUrl`。**默认留空**，应用会按请求本身推导：
  - scheme 取 `X-Forwarded-Proto`，缺省按是否 TLS 判断
  - 浏览器打开 `https://voice.example.com` → `serverUrl = wss://voice.example.com/livekit`
  - livekit-client 会在这个地址后自动拼 `/rtc/v1`，经路径前缀反代握手正常

所以：**LiveKit 不需要域名、不需要证书、不需要单独的 `wss://` 地址。** 你也不用再去填一个「浏览器可达的地址」了。

只有这两种情况才显式填写 `LIVEKIT_PUBLIC_URL`：

1. LiveKit 用**独立域名**对外（如 `wss://lk.example.com`），此时应用不再走同源 `/livekit`
2. 使用 **LiveKit Cloud**（如 `wss://xxx.livekit.cloud`）

### 4.2 密钥只有一份

`deploy/.env` 的 `LIVEKIT_KEY` / `LIVEKIT_SECRET` 是唯一来源。compose 以 `LIVEKIT_KEYS` / `LIVEKIT_WEBHOOK_API_KEY` 环境变量注入 LiveKit 容器，`livekit/livekit.yaml` 内 `keys: {}` 不再硬编码任何凭据，因此不存在「`.env` 与 yaml 两侧 key 不一致」的问题。

LiveKit 1.13 起 `webhook.api_key` 必须显式指定且必须存在于 keys 中，否则容器启动即失败（`api_key is required to use webhooks`）。compose 已自动注入。

---

## 5. 验证

```bash
# 容器状态：gospeak-app 应为 healthy
docker compose ps

# 应用健康检查（直接打容器映射端口）
curl -s http://localhost:8998/ping

# 走 Caddy 验证入口（有域名时）
curl -s https://voice.example.com/ping

# 证书是否已签发（有域名时）
curl -sI https://voice.example.com | head -1
```

浏览器里再确认三件事：

1. 页面能打开，地址栏是 HTTPS（或 localhost）
2. 能登录
3. 进语音频道后能看到自己的头像亮起、说话有绿圈

`serverUrl` 推导是否正确，可以在浏览器开发者工具 Network 面板里看 `/api/v1/signal/token` 的响应，`serverUrl` 应为 `wss://<你的域名>/livekit`。

---

## 6. 升级、停止、备份

### 升级

```bash
git pull
cd deploy
docker compose up -d --build
```

数据库表结构启动时自动迁移。升级前先备份。

### 停止

```bash
docker compose down          # 停服务，数据卷保留
docker compose down -v       # 连数据卷一起删（不可逆）
```

### 备份

```bash
# SQLite（默认）
docker run --rm -v gospeak_gospeak-db:/data -v "$PWD":/backup alpine \
  cp /data/app.db /backup/app-backup-$(date +%F).db

# PostgreSQL
docker compose exec -T postgres pg_dump -U gospeak gospeak | gzip > backup-$(date +%F).sql.gz

# 内嵌 NATS JetStream（JWT 签名密钥 / 黑名单 / refresh-family）
# 务必在**应用停止**时整体拷贝，避免运行中拷贝产生不一致快照
docker compose stop gospeak
docker run --rm -v gospeak_gospeak-nats:/data -v "$PWD":/backup alpine \
  tar czf /backup/nats-$(date +%F).tar.gz -C /data .
docker compose start gospeak
```

> **为什么必须备份这个卷**：`gospeak-nats` 保存 JWT 签名密钥、登出黑名单和 refresh-token 重放保护。
> 它丢了不会波及业务数据，但会导致**全体用户被强制登出**（签名密钥重新生成，旧 token 全部验签失败），
> 且登出吊销的 token 在过期前可重新使用。升级、迁移、`down -v` 前请一并处理。

上传的文件在 `gospeak-uploads` 卷或你配置的对象存储里，按同样思路备份。设成定时任务，不要手动。

---

## 7. 可选档位

档位写在 `deploy/.env` 的 `COMPOSE_PROFILES`，改一次之后所有命令都是裸 `docker compose ...`，不用每次加 `--profile`。

| 档位 | 追加到 COMPOSE_PROFILES | 说明 |
|------|------------------------|------|
| `edge` | 有域名时加 | 内置 Caddy，自动 HTTPS；已有反代则不要加 |
| `postgres` | 人多了换掉 SQLite | 同时要改 DB 段配置 |
| `rustfs` | 要对象存储 / 多实例 | S3 兼容，控制台在 9001 |
| `observability` | 想看监控 | Prometheus + Grafana + Alertmanager + Loki |

常用组合：

```ini
# 最小可用（默认）
COMPOSE_PROFILES=app,livekit

# 有域名，用内置 Caddy 自动 HTTPS
COMPOSE_PROFILES=app,livekit,edge

# 小团队生产
COMPOSE_PROFILES=app,livekit,edge,postgres

# 完整
COMPOSE_PROFILES=app,livekit,edge,postgres,rustfs
```

### PostgreSQL

```ini
COMPOSE_PROFILES=app,livekit,edge,postgres
DB_TYPE=PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=gospeak
DB_PASSWORD=<强密码>
POSTGRES_PASSWORD=<同一个密码>
```

`DB_PASSWORD` 与 `POSTGRES_PASSWORD` 必须一致：前者给应用用，后者给容器初始化用。**已有的 SQLite 数据不会自动迁移**，需要手动导出。

### RustFS 对象存储

```ini
COMPOSE_PROFILES=app,livekit,edge,postgres,rustfs
STORAGE_TYPE=s3
STORAGE_ENDPOINT=http://rustfs:9000
STORAGE_BUCKET=gospeak
STORAGE_ACCESS_KEY=<改掉默认值>
STORAGE_SECRET_KEY=<改掉默认值>
RUSTFS_ACCESS_KEY=<和上面一致>
RUSTFS_SECRET_KEY=<和上面一致>
```

生产环境必须设置 `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`，不要用默认的 `rustfsadmin/rustfsadmin`。

### 可观测栈

```ini
COMPOSE_PROFILES=app,livekit,edge,observability
```

- Prometheus：`http://localhost:9090`
- Grafana：`http://localhost:3000`（默认 `admin/admin`，预置 GOSpeak Overview Dashboard 与 Prometheus/Loki 数据源）
- Alertmanager：`http://localhost:9093`
- Loki：`http://localhost:3100`；Promtail 自动采集所有 Docker 容器 stdout 日志
- 应用暴露 `GET /metrics`；设置 `METRICS_TOKEN` 后需要 `Authorization: Bearer <token>`

这些端口不要直接暴露到公网，用防火墙限制，或在反代上加 basic auth。

> MinIO → RustFS 迁移提示：v1.x 起对象存储由 MinIO 换为 RustFS，volume 由 `minio-data` 更名为 `rustfs-data`。升级后旧数据不会自动迁移：如需保留旧文件请先从 `minio-data` 导出拷贝到 `rustfs-data`；确认无误后执行 `docker volume rm gospeak_minio-data` 清理。

---

## 8. 发布前 Checklist

- [ ] `./init.sh --check` 通过（确认 `.env` 与 `livekit.yaml` 一致、`Caddyfile` 存在）
- [ ] `JWT_KEY` / `LIVEKIT_KEY` / `LIVEKIT_SECRET` / `STORAGE_ENCRYPT_KEY` 已是随机值（`init.sh` 自动生成）
- [ ] `LIVEKIT_PUBLIC_URL` 保持留空（除非确实用独立域名或 LiveKit Cloud）
- [ ] 有域名时 `GOSPEAK_DOMAIN` 已填、`COMPOSE_PROFILES` 含 `edge`
- [ ] 已有反代时 `COMPOSE_PROFILES` **不含** `edge`，反代已指向 `gospeak:8998`
- [ ] 防火墙放行 80/443、7882/tcp、7882/udp、3478/udp（云安全组同步）
- [ ] 默认管理员密码已改（默认 `admin123`）
- [ ] 叠加 postgres 时 `DB_PASSWORD` 非默认且与 `POSTGRES_PASSWORD` 一致
- [ ] `CORS_ORIGIN` 设成你的域名，不是 `*`
- [ ] 备份策略已就位（`/app/db` 卷或 pg_dump）

---

## 9. NATS 信号事件总线（内嵌）

- gospeak 进程固定使用**内嵌 nats-server**（默认随机本机端口；可用 `NATS_EMBEDDED_PORT` 固定，如 `4222`），单二进制零依赖，不再支持外部 `NATS_URL`。
- 房间 membership/stream 存储走 NATS JetStream KV，内嵌 NATS 随进程启动，NATS 不可用时启动失败（fail-fast）。
- 监控：SSE health 含 `eventbus_mode`（固定 `embedded`）、`eventbus_connected`。

---

## 相关文档

- [反代与 HTTPS](/deploy/nginx) — 内置 Caddy 档位与已有反代的接法
- [Docker Compose 详解](/deploy/compose) — 每个档位、每个常用操作
- [故障排查](/deploy/troubleshooting) — 按症状查
- [单二进制部署](/deploy/binary) — 不用 Docker 时
