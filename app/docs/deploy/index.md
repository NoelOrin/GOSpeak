# 部署到服务器

这篇讲把 GOSpeak 放到一台服务器上跑起来。假设你有一台 Linux 机器，能 SSH 进去，装了 Docker。

## 先说结论

部署这件事现在只有三个变量：

1. **有没有域名** —— 有域名就填 `GOSPEAK_DOMAIN`，内置 Caddy 自动 HTTPS
2. **有没有现成的反向代理** —— 有就别开 `edge` 档位，让反代指向 `8998`
3. **防火墙** —— 只需 80/443 + 媒体端口

**只需要一个域名、一个上游。** LiveKit 不需要第二个域名、不需要单独证书、不需要单独的反代规则——信令走应用内置的同源反代 `/livekit`。

## 先想清楚三件事

**1. 多少人用？**
- 10 人以内：单机 + SQLite，够用
- 几十人：单机 + PostgreSQL，或者把数据库挪出去
- 上百人：得考虑 LiveKit 单独部署、对象存储、可能多实例

**2. 语音要自建还是用云服务？**
自建 LiveKit 省钱但要自己运维；LiveKit Cloud 不用管基础设施但按量计费。详见 [语音服务](/sfu/)。

**3. 有没有域名和 HTTPS？**
浏览器只在 HTTPS（或 localhost）下允许用麦克风。**没有 HTTPS，语音功能直接不可用。** 这是硬要求。有域名的话交给内置 Caddy 就行，不用手工折腾证书。

## 推荐路线：Docker Compose

一条命令拉起整套服务，包括应用、LiveKit，以及（可选的）内置 Caddy。

### 第一步：拿代码

```bash
git clone https://github.com/NoelOrin/GOSpeak.git
cd GOSpeak/deploy
```

### 第二步：生成配置

```bash
./init.sh --domain voice.example.com     # 没有域名就去掉 --domain
```

`./init.sh --domain voice.example.com` 会写入域名并自动把 `edge` 加进 `COMPOSE_PROFILES`（已有 `app,livekit` 时追加为 `app,livekit,edge`）。跑完直接 `docker compose up -d --build` 即可，无需手改档位。

这个脚本会：

- 生成 `deploy/.env`
- 随机生成 JWT 密钥、LiveKit 密钥、存储加密密钥
- 归一 `livekit/livekit.yaml`，让密钥只保留在 `.env` 一处
- 校验 `.env` / `livekit.yaml` / `Caddyfile` 一致性

密钥统一由 compose 从 `.env` 注入，脚本帮你对齐了，别手动改乱。

**不开 `edge` 的场景**：已有反代（Nginx / Traefik / Dokploy / 宝塔），或纯局域网无域名时，**不要传 `--domain`**（或手动把 `edge` 从 `COMPOSE_PROFILES` 里去掉），反代指向宿主机 `8998`。

### 第三步：确认档位

上一步传了 `--domain` 的话，`deploy/.env` 里应该已经是：

```ini
COMPOSE_PROFILES=app,livekit,edge
```

没传 `--domain`（已有反代 / 局域网）则是 `app,livekit`，不要有 `edge`。

### 第四步：启动

```bash
docker compose up -d --build
```

第一次会构建镜像，几分钟。之后重启很快。

### 第五步：验证

```bash
curl -s http://localhost:8998/ping      # 应用自身
curl -s https://voice.example.com/ping  # 走 Caddy（有域名时）
```

有返回就说明起来了。浏览器打开 `https://你的域名`（或 `http://服务器IP`）应该能看到界面。

## 三种场景

### 场景一：有域名，用内置 Caddy（推荐）

```bash
./init.sh --domain voice.example.com
docker compose up -d --build
```

Caddy 自动申请并续期证书、自动把 HTTP 跳转到 HTTPS、自动处理 WebSocket 升级。域名 A 记录指向本机公网 IP，防火墙放行 80/443 即可。

### 场景二：已有反向代理

**不传 `--domain`**（或手动把 `edge` 从 `COMPOSE_PROFILES` 去掉）：

```bash
./init.sh
```

此时 `.env` 里是：

```ini
COMPOSE_PROFILES=app,livekit
GOSPEAK_DOMAIN=
GOSPEAK_PORT=8998
```

把现有反代指向 `http://<宿主机>:8998` 这**一个上游**，`/api`、`/ws`、`/uploads`、`/livekit` 全部同源走它。反代侧只需保留 `Host` 与 `X-Forwarded-Proto` 头，并放行 WebSocket 升级。

**不需要给 LiveKit 写任何 location。** 详见 [反代与 HTTPS](/deploy/nginx)。

### 场景三：局域网 / 无域名测试

**不传 `--domain`**：

```bash
./init.sh
```

保持 `app,livekit`，浏览器直接访问 `http://<服务器IP>:8998`。

想让内置 Caddy 统一到 80 端口，就手动把 `edge` 加进档位（因为没传 `--domain`，不会被自动追加）：

```ini
COMPOSE_PROFILES=app,livekit,edge
GOSPEAK_DOMAIN=
```

`GOSPEAK_DOMAIN` 留空时 Caddy 只听 `:80`，纯 HTTP，不申请证书。浏览器打开 `http://<服务器IP>`。

