# 故障排查

按症状查。每一条都写清楚「怎么判断」和「怎么修」。

## 页面打不开

**先确认容器活着**

```bash
docker compose ps
```

有容器是 `Exited` 或 `Restarting` 状态，就看它的日志：

```bash
docker compose logs --tail=100 gospeak
```

**容器都在但打不开**

```bash
curl -v http://localhost:8998/ping
curl -v http://localhost/ping
```

第一条通、第二条不通 → Caddy / 反代的问题。两条都不通 → 应用没起来或端口没映射。

**报 `bind: address already in use`**

端口被占了：

```bash
sudo lsof -i :8998
```

杀掉占用的进程，或者改 `SERVER_PORT`。

## 能打开页面，但登录不了

**看浏览器网络面板**

- 请求 404 → 前端和 API 不同源，或者反代没把路径完整转发到 `8998`
- 请求 502 → 应用没起来
- 请求 200 但返回 `code` 非 0 → 后端业务错误，看 `msg` 字段
- 请求被重定向到登录页 → 登录成功但 Cookie 没存下来

**Cookie 存不下来**

八成是 HTTPS 配置问题。后端在 HTTPS 下会给 Cookie 加 `Secure` 属性，如果反代没传 `X-Forwarded-Proto`，后端以为还是 HTTP，行为就对不上。

用内置 Caddy 档位不会有这个问题（`reverse_proxy` 默认会传）。用自有反代就检查有没有这行：

```nginx
proxy_set_header X-Forwarded-Proto $scheme;
```

`COOKIE_SECURE=auto` 会读这个头判断，所以补上这一行就够了；确实不想依赖该头时再显式设 `COOKIE_SECURE=true`。

**一直提示用户名或密码错误**

默认管理员是 `admin` / `admin123`。如果之前改过密码又忘了，可以进容器直接改数据库：

```bash
docker compose exec gospeak sh
# 或者进数据库容器
docker compose exec postgres psql -U gospeak -d gospeak
```

实在不行删掉数据库重来（会丢数据）。

## 语音功能不通

这是最常遇到的问题，按下面顺序查。

### 第一步：LiveKit 活着吗

```bash
docker compose ps livekit
docker compose logs --tail=50 livekit
# 从应用容器内部探（7880 只绑宿主机回环，容器内用服务名）
docker compose exec gospeak wget -qO- http://livekit:7880
```

有响应说明活着。

### 第二步：密钥对得上吗

密钥只在 `deploy/.env` 里有一份，由 compose 注入 LiveKit 容器，`livekit.yaml` 不含任何凭据。先确认两边都对上了：

```bash
grep LIVEKIT_KEY deploy/.env
grep -E "LIVEKIT_KEYS|LIVEKIT_WEBHOOK_API_KEY" <(docker compose config)
```

不一致的话，重新跑 `./init.sh --force`。

### 第三步：媒体端口通吗

从外网测试（不是服务器上）：

```bash
# TCP 端口
nc -zv 你的服务器IP 7882

# UDP 端口没法用 nc 测，只能确认防火墙规则
```

UDP 端口没开是语音不通最常见的原因。云服务器除了系统防火墙，还有一层安全组——**两层都要开**。

需要开的：

- `7882/udp` 媒体主通道
- `7882/tcp` 媒体兜底
- `3478/udp` TURN

**7880 / 7881 不需要开。** 7880 只绑宿主机回环（`127.0.0.1:7880`）供本机调试，信令对外走应用同源的 `/livekit`；7881 不再发布。

### 第四步：ICE 候选地址对不对

看 LiveKit 日志里有没有 `candidate` 相关信息：

```bash
docker compose logs livekit | grep -i candidate
```

云服务器有多块网卡时，`use_external_ip: true` 可能探到内网地址。这时手动指定：

```yaml
rtc:
  use_external_ip: false
  node_ip: 你的公网IP
```

