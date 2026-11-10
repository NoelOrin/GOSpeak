# Docker Compose 详解

`deploy/docker-compose.yml` 把整套服务编排在一起。这篇讲清楚每个组件、每个档位、每个常用操作。

## 为什么用档位而不是多份 compose 文件

传统做法是维护好几个 compose 文件（基础版、完整版、生产版），改一处要同步好几处。

GOSpeak 只用一个文件，靠 `COMPOSE_PROFILES` 决定启哪些服务：

```ini
COMPOSE_PROFILES=app,livekit
```

这个值写在 `deploy/.env` 里，所以之后所有命令都是裸的：

```bash
docker compose up -d        # 不用加 --profile
docker compose logs -f
docker compose down
```

## 六个档位

| 档位 | 起什么 | 什么时候要 |
|------|--------|-----------|
| `app` | GOSpeak 应用 | 永远要 |
| `livekit` | LiveKit 语音服务 | 自建语音就要 |
| `edge` | 内置 Caddy 入口 | 有域名、想让 Caddy 自动 HTTPS 时 |
| `postgres` | PostgreSQL | 人多了换掉 SQLite |
| `rustfs` | RustFS 对象存储 | 要 CDN 或多实例 |
| `observability` | Prometheus + Grafana + Loki + Alertmanager | 想看监控 |

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

### `edge` 是什么

`edge` 起一个 `caddy:2-alpine` 容器，读 `deploy/Caddyfile`，自动申请证书、自动 HTTPS、自动处理 WebSocket 升级。它唯一的用途就是「有域名时省掉手工配反代」。

两种情况**不要**开 `edge`：

- 前面已经有 Nginx / Traefik / Dokploy / 宝塔在终结 TLS（会抢 80/443）
- 纯局域网、没有域名

不开 `edge` 时，`app,livekit` 就够了，反代或浏览器直接访问 `8998`。对应做法是 `./init.sh` 时**不传 `--domain`**。

## 从零到跑起来

```bash
cd deploy
./init.sh
docker compose up -d --build
```

`init.sh` 做了这些事：

1. 生成 `deploy/.env`（已存在则报错，加 `--force` 覆盖）
2. 随机生成 `JWT_KEY`、`LIVEKIT_KEY`、`LIVEKIT_SECRET`、`STORAGE_ENCRYPT_KEY`
3. 归一 `livekit/livekit.yaml`：剥离硬编码 `keys` 与 `webhook.api_key`
4. 校验 `.env` / `livekit.yaml` / `Caddyfile` 一致性，不一致直接退出

第 3 步很关键：密钥只在 `deploy/.env` 里有一份，由 compose 注入 LiveKit 容器，`livekit.yaml` 不含任何凭据，所以不存在「两边密钥对不上」的问题。

有域名的话加上 `--domain`：

```bash
./init.sh --domain voice.example.com
```

`./init.sh --domain voice.example.com` 会写入域名并自动把 `edge` 加进 `COMPOSE_PROFILES`（已有 `app,livekit` 时追加为 `app,livekit,edge`）。跑完直接 `docker compose up -d --build` 即可，无需手改档位。

**不开 `edge` 的场景**：已有反代（Nginx / Traefik / Dokploy / 宝塔），或纯局域网无域名时，**不要传 `--domain`**（或手动把 `edge` 从 `COMPOSE_PROFILES` 里去掉），反代指向宿主机 `8998`。

## 服务之间怎么连

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

关键点：

- **一个上游。** `/api`、`/ws`、`/uploads`、`/livekit` 全在 `gospeak:8998` 里，反代层只面对它。
- **LiveKit 不需要域名或证书。** 浏览器访问 `https://<你的域名>/livekit`，应用内部转发到 `ws://livekit:7880`，WebSocket 升级由应用处理。
- **媒体流量不经过应用和反代。** 浏览器和 LiveKit 之间直连 WebRTC（7882/udp 为主），应用只负责签发令牌。这样应用不会被音频流量拖垮。
- **7880 不对公网开放。** 它只绑宿主机回环（`127.0.0.1:7880`）供本机调试；对外信令走应用同源反代。