> 麦克风需要安全上下文：只有 localhost 或 HTTPS 才给权限。`http://<局域网IP>` 能进页面，但语音用不了。局域网验证语音请用 SSH 端口转发到 `http://localhost:8998`。

## 档位怎么选

`deploy/.env` 里的 `COMPOSE_PROFILES` 决定起哪些组件。改完这一行，之后命令都是裸 `docker compose`，不用每次加 `--profile`。

| 你的情况 | 填什么 |
|---------|--------|
| 局域网 / 已有反代 | `app,livekit` |
| 有域名，自动 HTTPS | `app,livekit,edge` |
| 要 PostgreSQL | `app,livekit,edge,postgres` |
| 还要对象存储 | `app,livekit,edge,postgres,rustfs` |
| 还要监控面板 | `app,livekit,edge,postgres,rustfs,observability` |

从 SQLite 换 PostgreSQL 时，除了改档位，还要改数据库那几行：

```ini
DB_TYPE=PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=gospeak
DB_PASSWORD=<换成强密码>
POSTGRES_PASSWORD=<同一个密码>
```

**注意**：换数据库不会自动搬数据。SQLite 里已有的数据得手动导。

## 端口要开哪些

| 端口 | 协议 | 服务 | 要不要开公网 |
|------|------|------|-------------|
| 80 / 443 | TCP | Caddy 或自有反代 | 必须 |
| 443 | UDP | Caddy（HTTP/3） | 可选 |
| 7882 | UDP | LiveKit 媒体（主通道） | 必须 |
| 7882 | TCP | LiveKit 媒体（UDP 不通时兜底） | 必须 |
| 3478 | UDP | TURN 中继 | 建议开 |
| 8998 | TCP | 应用 | 仅自有反代场景需要可达，不要直接暴露公网 |
| 5432 | TCP | PostgreSQL | 绝对不要 |

**不需要 7880 / 7881。** 7880 只绑宿主机回环（`127.0.0.1:7880`）供本机 `pnpm dev` 调试直连，不对公网开放；浏览器经应用同源的 `/livekit` 访问它。7881 已不再发布。

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

**媒体端口没开是语音不通最常见的原因。** 页面能打开、能登录、能进频道，但就是听不到声音——八成是这个。云服务器除了系统防火墙，还有一层安全组，两层都要开。

## 服务器要什么配置

| 人数 | CPU | 内存 | 说明 |
|------|-----|------|------|
| 10 人内 | 1 核 | 1G | 能跑，但别指望流畅 |
| 几十人 | 2 核 | 4G | 推荐起点 |
| 上百人 | 4 核+ | 8G+ | LiveKit 单独一台机器更好 |

真正吃资源的是音频转发。人越多，带宽越重要——每个说话的人大约要 40-60 kbps 上行，听的人按人数累加。

## 备份

**SQLite**：整个 `/app/db` 目录就是数据库，拷走即可：

```bash
docker run --rm -v gospeak_gospeak-db:/data -v $(pwd):/backup alpine \
  tar czf /backup/db-$(date +%F).tar.gz -C /data .
```

**PostgreSQL**：

```bash
docker compose exec -T postgres pg_dump -U gospeak gospeak | gzip > backup-$(date +%F).sql.gz
```

**上传的文件**：`gospeak-uploads` 卷或者你的对象存储桶。

设成定时任务，别手动。

## 升级

```bash
cd GOSpeak
git pull
cd deploy
docker compose up -d --build
```

数据库表结构会自动迁移，不用手动跑 SQL。升级前先备份，这是习惯问题不是流程要求。

## 不想用 Docker

也可以只跑一个二进制文件，前端已经打进去了：

```bash
# 从 Release 下载对应平台的 gospeak-linux-amd64
chmod +x gospeak-linux-amd64
./gospeak-linux-amd64 server -e prod
```

配合 systemd 做开机自启和崩溃重启。这样部署的话，LiveKit 得你自己另外跑。

详细说明见 [单二进制部署](/deploy/binary)。

## 只想要个容器

不想用 compose，就一个 `docker run`：

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

但这样里面**没有 LiveKit**，`LIVEKIT_HOST` 必须指向一个外部可用的 LiveKit 实例。

详见 [单容器部署](/deploy/docker)。

## 上公网前检查一遍

- [ ] `JWT_KEY` 不是默认值
- [ ] `LIVEKIT_KEY` / `LIVEKIT_SECRET` 不是默认值
- [ ] `STORAGE_ENCRYPT_KEY` 已设置
- [ ] 管理员密码改掉了（默认 `admin123`）
- [ ] 数据库密码改掉了
- [ ] HTTPS 配好了（有域名时 `GOSPEAK_DOMAIN` 已填且档位含 `edge`）
- [ ] 已有反代时档位**不含** `edge`
- [ ] 媒体端口在防火墙上放行了（7882 tcp/udp、3478/udp）
- [ ] 有定时备份
- [ ] `CORS_ORIGIN` 设成了你的域名，不是 `*`

## 下一步

- [Docker Compose 详解](/deploy/compose) — 每个档位讲清楚
- [反代与 HTTPS](/deploy/nginx) — 内置 Caddy 与已有反代
- [监控](/deploy/monitoring) — Prometheus + Grafana
- [故障排查](/deploy/troubleshooting) — 出问题先看这篇
