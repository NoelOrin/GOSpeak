# SRS 自建部署

SRS（Simple Realtime Server）是国产开源的高性能流媒体服务器，支持 WHIP/WHEP WebRTC 推拉流协议。在 GOSpeak 中通过 WHIP/WHEP 实现语音通信。

## 架构说明

SRS 模式使用**同源反代**架构：

```
浏览器
  ├─ /api /ws / ──► Nginx ──► GOSpeak:8998
  ├─ /rtc/v1/*          ──► Nginx ──► SRS:1985  (WHIP/WHEP HTTP 信令)
  └─ WebRTC UDP/TCP     ──► SRS:8000 (媒体直连)
```

- **信令 HTTP**（WHIP/WHEP）可经 Nginx 反代 → 同源访问
- **媒体流 UDP/TCP** 必须直连 SRS 8000 端口，不能经过 Nginx

## Docker Compose 部署

### 最小部署

```bash
cd deploy
cp .env.example .env
cp env/app.srs.env.example env/app.srs.env
# 编辑 env/app.srs.env: SRS_SECRET 改为随机值

docker compose --profile srs --profile app up -d --build
```

### SRS 配置

SRS 启用了 WebRTC 和 HTTP Callback：

```conf
rtc_server {
    enabled on;
    listen 8000;
    candidate $CANDIDATE;  # 由环境变量 SRS_CANDIDATE 注入
}

vhost __defaultVhost__ {
    rtc { enabled on; }

    http_hooks {
        on_publish   http://gospeak:8998/api/v1/srs/callback;
        on_unpublish http://gospeak:8998/api/v1/srs/callback;
        on_play      http://gospeak:8998/api/v1/srs/callback;
        on_stop      http://gospeak:8998/api/v1/srs/callback;
    }
}
```

### Nginx 配置要点

Nginx 负责把 `/rtc/v1/` 转发到 SRS 的 HTTP API，同时托管前端静态资源和后端 API：

```nginx
# WHIP/WHEP 同源反代（必须写在 location / 之前）
location /rtc/v1/ {
    proxy_pass http://srs:1985;
    proxy_buffering off;
    proxy_request_buffering off;
}

# 静态资源和 API 反代
location / {
    proxy_pass http://gospeak:8998;
}
```

## 手动部署（非 Docker）

```bash
# 1. 下载 SRS
git clone -b develop https://github.com/ossrs/srs.git
cd srs/trunk
./configure --rtc
make

# 2. 配置 srs.conf
cat > conf/gospeak.conf << 'EOF'
listen              1935;
max_connections     1000;
daemon              off;
http_api {
    enabled on;
    listen 1985;
}
rtc_server {
    enabled on;
    listen 8000;
    candidate YOUR_SERVER_IP;
}
vhost __defaultVhost__ {
    rtc { enabled on; }
    http_hooks {
        on_publish   http://YOUR_GOSPEAK_HOST:8098/api/v1/srs/callback;
        on_unpublish http://YOUR_GOSPEAK_HOST:8098/api/v1/srs/callback;
    }
}
EOF

# 3. 启动 SRS
./objs/srs -c conf/gospeak.conf
```

## 环境变量配置

```env
SFU_PROVIDER=srs
SRS_HOST=localhost       # Go → SRS 管理 API 地址（Docker 内填 srs）
SRS_API_PORT=1985        # SRS API 端口
SRS_WHIP_URL=/rtc/v1/whip/  # WHIP 端点路径
SRS_SECRET=your-secret   # ⚠️ 必填！HMAC 密钥
SRS_PUBLIC_HOST=http://localhost  # 浏览器侧 serverUrl 前缀
```

### 关键配置说明

| 变量 | 说明 | 生产推荐值 |
|------|------|-----------|
| `SRS_SECRET` | Token HMAC 密钥。**必填非空** | `openssl rand -hex 32` |
| `SRS_PUBLIC_HOST` | 浏览器收到的 serverUrl | `https://your.domain` |
| `SRS_CANDIDATE` | SRS ICE candidate（Docker 环境变量）| 公网 IP 或可达 LAN IP |
| `SRS_HOST` | Go 管理 SRS 的地址 | compose 内 `srs`，公网 `localhost` |

## WHIP/WHEP 协议

SRS 通过 **WHIP**（WebRTC-HTTP ingestion protocol）推流、**WHEP**（WebRTC-HTTP egress protocol）拉流。

```
用户 A 推流: POST /rtc/v1/whip/{stream}  →  SDP Offer/Answer
用户 B 拉流: GET  /rtc/v1/whep/{stream}   →  SDP Offer/Answer
媒体传输:    WebRTC UDP/TCP :8000
```

GOSpeak 的 SRS Provider 实现了 `StreamProvider` 接口：

```go
type StreamProvider interface {
    Provider
    StreamName(room, identity string) string
    StreamInfo(room, identity string) (stream, token string, err error)
}
```

## 故障排查

| 症状 | 排查方向 |
|------|---------|
| WHIP 404 返回 HTML | `/rtc/v1/` 被 SPA 兜底吃掉，检查 Nginx location 顺序 |
| WHIP 502/504 | Nginx → SRS:1985 不通，检查 compose 网络 |
| ICE failed / 无声 | `SRS_CANDIDATE` 不对，或 8000 端口未开放 |
| on_publish 403 | Backend 未启动 / `SRS_SECRET` 不一致 / callback 地址不可达 |
| `non ISO-8859-1 code point` | `SRS_SECRET` 为空 |
| 浏览器连 localhost:1985 生产挂 | 应走同源 `/rtc/v1` + `SRS_PUBLIC_HOST` |
| 页面加载后不动 | 检查 WHIP/WHEP endpoint 是否正确拼写 |

## 生产安全

1. SRS 默认不强制校验 WHIP Bearer，token 安全依赖网络边界 + callback 校验
2. `SRS_SECRET` 只放服务端 env，不进前端代码
3. SRS 管理 API `:1985` 生产不要对公网裸奔，只暴露经 Nginx 的 `/rtc/v1`
