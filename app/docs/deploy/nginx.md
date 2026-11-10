# 反代与 HTTPS

生产环境需要一个 HTTPS 入口，原因很实在：**浏览器只在 HTTPS（或 localhost）下授予麦克风权限**。没有 HTTPS，语音功能直接不可用。

好消息是这件事比过去简单得多：整个应用只有一个上游 `gospeak:8998`，`/api`、`/ws`、`/uploads`、`/livekit` 全在它里面。**LiveKit 不需要第二个域名、不需要单独证书、也不需要单独的反代规则。**

## 一条规则：只有一个上游

```
浏览器 ──HTTPS/WSS──► Caddy / 你的反代 ──► gospeak:8998 ──► 内部 LiveKit :7880
                                                    └─ /livekit（同源反代，WebSocket 自动升级）

浏览器 ──WebRTC(UDP)──────────────────────► LiveKit :7882 ── 媒体直连，不经反代
```

- **控制面**（页面、API、信令）全部走一个上游。
- `/livekit` 是应用内置的同源反代：请求进 `gospeak:8998`，由应用转发到内部 LiveKit 的 7880，WebSocket 升级自动处理。所以反代只需要看到普通 HTTP 请求，**不用为 LiveKit 写任何 location**。
- **媒体面**（音频）走 WebRTC UDP 直连 LiveKit 的 7882，不经过反代，也不会被反代影响。媒体端口要在防火墙上直接对公网开放。

## 首选：内置 Caddy 档位

`deploy/` 里已经内置了 Caddy 配置（`deploy/Caddyfile`），不用自己装、不用自己写规则。在 `deploy/.env` 里：

```ini
COMPOSE_PROFILES=app,livekit,edge
GOSPEAK_DOMAIN=voice.example.com
```

然后：

```bash
docker compose up -d --build
```

Caddy 会自动：

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

`{$GOSPEAK_DOMAIN::80}` 的意思是「取 `GOSPEAK_DOMAIN`，为空时退化为 `:80`」。所以：

- **填了域名** → 自动 HTTPS
- **留空** → 只监听 80，纯 HTTP，供局域网或「前面已有反代」的场景使用，不申请证书也不跳转

前置条件只有两个：域名 A 记录指向本机公网 IP，防火墙放行 80/443。

## 已有反代：指向一个上游就行

如果你已经有 Nginx、Traefik、Dokploy、宝塔之类的入口，**不要开 `edge` 档位**，让现有反代指向 `gospeak:8998` 即可：

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

不需要按路径拆 location，也不需要给 LiveKit 单独配置。下面是一个最小可用的 Nginx 片段（`upstream` 里的地址按你的实际部署改：Nginx 也在同一 Docker 网络里就用服务名 `gospeak:8998`，装在宿主机上就用 `127.0.0.1:8998`）：

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

所有路径都走同一个 `location /`，靠应用自己区分 `/api`、`/ws`、`/uploads`、`/livekit`。HTTPS 用 certbot 或云厂商证书终结在 Nginx 上即可：

```bash
apt install certbot python3-certbot-nginx
certbot --nginx -d voice.example.com
```

### 片段里几处不能省

**`proxy_http_version 1.1` + `Upgrade` 头**

少了这两行，WebSocket 握手失败，表现是页面能开但一直显示「已断开」，语音也连不上。

**`proxy_buffering off`**

开着缓冲会让信令消息延迟变大，语音状态更新不及时。

**`proxy_read_timeout 86400s`**

默认 60 秒会把长连接掐掉。语音挂着不动的时候连接是静默的，60 秒后就被踢了。

**`X-Forwarded-Proto`**

应用靠它推导 `serverUrl` 的协议。不传的话 HTTPS 页面下可能推导出 `ws://`，被浏览器按混合内容拦掉。

**`client_max_body_size`**

Nginx 默认只有 1m，上传大文件会报 413。按需要调到 20m 或更大。

## 配好 HTTPS 之后

**不需要改 `LIVEKIT_HOST`，也不需要填 `LIVEKIT_PUBLIC_URL`。**

以前这里要求把地址改成 `wss://`，现在不需要了。`LIVEKIT_HOST` 保持容器内网地址 `ws://livekit:7880`，`LIVEKIT_PUBLIC_URL` 保持留空，应用会按请求 Host 自动推导出：

```
https://voice.example.com  →  serverUrl = wss://voice.example.com/livekit
```

反代把 `/livekit` 当作普通路径转发到 `gospeak:8998`，应用内部再转到 LiveKit 的 7880，WebSocket 升级由应用和 Caddy/Nginx 各自处理。

## 常见问题

**页面能开，语音连不上**

先查两个方向：媒体端口 7882/udp、7882/tcp、3478/udp 有没有在防火墙（含云安全组）放行；再看浏览器 Network 面板里 `/api/v1/signal/token` 返回的 `serverUrl` 是不是 `wss://<你的域名>/livekit`。如果 `serverUrl` 是 `ws://` 而页面是 HTTPS，说明反代没传 `X-Forwarded-Proto`。

**一直显示「已断开」**

`/ws` 的 WebSocket 升级没配。用内置 Caddy 档位不会出现这个问题；用自有反代就检查 `Upgrade` 与 `Connection` 头。

**语音挂一会儿就断**

`proxy_read_timeout` 用的默认 60 秒。改成 86400。

**上传大文件报 413**

`client_max_body_size` 太小。

**登录后一直跳回登录页**

`X-Forwarded-Proto` 没传，后端以为请求是 HTTP 的，Cookie 的 `Secure` 属性对不上。`COOKIE_SECURE=auto` 正是读这个头，补上即可；确实不想依赖该头时再显式设 `COOKIE_SECURE=true`。

**证书申请失败**

确认域名 A 记录已经指向本机、80 端口从公网可达。Let's Encrypt 的 HTTP-01 校验需要访问 80。反代或 CDN 挡在 Caddy 前面时，别让两边同时抢 80/443。

## 下一步

- [部署到服务器](/deploy/) — 整体流程
- [故障排查](/deploy/troubleshooting) — 按症状查
