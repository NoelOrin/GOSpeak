# 故障排查

本页按症状组织，每一条均说明对应的判断方法与处理方式。

## 页面无法访问

**确认容器运行状态**

```bash
docker compose ps
```

存在 `Exited` 或 `Restarting` 状态的容器时，查看对应日志：

```bash
docker compose logs --tail=100 gospeak
```

**容器运行正常但页面仍无法访问**

```bash
curl -v http://localhost:8998/ping
curl -v http://localhost/ping
```

第一条命令连通而第二条不通，说明问题出在 Caddy / 反代；两条均不通，说明应用未启动或端口未映射。

**出现 `bind: address already in use` 报错**

端口已被其他进程占用，查看占用进程：

```bash
sudo lsof -i :8998
```

结束占用进程，或修改 `SERVER_PORT`。

## 页面可访问但无法登录

**检查浏览器网络面板**

- 请求返回 404 → 前端与 API 不同源，或反代未将路径完整转发到 `8998`
- 请求返回 502 → 应用未启动
- 请求返回 200 但 `code` 非 0 → 后端业务错误，查看 `msg` 字段
- 请求被重定向到登录页 → 登录成功但 Cookie 未保存

**Cookie 无法保存**

该现象通常源于 HTTPS 配置。后端在 HTTPS 下会为 Cookie 附加 `Secure` 属性，若反代未传递 `X-Forwarded-Proto`，后端会按 HTTP 处理，两侧行为无法匹配。

使用内置 Caddy 档位不会出现此问题（`reverse_proxy` 默认传递该头）。使用自有反代时，检查配置中是否包含以下行：

```nginx
proxy_set_header X-Forwarded-Proto $scheme;
```

`COOKIE_SECURE=auto` 依赖该头判断，补上这一行即可解决；若确实不希望依赖该头，则显式设置 `COOKIE_SECURE=true`。

**持续提示用户名或密码错误**

默认管理员账号为 `admin` / `admin123`。若此前修改过密码且已遗忘，可进入容器直接修改数据库：

```bash
docker compose exec gospeak sh
# 或进入数据库容器
docker compose exec postgres psql -U gospeak -d gospeak
```

上述方式均无法恢复时，可删除数据库重新初始化（数据将丢失）。

## 语音功能不可用

这是最常遇到的问题，按以下顺序排查。

### 第一步：确认 LiveKit 运行状态

```bash
docker compose ps livekit
docker compose logs --tail=50 livekit
# 从应用容器内部探测（7880 仅绑定宿主机回环，容器内使用服务名）
docker compose exec gospeak wget -qO- http://livekit:7880
```

有响应说明服务正常运行。

### 第二步：确认密钥配置一致

密钥仅在 `deploy/.env` 中保存一份，由 compose 注入 LiveKit 容器，`livekit.yaml` 不含任何凭据。先确认两侧配置一致：

```bash
grep LIVEKIT_KEY deploy/.env
grep -E "LIVEKIT_KEYS|LIVEKIT_WEBHOOK_API_KEY" <(docker compose config)
```

若两侧不一致，重新执行 `./init.sh --force`。

### 第三步：确认媒体端口连通性

从外网环境测试（而非服务器本机）：

```bash
# TCP 端口
nc -zv 你的服务器IP 7882

# UDP 端口无法使用 nc 测试，只能确认防火墙规则
```

UDP 端口未开放是语音不可用最常见的原因。云服务器在系统防火墙之外还有一层安全组，**两层均需开放**。

需要开放的端口：

- `7882/udp` 媒体主通道
- `7882/tcp` 媒体兜底
- `3478/udp` TURN

**7880 / 7881 无需开放。** 7880 只绑定宿主机回环（`127.0.0.1:7880`）供本机调试，信令对外走应用同源的 `/livekit`；7881 不再发布。

### 第四步：确认 ICE 候选地址

检查 LiveKit 日志中的 `candidate` 相关信息：

```bash
docker compose logs livekit | grep -i candidate
```

云服务器存在多块网卡时，`use_external_ip: true` 可能探测到内网地址。此时需手动指定：

