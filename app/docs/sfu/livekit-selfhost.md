# LiveKit 自建部署

LiveKit 是目前 GOSpeak 支持度最高的 SFU 后端。自建部署可获得完整功能：房间管理、Token 生成、踢人、禁言、Webhook 回调等。

## Docker Compose 部署（推荐）

### 配置文件

`deploy/livekit/livekit.yaml`：

```yaml
port: 7880
rtc:
  tcp_port: 7882
  udp_port: 7882
  use_external_ip: false
  stun_servers:
    - stun.l.google.com:19302

keys:
  API7ar5gCTyVkfY: xtpW2PGUKPugyVzEvX4OaPaHtNGm5bRUmrIXe6NMIqb

turn:
  enabled: true
  udp_port: 3478
```

### docker-compose 服务

```yaml
livekit:
  image: livekit/livekit-server:latest
  ports:
    - "7880:7880"    # HTTP API
    - "7881:7881"    # WebRTC
    - "7882:7882"    # WebRTC TCP
    - "7882:7882/udp"
    - "3478:3478/udp" # TURN
  volumes:
    - ./livekit/livekit.yaml:/etc/livekit.yaml:ro
  command: --config /etc/livekit.yaml
```

### 启动

```bash
# 完整栈：LiveKit + GOSpeak 应用
docker compose -f deploy/docker-compose.yml \
  --profile livekit --profile app up -d --build
```

## 使用 Docker 官方镜像

```bash
docker run -d \
  --name livekit \
  -p 7880:7880 \
  -p 7881:7881 \
  -p 7882:7882 \
  -p 7882:7882/udp \
  -p 3478:3478/udp \    # TURN 中继
  -v $(pwd)/livekit.yaml:/etc/livekit.yaml \
  livekit/livekit-server:latest \
  --config /etc/livekit.yaml
```

## 生成 API Key

LiveKit 使用预共享密钥对（在配置文件中声明）：

```yaml
keys:
  my-api-key: my-api-secret
```

生成随机密钥：

```bash
openssl rand -hex 32
```

## 环境变量配置

```env
SFU_PROVIDER=livekit
LIVEKIT_HOST=ws://localhost:7880       # 自建 HTTP
# LIVEKIT_HOST=wss://livekit.example.com  # 生产 HTTPS 走 WSS
LIVEKIT_KEY=API7ar5gCTyVkfY
LIVEKIT_SECRET=xtpW2PGUKPugyVzEvX4OaPaHtNGm5bRUmrIXe6NMIqb
```

> 生产环境：`LIVEKIT_HOST` 使用 `wss://` 前缀，且 LiveKit 服务器需要 TLS 证书（可通过 Nginx 反代或 LiveKit 自带 TLS）。

## 完整功能

| 功能 | 支持状态 |
|------|---------|
| 生成房间 Token | ✅ |
| 踢出参与者 | ✅ |
| 禁言/取消禁言 | ✅（服务端强制）|
| 列出房间/参与者 | ✅ |
| 删除房间 | ✅ |
| Webhook 回调 | ✅ |
| TURN 中继 | ✅ |

## 生产注意事项

1. **密钥安全**：生产环境修改默认 API Key/Secret
2. **Redis 依赖**：GOSpeak 应用不依赖 Redis；LiveKit 单节点无需 Redis。多节点 LiveKit 集群如需共享状态，请按 LiveKit 文档自行配置
3. **Webhook**：LiveKit 支持事件回调，GOSpeak 通过 `POST /api/v1/signal/webhook` 接收
4. **TURN**：部署在 NAT 后时，开启 TURN 中继（`turn.enabled: true`）帮助 ICE 连通
5. **TLS**：生产环境为 LiveKit 配置 TLS 证书，或通过 Nginx 反代 `7880` 端口
6. **端口放行**：7880(API)、7881-7882(WebRTC)、3478/udp(TURN)
