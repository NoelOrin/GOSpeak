# Docker Compose 详解

`deploy/docker-compose.yml` 将整套服务编排在一起。本文逐一说明每个组件、每个档位与常用操作。

## 单文件与档位机制

传统做法是维护多份 compose 文件（基础版、完整版、生产版），一处修改需要在多处同步。

GOSpeak 只使用一个文件，由 `COMPOSE_PROFILES` 决定启用的服务：

```ini
COMPOSE_PROFILES=app,livekit
```

该值写在 `deploy/.env` 中，因此后续命令无需再携带 profile 参数：

```bash
docker compose up -d        # 不用加 --profile
docker compose logs -f
docker compose down
```

## 六个档位

| 档位 | 功能 | 启用条件 |
|------|--------|-----------|
| `app` | GOSpeak 应用 | 必启用 |
| `livekit` | LiveKit 语音服务 | 使用内置语音服务时 |
| `edge` | 内置 Caddy 入口 | 拥有域名、由 Caddy 自动签发 HTTPS 证书时 |
| `postgres` | PostgreSQL | 用户规模增长后替换 SQLite |
| `rustfs` | RustFS 对象存储 | 需要 CDN 或多实例部署时 |
| `observability` | Prometheus + Grafana + Loki + Alertmanager | 需要监控时 |

### 组合建议

```ini
# 最小可用（默认，局域网或前面已有反代）
COMPOSE_PROFILES=app,livekit

# 有域名，内置 Caddy 自动 HTTPS（推荐）
COMPOSE_PROFILES=app,livekit,edge

# 小团队生产
COMPOSE_PROFILES=app,livekit,edge,postgres

# 完整
COMPOSE_PROFILES=app,livekit,edge,postgres,rustfs

# 再加监控
COMPOSE_PROFILES=app,livekit,edge,postgres,rustfs,observability
```

### `edge` 组件的功能

`edge` 启动一个 `caddy:2-alpine` 容器，读取 `deploy/Caddyfile`，自动申请证书、自动启用 HTTPS、自动处理 WebSocket 升级。其唯一用途是在拥有域名时省去手工配置反向代理。

以下两种情况**不应**启用 `edge`：

- 已有 Nginx / Traefik / Dokploy / 宝塔在前方终结 TLS（会占用 80/443 端口）
- 纯局域网环境、没有域名

不启用 `edge` 时，`app,livekit` 两个档位已经足够，反向代理或浏览器直接访问 `8998` 端口。对应做法是在 `./init.sh` 时**不传 `--domain`**。

## 首次启动流程

```bash
cd deploy
./init.sh
docker compose up -d --build
```

`init.sh` 完成以下工作：

1. 生成 `deploy/.env`（已存在时报错，加 `--force` 覆盖）
2. 随机生成 `JWT_KEY`、`LIVEKIT_KEY`、`LIVEKIT_SECRET`、`STORAGE_ENCRYPT_KEY`
3. 归一 `livekit/livekit.yaml`：剥离硬编码 `keys` 与 `webhook.api_key`
4. 校验 `.env` / `livekit.yaml` / `Caddyfile` 的一致性，不一致时直接退出

第 3 步十分关键：密钥只在 `deploy/.env` 中保存一份，由 compose 注入 LiveKit 容器，`livekit.yaml` 不含任何凭据，因此不存在「两边密钥对不上」的问题。

拥有域名时加上 `--domain` 参数：

```bash
./init.sh --domain voice.example.com
```

`./init.sh --domain voice.example.com` 会写入域名并自动将 `edge` 加入 `COMPOSE_PROFILES`（已有 `app,livekit` 时追加为 `app,livekit,edge`）。执行完成后直接运行 `docker compose up -d --build`，无需手动修改档位。

**不启用 `edge` 的场景**：已有反向代理（Nginx / Traefik / Dokploy / 宝塔），或纯局域网无域名时，**不传 `--domain`**（或手动将 `edge` 从 `COMPOSE_PROFILES` 中移除），反向代理指向宿主机 `8998` 端口。

## 服务间连接关系

```
                   公网
                    │
            ┌───────▼────────┐
            │ Caddy（edge）   │  :80 / :443，可选
            └───────┬────────┘
                    │ 唯一上游
            ┌───────▼────────┐        ┌──────────┐
            │    gospeak     │───────►│ postgres │  可选
            │     :8998      │        └──────────┘
            └───┬────────┬───┘
                │        │ LIVEKIT_HOST=ws://livekit:7880（容器内网）
                │        ▼
                │   ┌──────────┐
                │   │ livekit  │  :7880 信令（仅回环）
                │   │          │  :7882 tcp/udp 媒体
                │   └────▲─────┘  :3478/udp TURN
                │        │
                └────────┼──── /livekit 同源反代（应用内置）
                         │
              浏览器 ──WebRTC(UDP)──► 媒体直连
```

关键点如下：

- **单一上游。** `/api`、`/ws`、`/uploads`、`/livekit` 全部由 `gospeak:8998` 提供，反向代理层只需对接这一个地址。
- **LiveKit 不需要域名或证书。** 浏览器访问 `https://<域名>/livekit`，应用内部转发到 `ws://livekit:7880`，WebSocket 升级由应用处理。
- **媒体流量不经过应用和反代。** 浏览器与 LiveKit 之间直接建立 WebRTC 连接（以 7882/udp 为主），应用只负责签发令牌。应用因此不会被音频流量拖垮。
- **7880 端口不对公网开放。** 该端口只绑定宿主机回环地址（`127.0.0.1:7880`）供本机调试；对外信令走应用同源反代。

