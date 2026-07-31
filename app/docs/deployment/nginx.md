# Nginx 配置

## 生产配置

完整的 Nginx 生产配置见 `deploy/nginx-docker.conf`（Docker 内使用）或 `deploy/nginx.conf`（宿主机使用）。

## Docker Compose 中的 Nginx

SRS 模式必须使用 Nginx 实现 WHIP/WHEP 同源反代：

```nginx
# 控制面 + SRS WHIP/WHEP 同源反代
# WebRTC 媒体 :8000/udp|tcp 由 SRS 直接暴露

upstream gospeak_backend {
    server gospeak:8998;
    keepalive 32;
}

upstream gospeak_srs_http_api {
    server srs:1985;
    keepalive 16;
}

map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80;
    server_name _;
    client_max_body_size 20m;

    # 健康检查
    location = /ping {
        proxy_pass http://gospeak_backend;
        access_log off;
    }

    # WebSocket（WebSocket）
    location /ws {
        proxy_pass http://gospeak_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
        proxy_buffering off;
    }

    # API
    location /api/ {
        proxy_pass http://gospeak_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # SRS WHIP/WHEP 信令同源反代（⚠️ 必须写在 location / 之前）
    location /rtc/v1/ {
        proxy_pass http://gospeak_srs_http_api;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_request_buffering off;
        proxy_read_timeout 120s;
        proxy_send_timeout 120s;
    }

    # 静态资源缓存
    location ~* \.(?:js|css|woff2?|ttf|png|jpe?g|gif|svg|ico|webp)$ {
        proxy_pass http://gospeak_backend;
        add_header Cache-Control "public, max-age=604800, immutable";
        access_log off;
    }

    # SPA 兜底（必须放在最后）
    location / {
        proxy_pass http://gospeak_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 安全头
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
}
```

## 关键规则说明

### WebSocket 代理

```nginx
location /ws {
    proxy_pass http://gospeak_backend;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
    proxy_read_timeout 86400s;  # 长连接超时
    proxy_buffering off;        # 实时性要求
}
```

### WHIP/WHEP 反代

```nginx
location /rtc/v1/ {
    proxy_pass http://gospeak_srs_http_api;
    proxy_buffering off;           # 媒体信令不能缓冲
    proxy_request_buffering off;   # POST body 流式发送
}
```

### Location 顺序

```nginx
# 正确顺序（nginx - 精确匹配 > 前缀匹配按文件中的顺序）
location = /ping { ... }     # 1. 精确匹配
location /ws { ... } # 2. WebSocket
location /api/ { ... }       # 3. API
location /rtc/v1/ { ... }    # 4. SRS 信令（关键）
location /swagger/ { ... }   # 5. Swagger
location ~* \.(css|js) { ... } # 6. 静态资源
location / { ... }           # 7. SPA 兜底（最后）
```

> **⚠️ SRS 模式**：`location /rtc/v1/` **必须**写在 `location /` 之前，否则 WHIP 请求会被 SPA 的 `index.html` 吃掉，返回 404 前端页面。

## HTTPS 配置（Nginx）

```nginx
server {
    listen 443 ssl http2;
    server_name gospeak.example.com;

    ssl_certificate     /etc/nginx/ssl/fullchain.pem;
    ssl_certificate_key /etc/nginx/ssl/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    # ... 同上 location 配置 ...
}

server {
    listen 80;
    return 301 https://$host$request_uri;
}
```

## LiveKit 模式（无 Nginx）

使用 LiveKit 时，GOSpeak 直接提供 SPA 静态文件和 API，不需要 Nginx 反代：

```bash
# 直接暴露 GOSpeak :8998，LiveKit 在 :7880
docker compose --profile livekit --profile redis --profile app up -d
```

前端代码中 `LIVEKIT_HOST` 指向 LiveKit 地址，不走 Nginx 反代。
