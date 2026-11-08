# Docker Compose 详解

`deploy/docker-compose.yml` 把整套服务编排在一起。这篇讲清楚每个组件、每个档位、每个常用操作。

## 为什么用档位而不是多份 compose 文件

传统做法是维护好几个 compose 文件（基础版、完整版、生产版），改一处要同步好几处。

GOSpeak 只用一个文件，靠 `COMPOSE_PROFILES` 决定启哪些服务：

```ini
COMPOSE_PROFILES=livekit,app
```

这个值写在 `deploy/.env` 里，所以之后所有命令都是裸的：

```bash
docker compose up -d        # 不用加 --profile
docker compose logs -f
docker compose down
```

## 五个档位

| 档位 | 起什么 | 什么时候要 |
|------|--------|-----------|
| `app` | GOSpeak 应用 | 永远要 |
| `livekit` | LiveKit 语音服务 | 自建语音就要 |
| `postgres` | PostgreSQL | 人多了换掉 SQLite |
| `rustfs` | RustFS 对象存储 | 要 CDN 或多实例 |
| `observability` | Prometheus + Grafana + Loki + Alertmanager | 想看监控 |

### 组合建议

```ini
# 最小可用
COMPOSE_PROFILES=livekit,app

# 小团队生产
COMPOSE_PROFILES=postgres,livekit,app

# 完整
COMPOSE_PROFILES=postgres,livekit,rustfs,app

# 再加监控
COMPOSE_PROFILES=postgres,livekit,rustfs,observability,app
```

## 从零到跑起来

```bash
cd deploy
./init.sh
docker compose up -d --build
```

`init.sh` 做了这些事：

1. 生成 `deploy/.env`（已存在则报错，加 `--force` 覆盖）
2. 随机生成 `JWT_KEY`、`LIVEKIT_KEY`、`LIVEKIT_SECRET`、`STORAGE_ENCRYPT_KEY`
3. 探测公网 IP，写进 LiveKit 的 `rtc.use_external_ip` 和 candidate 配置
4. 把 LiveKit 密钥同步写进 `livekit/livekit.yaml`

第 4 步很关键：webhook 签名校验要求应用和 LiveKit 的密钥完全一致。手动改的话容易忘。

## 服务之间怎么连

```
        公网
          │
      ┌───▼────┐
      │ nginx  │  :80 / :443
      └───┬────┘
          │ /api /ws / → gospeak:8998
      ┌───▼─────┐        ┌──────────┐
      │ gospeak │───────►│ postgres │  可选
      └───┬─────┘        └──────────┘
          │ LIVEKIT_HOST=ws://livekit:7880
      ┌───▼─────┐
      │ livekit │  :7880 信令  :7881/:7882 媒体
      └─────────┘
```

关键点：**媒体流量不经过应用和 Nginx**。浏览器和 LiveKit 之间直连 WebRTC，应用只负责签发令牌。这样应用不会被音频流量拖垮。

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

这时 `app/server/.env.dev` 里的 `LIVEKIT_HOST` 要指向 `ws://localhost:7880`（不是 `livekit`，因为应用在宿主机上跑，不在 Docker 网络里）。

## 换数据库

改 `deploy/.env`：

```ini
COMPOSE_PROFILES=postgres,livekit,app
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
COMPOSE_PROFILES=postgres,livekit,rustfs,app
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

## 常见问题

**构建卡在拉基础镜像**
国内网络问题。配 Docker 镜像加速，或者用 `GOSPEAK_IMAGE` 直接拉已构建好的镜像。

**容器起来了但访问 502**
Nginx 先起来了，应用还没就绪。等一下，或者 `docker compose logs gospeak` 看应用是不是在报错。

**改了 .env 没生效**
用了 `restart` 而不是 `up -d`。

**`docker compose down -v` 之后数据没了**
`-v` 会删数据卷。这是不可逆的，只在确实想清空时用。

**LiveKit 起来了但连不上**
先看 `docker compose logs livekit`。常见原因是密钥不一致，或者 `use_external_ip` 探测到的 IP 不对——云服务器有多块网卡时容易探错，手动改 `livekit/livekit.yaml` 里的 `node_ip`。

## 下一步

- [Nginx 配置](/deploy/nginx) — 反代和 HTTPS
- [监控](/deploy/monitoring) — 可观测档位怎么用
- [故障排查](/deploy/troubleshooting) — 按症状查
