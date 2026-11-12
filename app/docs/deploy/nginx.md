# 反代与 HTTPS

生产环境需要一个 HTTPS 入口：**浏览器只在 HTTPS（或 localhost）下授予麦克风权限**，没有 HTTPS 时语音功能不可用。

这项配置比以往简单得多：整个应用只有一个上游 `gospeak:8998`，`/api`、`/ws`、`/uploads`、`/livekit` 全在它里面。**LiveKit 不需要第二个域名、不需要单独证书、也不需要单独的反代规则。**

## 只有一个上游

```
浏览器 ──HTTPS/WSS──► Caddy / 用户侧反代 ──► gospeak:8998 ──► 内部 LiveKit :7880
                                                    └─ /livekit（同源反代，WebSocket 自动升级）

浏览器 ──WebRTC(UDP)──────────────────────► LiveKit :7882 ── 媒体直连，不经反代
```

- **控制面**（页面、API、信令）全部走一个上游。
- `/livekit` 是应用内置的同源反代：请求进 `gospeak:8998`，由应用转发到内部 LiveKit 的 7880，WebSocket 升级自动处理。因此反代只需要处理普通 HTTP 请求，**无需为 LiveKit 编写任何 location**。
- **媒体面**（音频）走 WebRTC UDP 直连 LiveKit 的 7882，不经过反代，也不会被反代影响。媒体端口要在防火墙上直接对公网开放。

## 首选：内置 Caddy 档位

`deploy/` 内置了 Caddy 配置（`deploy/Caddyfile`），无需单独安装或编写规则。在 `deploy/.env` 里：

```ini
COMPOSE_PROFILES=app,livekit,edge
GOSPEAK_DOMAIN=voice.example.com
```

然后：

```bash
docker compose up -d --build
```

Caddy 会自动完成以下工作：

- 申请并续期 Let's Encrypt 证书
- 把 HTTP 重定向到 HTTPS
- 为 `/ws`、`/livekit` 完成 WebSocket 升级（`reverse_proxy` 默认行为，无需额外指令）
- 启用 zstd / gzip 压缩

`deploy/Caddyfile` 的全部内容就是：

```
{$GOSPEAK_DOMAIN::80} {
	encode zstd gzip
	reverse_proxy gospeak:8998
}
```

`{$GOSPEAK_DOMAIN::80}` 的含义是「取 `GOSPEAK_DOMAIN`，为空时退化为 `:80`」。因此：

- **填入域名** → 自动启用 HTTPS
- **留空** → 只监听 80，纯 HTTP，适用于局域网或前面已有反代的场景，不申请证书也不跳转

前置条件只有两项：域名 A 记录指向本机公网 IP，防火墙放行 80/443。

## 已有反代：指向同一个上游

当入口已经是 Nginx、Traefik、Dokploy、宝塔之类的组件时，**不开启 `edge` 档位**，由现有反代指向 `gospeak:8998`：

```ini
# deploy/.env
COMPOSE_PROFILES=app,livekit
GOSPEAK_DOMAIN=
GOSPEAK_PORT=8998
```

反代侧只需要做三件事：

1. 转发到 `http://<宿主机>:8998`
2. 保留 `Host` 与 `X-Forwarded-Proto` 请求头（应用靠 `X-Forwarded-Proto` 判断推导 `ws://` 还是 `wss://`）
3. 为 WebSocket 放行升级（`/ws` 与 `/livekit` 都是长连接）

不需要按路径拆分 location，也不需要为 LiveKit 单独配置。以下是一个最小可用的 Nginx 片段（`upstream` 里的地址按实际部署调整：Nginx 与应用在同一 Docker 网络时使用服务名 `gospeak:8998`，装在宿主机上时使用 `127.0.0.1:8998`）：

```nginx
upstream gospeak_backend {
    server gospeak:8998;    # 宿主机上的 Nginx 改成 127.0.0.1:8998
    keepalive 32;
}

map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80;
    server_name voice.example.com;
    client_max_body_size 20m;

    location / {
        proxy_pass http://gospeak_backend;
        proxy_http_version 1.1;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade           $http_upgrade;
        proxy_set_header Connection        $connection_upgrade;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
        proxy_buffering off;
    }
}
```

所有路径都走同一个 `location /`，由应用自行区分 `/api`、`/ws`、`/uploads`、`/livekit`。HTTPS 使用 certbot 或云厂商证书终结在 Nginx 上：

```bash
apt install certbot python3-certbot-nginx
certbot --nginx -d voice.example.com
```

### 片段中的关键配置

**`proxy_http_version 1.1` + `Upgrade` 头**

缺少这两行时 WebSocket 握手失败，表现为页面可以打开但一直显示「已断开」，语音也无法连接。

**`proxy_buffering off`**

开启缓冲会放大信令消息的延迟，语音状态更新不及时。

**`proxy_read_timeout 86400s`**

默认的 60 秒会中断长连接。语音保持挂机状态时连接处于静默，60 秒后即被断开。

**`X-Forwarded-Proto`**

应用依赖该头推导 `serverUrl` 的协议。缺失时，HTTPS 页面下可能推导出 `ws://`，进而被浏览器按混合内容拦截。

**`client_max_body_size`**

Nginx 默认值只有 1m，上传大文件时报 413。按需要调整到 20m 或更大。

## 配置 HTTPS 之后

**无需修改 `LIVEKIT_HOST`，也无需填写 `LIVEKIT_PUBLIC_URL`。**

早期版本要求把地址改为 `wss://`，现已不再需要。`LIVEKIT_HOST` 保持容器内网地址 `ws://livekit:7880`，`LIVEKIT_PUBLIC_URL` 保持留空，应用按请求 Host 自动推导出：

```
https://voice.example.com  →  serverUrl = wss://voice.example.com/livekit
```

反代把 `/livekit` 当作普通路径转发到 `gospeak:8998`，应用内部再转到 LiveKit 的 7880，WebSocket 升级由应用和 Caddy/Nginx 各自处理。

## 常见问题

**页面能开，语音连不上**

排查两个方向：其一，媒体端口 7882/udp、7882/tcp、3478/udp 是否已在防火墙（含云安全组）放行；其二，浏览器 Network 面板中 `/api/v1/signal/token` 返回的 `serverUrl` 是否为 `wss://<你的域名>/livekit`。`serverUrl` 为 `ws://` 而页面是 HTTPS 时，说明反代未传递 `X-Forwarded-Proto`。

**一直显示「已断开」**

`/ws` 的 WebSocket 升级未配置。内置 Caddy 档位不会出现此问题；自有反代需检查 `Upgrade` 与 `Connection` 头。

**语音挂一会儿就断**

`proxy_read_timeout` 为默认值 60 秒，改为 86400。

**上传大文件报 413**

`client_max_body_size` 取值过小。

**登录后一直跳回登录页**

`X-Forwarded-Proto` 未传递，后端据此判定请求为 HTTP，Cookie 的 `Secure` 属性随之不匹配。`COOKIE_SECURE=auto` 正是读取该头，补上该头即可；不依赖该头时，显式设置 `COOKIE_SECURE=true`。

**证书申请失败**

确认域名 A 记录已指向本机、80 端口从公网可达——Let's Encrypt 的 HTTP-01 校验需要访问 80。反代或 CDN 位于 Caddy 前面时，避免两边同时占用 80/443。

## 下一步

- [部署到服务器](/deploy/) — 整体流程
- [故障排查](/deploy/troubleshooting) — 按症状查
