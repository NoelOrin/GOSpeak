# Nginx 与 HTTPS

## 为什么需要反向代理

应用自己就能提供页面和接口，但直接暴露 8998 有几个问题：

- 没有 HTTPS，浏览器不给麦克风权限
- 没有统一入口，端口号暴露在地址栏
- 静态资源没有缓存头

所以生产环境前面挂一个 Nginx（或者 Caddy）。

## 关键点：媒体不走 Nginx

这是最容易搞错的地方。LiveKit 的音频走 WebRTC，是 UDP 直连，**不经过 Nginx**。

```
浏览器 ──HTTPS/WSS──► Nginx ──► 应用 :8998      控制面
浏览器 ──WebRTC─────► LiveKit :7881/:7882       媒体面（直连）
```

所以 Nginx 只需要转发 HTTP 和 WebSocket，不用碰媒体端口。媒体端口要在防火墙上直接对公网开放。

## 最小可用配置

```nginx
upstream gospeak_backend {
    server gospeak:8998;
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

    location = /ping {
        proxy_pass http://gospeak_backend;
        access_log off;
    }

    # 信令 WebSocket：必须升级连接，且不能缓冲
    location /ws {
        proxy_pass http://gospeak_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
        proxy_buffering off;
    }

    location /api/ {
        proxy_pass http://gospeak_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 静态资源长缓存
    location ~* \.(?:js|css|woff2?|ttf|png|jpe?g|gif|svg|ico|webp)$ {
        proxy_pass http://gospeak_backend;
        add_header Cache-Control "public, max-age=604800, immutable";
        access_log off;
    }

    # SPA 兜底，必须放最后
    location / {
        proxy_pass http://gospeak_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

仓库里 `deploy/nginx-docker.conf` 就是这个的完整版，直接用也行。

### 几处不能省

**`proxy_http_version 1.1` + `Upgrade` 头**
少了这两行，WebSocket 握手失败，表现是页面能开但一直显示「已断开」。

**`proxy_buffering off` 在 `/ws`**
开着缓冲会让消息延迟变大，语音状态更新不及时。

**`proxy_read_timeout 86400s`**
默认 60 秒会把长连接掐掉。语音挂着不动的时候连接是静默的，60 秒后就被踢了。

**`location /` 放最后**
Nginx 的前缀匹配按配置里出现的顺序，`/` 写前面会把所有请求都吃掉。

## 加 HTTPS

### 方案一：Let's Encrypt + Nginx

```bash
apt install certbot python3-certbot-nginx
certbot --nginx -d voice.example.com
```

certbot 会自动改你的 Nginx 配置加 SSL 段，并设置续期定时任务。

手动配的话大概长这样：

```nginx
server {
    listen 443 ssl http2;
    server_name voice.example.com;

    ssl_certificate     /etc/nginx/ssl/fullchain.pem;
    ssl_certificate_key /etc/nginx/ssl/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    # ... 上面那些 location ...
}

server {
    listen 80;
    return 301 https://$host$request_uri;
}
```

### 方案二：Caddy（更省事）

Caddy 会自动申请和续期证书，配置就三行：

```
voice.example.com {
    reverse_proxy localhost:80
}
```

适合不想折腾证书的场景。

## 配完 HTTPS 要改的地方

`deploy/.env` 里的 LiveKit 地址必须改成 `wss://`：

```ini
LIVEKIT_HOST=wss://voice.example.com
```

因为页面是 HTTPS 的，浏览器不允许它连 `ws://` 的地址（混合内容拦截）。表现是页面能开、能登录，但语音连不上。

## LiveKit 的 TLS 怎么处理

两种方式：

**方式一：LiveKit 自己带证书**

在 `livekit/livekit.yaml` 里配：

```yaml
cert_file: /etc/livekit/cert.pem
key_file: /etc/livekit/key.pem
```

**方式二：Nginx 反代 7880**

```nginx
location /livekit/ {
    proxy_pass http://livekit:7880/;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
}
```

然后 `LIVEKIT_HOST=wss://voice.example.com/livekit`。

注意：这只解决**信令**的 TLS，媒体还是 UDP 直连，端口照样要开。

## 常见问题

**页面能开，语音连不上**
九成是这两个原因之一：媒体端口 7881/7882/3478 没在防火墙放行；或者 HTTPS 页面配了 `ws://` 的 LiveKit 地址被浏览器拦了。

**一直显示「已断开」**
`/ws` 的 `Upgrade` 头没配。检查 Nginx 配置里 `location /ws` 那段。

**语音挂一会儿就断**
`proxy_read_timeout` 用的默认 60 秒。改成 86400。

**上传大文件报 413**
`client_max_body_size` 太小。默认 20m，需要传更大的附件就调高。

**登录后一直跳回登录页**
`X-Forwarded-Proto` 没传，后端以为请求是 HTTP 的，Cookie 的 Secure 属性对不上。检查 API location 里有没有这行。

## 下一步

- [部署到服务器](/deploy/) — 整体流程
- [故障排查](/deploy/troubleshooting) — 按症状查