## 仅启动依赖服务，应用在本地运行

开发时经常采用这种方式：LiveKit 和数据库运行在 Docker 中，应用通过 `pnpm dev` 运行，便于修改代码：

```bash
COMPOSE_PROFILES=livekit,rustfs docker compose up -d
```

然后回到仓库根目录：

```bash
pnpm dev:server
pnpm dev:web
```

此时应用运行在宿主机上，不在 Docker 网络内，因此 `LIVEKIT_HOST` 需要填写 `ws://localhost:7880`（而不是服务名 `livekit`）。

这种方式能够直接工作，原因是 compose 中 `livekit` 的 7880 端口绑定到了宿主机回环地址（`127.0.0.1:7880`）：**只供本机调试直连，不对公网开放**。生产环境中浏览器不会访问该端口，信令走应用同源的 `/livekit`。

`LIVEKIT_PUBLIC_URL` 保持留空，本地访问 `http://localhost:3000` 时应用会推导出 `ws://localhost:8998/livekit`。

## 切换数据库

修改 `deploy/.env`：

```ini
COMPOSE_PROFILES=app,livekit,edge,postgres
DB_TYPE=PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=gospeak
DB_PASSWORD=<强密码>
POSTGRES_PASSWORD=<同一个密码>
```

`DB_PASSWORD` 与 `POSTGRES_PASSWORD` 必须保持一致——前者供应用使用，后者供容器初始化使用。

修改完成后：

```bash
docker compose up -d
```

**已有的 SQLite 数据不会自动迁移**，需要手动导入。

## 切换对象存储

```ini
COMPOSE_PROFILES=app,livekit,edge,postgres,rustfs
STORAGE_TYPE=s3
STORAGE_ENDPOINT=http://rustfs:9000
STORAGE_BUCKET=gospeak
STORAGE_ACCESS_KEY=<改掉默认值>
STORAGE_SECRET_KEY=<改掉默认值>
STORAGE_REGION=us-east-1
RUSTFS_ACCESS_KEY=<和上面一致>
RUSTFS_SECRET_KEY=<和上面一致>
```

RustFS 控制台运行在 9001 端口，可用于确认桶是否已创建。

## 常用操作

```bash
# 起
docker compose up -d

# 改了代码或配置后重建
docker compose up -d --build

# 看日志（只跟应用）
docker compose logs -f gospeak

# 看所有服务状态
docker compose ps

# 进应用容器看文件
docker compose exec gospeak sh

# 停（数据留着）
docker compose down

# 停并删数据（慎用）
docker compose down -v

# 只重启应用，不动数据库
docker compose restart gospeak
```

**注意**：修改 `.env` 之后必须使用 `up -d`，不能使用 `restart`。`restart` 只重启进程，不会重新读取环境变量。

## 镜像来源

默认使用本地构建。改用官方发布镜像可以省去构建时间，方法是在 `.env` 中添加：

```ini
GOSPEAK_IMAGE=ghcr.io/noelorin/gospeak:1
GOSPEAK_PULL_POLICY=missing
```

`missing` 表示本地不存在镜像时才拉取，`always` 表示每次都拉取最新版本。

## 更新版本

```bash
cd GOSpeak
git pull
cd deploy
docker compose up -d --build
```

数据库表结构在启动时自动迁移。升级前进行备份是良好的操作习惯。

## 端口一览

| 端口 | 服务 | 说明 |
|------|------|------|
| 80 / 443 | caddy（`edge`） | 可选；内置 HTTPS 入口 |
| 8998 | gospeak | 应用端口（`GOSPEAK_PORT` 可改） |
| 7882 tcp/udp | livekit | 媒体，必须对公网开放 |
| 3478/udp | livekit | TURN 中继 |
| 5432 | postgres | 可选，不要暴露公网 |
| 9000 / 9001 | rustfs | 可选，不要暴露公网 |
| 9090 / 9093 / 3000 / 3100 | observability | 可选，不要暴露公网 |

**7880 / 7881 端口不对公网发布。** 7880 只绑定宿主机回环地址（`127.0.0.1:7880`）供本机调试，信令对外走应用同源反代；7881 不再发布。

## 常见问题

**构建卡在拉取基础镜像**

国内网络所致。可配置 Docker 镜像加速，或使用 `GOSPEAK_IMAGE` 直接拉取已构建好的镜像。

**容器已启动但访问返回 502**

Caddy 或反向代理先启动，应用尚未就绪。等待片刻，或通过 `docker compose logs gospeak` 查看应用是否在报错。

**修改 .env 后未生效**

使用了 `restart` 而不是 `up -d`。

**`docker compose down -v` 之后数据丢失**

`-v` 参数会删除数据卷。该操作不可逆，仅在确实需要清空数据时使用。

**Caddy 申请证书持续失败**

`edge` 档位与其他反代抢占 80/443 端口，或域名 A 记录未指向本机。已有反代时不应启用 `edge`。

**LiveKit 已启动但无法连接**

先查看 `docker compose logs livekit`。常见原因是 `use_external_ip` 探测到的 IP 不正确——云服务器有多块网卡时容易探测错误，需手动修改 `livekit/livekit.yaml` 中的 `node_ip`。

## 下一步

- [反代与 HTTPS](/deploy/nginx) — 内置 Caddy 与已有反代
- [监控](/deploy/monitoring) — 可观测档位的使用方法
- [故障排查](/deploy/troubleshooting) — 按症状索引
