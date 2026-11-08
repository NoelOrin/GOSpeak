# 部署到服务器

这篇讲把 GOSpeak 放到一台服务器上跑起来。假设你有一台 Linux 机器，能 SSH 进去，装了 Docker。

## 先想清楚三件事

部署前先回答这三个问题，后面的选择都跟着它走：

**1. 多少人用？**
- 10 人以内：单机 + SQLite，够用
- 几十人：单机 + PostgreSQL，或者把数据库挪出去
- 上百人：得考虑 LiveKit 单独部署、对象存储、可能多实例

**2. 语音要自建还是用云服务？**
自建 LiveKit 省钱但要自己运维；LiveKit Cloud 不用管基础设施但按量计费。详见 [语音服务](/sfu/)。

**3. 有没有域名和 HTTPS？**
浏览器只在 HTTPS（或 localhost）下允许用麦克风。**没有 HTTPS，语音功能直接不可用。** 这是硬要求。

## 推荐路线：Docker Compose

一条命令拉起整套服务，包括应用、LiveKit、Nginx。

### 第一步：拿代码

```bash
git clone https://github.com/NoelOrin/GOSpeak.git
cd GOSpeak/deploy
```

### 第二步：生成配置

```bash
./init.sh
```

这个脚本会：

- 生成 `deploy/.env`
- 随机生成 JWT 密钥、LiveKit 密钥、存储加密密钥
- 探测服务器公网 IP，填进 LiveKit 的 ICE candidate 配置
- 把生成的 LiveKit 密钥同步写进 `livekit/livekit.yaml`

密钥两边必须一致，脚本帮你对齐了，别手动改乱。

### 第三步：启动

```bash
docker compose up -d --build
```

第一次会构建镜像，几分钟。之后重启很快。

### 第四步：验证

```bash
curl -s http://localhost:8998/ping
```

有返回就说明应用起来了。浏览器打开 `http://服务器IP` 应该能看到界面。

### 第五步：配 HTTPS

这一步不能省。用 Caddy 最省事，它自动申请证书：

```
voice.example.com {
    reverse_proxy localhost:80
}
```

Nginx 的方案见 [Nginx 配置](/deploy/nginx)。

配好之后把 `deploy/.env` 里的地址改成 HTTPS：

```ini
LIVEKIT_HOST=wss://voice.example.com
```

然后 `docker compose up -d` 重启。

## 档位怎么选

`deploy/.env` 里的 `COMPOSE_PROFILES` 决定起哪些组件。改完这一行，之后命令都是裸 `docker compose`，不用每次加 `--profile`。

| 你的情况 | 填什么 |
|---------|--------|
| 小团队，先跑起来 | `livekit,app` |
| 要 PostgreSQL | `postgres,livekit,app` |
| 还要对象存储 | `postgres,livekit,rustfs,app` |
| 还要监控面板 | `postgres,livekit,rustfs,observability,app` |

从 SQLite 换 PostgreSQL 时，除了改档位，还要改数据库那几行：

```ini
DB_TYPE=PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=gospeak
DB_PASSWORD=<换成强密码>
```

**注意**：换数据库不会自动搬数据。SQLite 里已有的数据得手动导。

## 端口要开哪些

| 端口 | 服务 | 要不要开公网 |
|------|------|-------------|
| 80 / 443 | Nginx | 必须 |
| 7880 | LiveKit 信令 | 必须（走 WSS） |
| 7881 | LiveKit 媒体（TCP） | 必须 |
| 7882/udp | LiveKit 媒体（UDP） | 必须 |
| 3478/udp | TURN 中继 | 建议开 |
| 8998 | 应用 | 不用，Nginx 转发就行 |
| 5432 | PostgreSQL | 绝对不要 |

防火墙示例（Ubuntu）：

```bash
ufw allow ssh
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 7880/tcp
ufw allow 7881/tcp
ufw allow 7882/udp
ufw allow 3478/udp
```

**媒体端口没开是语音不通最常见的原因。** 页面能打开、能登录、能进频道，但就是听不到声音——八成是这个。

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
docker run --rm -v gospeak-db:/data -v $(pwd):/backup alpine \
  tar czf /backup/db-$(date +%F).tar.gz -C /data .
```

**PostgreSQL**：

```bash
docker compose exec -T postgres pg_dump -U gospeak gospeak | gzip > backup-$(date +%F).sql.gz
```

**上传的文件**：`/app/uploads` 或者你的对象存储桶。

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
- [ ] HTTPS 配好了
- [ ] 媒体端口在防火墙上放行了
- [ ] 有定时备份
- [ ] `CORS_ORIGIN` 设成了你的域名，不是 `*`

## 下一步

- [Docker Compose 详解](/deploy/compose) — 每个档位讲清楚
- [Nginx 配置](/deploy/nginx) — 反代和 TLS
- [监控](/deploy/monitoring) — Prometheus + Grafana
- [故障排查](/deploy/troubleshooting) — 出问题先看这篇