## 只起依赖，应用在本地跑

开发时经常需要这样——Docker 里跑 LiveKit 和数据库，应用用 `pnpm dev` 跑，方便改代码：

```bash
COMPOSE_PROFILES=livekit,rustfs docker compose up -d
```

然后回仓库根目录：

```bash
pnpm dev:server
pnpm dev:web
```

这时应用跑在宿主机上，不在 Docker 网络里，所以 `LIVEKIT_HOST` 要填 `ws://localhost:7880`（不是服务名 `livekit`）。

这能直接工作，因为 compose 里 `livekit` 的 7880 端口绑到了宿主机回环（`127.0.0.1:7880`）：**只供本机调试直连，不对公网开放**。生产上浏览器不碰这个端口，信令走应用同源的 `/livekit`。

`LIVEKIT_PUBLIC_URL` 保持留空即可，本地访问 `http://localhost:3000` 时应用会推导出 `ws://localhost:8998/livekit`。

## 换数据库

改 `deploy/.env`：

```ini
COMPOSE_PROFILES=app,livekit,edge,postgres
DB_TYPE=PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=gospeak
DB_PASSWORD=<强密码>
POSTGRES_PASSWORD=<同一个密码>
```

注意 `DB_PASSWORD` 和 `POSTGRES_PASSWORD` 要一致——前者给应用用，后者给容器初始化用。

然后：

```bash
docker compose up -d
```

**已有的 SQLite 数据不会自动迁移**，得手动导。

## 换对象存储

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

RustFS 的控制台在 9001 端口，可以上去看看桶建出来没有。

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

**注意**：改 `.env` 之后必须用 `up -d`，不能用 `restart`。`restart` 只是重启进程，不会重新读环境变量。

## 镜像从哪来

默认本地构建。想用官方发布镜像省掉构建时间，在 `.env` 里加：

```ini
GOSPEAK_IMAGE=ghcr.io/noelorin/gospeak:1
GOSPEAK_PULL_POLICY=missing
```

`missing` 表示本地没有才拉，`always` 表示每次都拉最新。

## 更新版本

```bash
cd GOSpeak
git pull
cd deploy
docker compose up -d --build
```

数据库表结构启动时自动迁移。升级前备份一下是好习惯。

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

**7880 / 7881 不对公网发布。** 7880 只绑宿主机回环（`127.0.0.1:7880`）供本机调试，信令对外走应用同源反代；7881 不再发布。

## 常见问题

**构建卡在拉基础镜像**

国内网络问题。配 Docker 镜像加速，或者用 `GOSPEAK_IMAGE` 直接拉已构建好的镜像。

**容器起来了但访问 502**

Caddy 或反代先起来了，应用还没就绪。等一下，或者 `docker compose logs gospeak` 看应用是不是在报错。

**改了 .env 没生效**

用了 `restart` 而不是 `up -d`。

**`docker compose down -v` 之后数据没了**

`-v` 会删数据卷。这是不可逆的，只在确实想清空时用。

**Caddy 一直申请证书失败**

`edge` 档位和别的反代抢了 80/443，或者域名 A 记录没指到本机。已有反代时不要开 `edge`。

**LiveKit 起来了但连不上**

先看 `docker compose logs livekit`。常见原因是 `use_external_ip` 探测到的 IP 不对——云服务器有多块网卡时容易探错，手动改 `livekit/livekit.yaml` 里的 `node_ip`。

## 下一步

- [反代与 HTTPS](/deploy/nginx) — 内置 Caddy 与已有反代
- [监控](/deploy/monitoring) — 可观测档位怎么用
- [故障排查](/deploy/troubleshooting) — 按症状查