```yaml
rtc:
  use_external_ip: false
  node_ip: 你的公网IP
```

### 第五步：浏览器控制台

打开开发者工具，检查 Console 中的 WebRTC 相关报错。常见报错如下：

| 报错信息 | 可能原因 |
|------|------|
| `NotAllowedError` | 未授予麦克风权限 |
| `NotFoundError` | 没有可用的麦克风设备 |
| `mixed content` | HTTPS 页面连接了 `ws://` 地址（反代未传递 `X-Forwarded-Proto`） |
| `ICE connection failed` | 媒体端口不通 |

同时确认 `serverUrl` 推导正确：查看 Network 面板中 `/api/v1/signal/token` 的响应，正常值应为 `wss://<你的域名>/livekit`。

## 上传文件失败

**413 错误**

原因通常是反代的请求体上限过小。内置 Caddy 默认不限制请求体，因此 `edge` 档位一般不会遇到该问题；若自行在 Caddyfile 中添加了 `request_body` 限制，将其调大即可：

```
request_body {
    max_size 100MB
}
```

使用自有 Nginx 时调大 `client_max_body_size`（默认值仅为 1m）：

```nginx
client_max_body_size 100m;
```

**500 错误且日志中出现 `storage` 字样**

存储配置存在问题。进入「管理 → 存储」页面执行「测试配置」，查看具体报错。

本机磁盘模式下，检查目录权限：

```bash
docker compose exec gospeak ls -la /app/uploads
```

**文件上传成功但图片不显示**

`STORAGE_PUBLIC_BASE_URL` 配置错误，或反代未将 `/uploads/` 转发到 `8998`。

## 数据库问题

**启动时报连接失败**

```bash
docker compose exec postgres pg_isready -U gospeak
```

若检查不通，查看 postgres 容器日志。常见原因为 `DB_PASSWORD` 与 `POSTGRES_PASSWORD` 不一致。

**SQLite 报 `database is locked`**

并发写入量过大。启用 WAL 模式：

```ini
DB_WAL=true
```

若问题仍未解决，说明 SQLite 无法承载当前使用量，应改用 PostgreSQL。

**表结构不对 / 缺字段**

GORM 自动迁移在启动时执行。检查启动日志中的 migration 相关错误；有时原因是数据库用户缺少建表权限。

## 性能问题

**CPU 持续偏高**

先定位占用来源：

```bash
docker stats
```

- `gospeak` 高 → 检查是否日志输出过多（`LOG_LEVEL=debug` 时尤其严重）
- `livekit` 高 → 音频转发负载大，房间人数过多或需升级服务器
- `postgres` 高 → 慢查询，需要加索引

**内存持续增长**

Go 程序的内存不会立即归还系统，增长到一定程度后会趋于稳定。若内存持续线性增长且不回落，则可能存在内存泄漏，需抓取 profile 分析。

**语音有回声**

该现象源于客户端而非服务器：外放与麦克风会互相拾音，使用耳机可避免。

## 磁盘空间不足

```bash
df -h
docker system df
```

清理方式：

```bash
# 清理无用的镜像和构建缓存
docker system prune -a

# 清理日志（未配置轮转时）
docker compose down
sudo truncate -s 0 /var/lib/docker/containers/*/*-json.log
docker compose up -d
```

长期方案为配置 Docker 日志轮转。

## 收集诊断信息

提交 issue 前先收集以下信息：

```bash
# 版本
cat VERSION

# 容器状态
docker compose ps

# 应用日志
docker compose logs --tail=200 gospeak > gospeak.log

# 配置（需抹除密钥）
grep -vE "KEY|SECRET|PASSWORD" deploy/.env

# 健康检查
curl -s http://localhost:8998/ping
curl -s http://localhost:8998/readyz
```

## 仍未解决时

- 查看 [常见问题](/guide/faq) 是否有类似条目
- 在 GitHub 提交 issue，附上上述收集的信息
- 提交前检索已有的 issue

## 下一步

- [监控](/deploy/monitoring) — 提前发现问题
- [反代与 HTTPS](/deploy/nginx) — 反代相关的问题