### 第五步：浏览器控制台

打开开发者工具，看 Console 有没有 WebRTC 相关的报错。常见的：

| 报错 | 原因 |
|------|------|
| `NotAllowedError` | 没给麦克风权限 |
| `NotFoundError` | 没有可用的麦克风设备 |
| `mixed content` | HTTPS 页面连了 `ws://` 的地址（反代没传 `X-Forwarded-Proto`） |
| `ICE connection failed` | 媒体端口不通 |

顺带确认一下 `serverUrl` 推导是否正确：看 Network 面板里 `/api/v1/signal/token` 的响应，正常应该是 `wss://<你的域名>/livekit`。

## 上传文件失败

**413 错误**

反代的请求体上限太小。内置 Caddy 默认不限制请求体，所以走 `edge` 档位一般不会遇到；如果你自己给 Caddyfile 加了 `request_body` 限制，调大它即可：

```
request_body {
    max_size 100MB
}
```

用自有 Nginx 时调大 `client_max_body_size`（默认只有 1m）：

```nginx
client_max_body_size 100m;
```

**500 错误，日志里有 `storage` 字样**

存储配置有问题。进「管理 → 存储」点「测试配置」，看具体报错。

本机磁盘模式的话，检查目录权限：

```bash
docker compose exec gospeak ls -la /app/uploads
```

**传上去了但图片不显示**

`STORAGE_PUBLIC_BASE_URL` 配错了，或者反代没把 `/uploads/` 转发到 `8998`。

## 数据库问题

**启动时报连接失败**

```bash
docker compose exec postgres pg_isready -U gospeak
```

不通的话看 postgres 容器日志。常见原因：`DB_PASSWORD` 和 `POSTGRES_PASSWORD` 不一致。

**SQLite 报 `database is locked`**

并发写太多了。开 WAL 模式：

```ini
DB_WAL=true
```

还是不行的话，说明 SQLite 撑不住你的使用量了，换 PostgreSQL。

**表结构不对 / 缺字段**

GORM 自动迁移在启动时跑。看启动日志里有没有 migration 相关的错误。有时候是数据库用户没有建表权限。

## 性能问题

**CPU 一直很高**

先看是谁吃掉的：

```bash
docker stats
```

- `gospeak` 高 → 看是不是日志打太多（`LOG_LEVEL=debug` 时尤其严重）
- `livekit` 高 → 音频转发负载大，人太多或者该升级服务器
- `postgres` 高 → 慢查询，需要加索引

**内存一直涨**

Go 程序的内存不会立刻还给系统，涨到一定程度会稳定。如果一直线性增长不回落，可能是内存泄漏，抓个 profile 看看。

**语音有回声**

不是服务器问题，是客户端问题：外放 + 麦克风会互相拾音。戴耳机。

## 磁盘满了

```bash
df -h
docker system df
```

清理：

```bash
# 清理无用的镜像和构建缓存
docker system prune -a

# 清理日志（如果没配轮转）
docker compose down
sudo truncate -s 0 /var/lib/docker/containers/*/*-json.log
docker compose up -d
```

长期方案：配 Docker 日志轮转。

## 收集诊断信息

提 issue 之前，先收集这些：

```bash
# 版本
cat VERSION

# 容器状态
docker compose ps

# 应用日志
docker compose logs --tail=200 gospeak > gospeak.log

# 配置（记得抹掉密钥）
grep -vE "KEY|SECRET|PASSWORD" deploy/.env

# 健康检查
curl -s http://localhost:8998/ping
curl -s http://localhost:8998/readyz
```

## 还是解决不了

- 看 [常见问题](/guide/faq) 有没有类似的
- 在 GitHub 提 issue，带上上面收集的信息
- 提之前搜一下已有的 issue

## 下一步

- [监控](/deploy/monitoring) — 提前发现问题
- [反代与 HTTPS](/deploy/nginx) — 反代相关的问题
