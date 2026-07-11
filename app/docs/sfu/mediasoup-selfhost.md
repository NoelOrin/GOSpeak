# MediaSoup 自建部署

MediaSoup 是一个底层的 WebRTC SFU 库，以 Node.js 模块形式提供。GOSpeak 通过一个 Bridge HTTP Worker 与之通信。

## 架构

```
GOSpeak (Go)
    │  HTTP POST /room/create, /produce, /consume ...
    ▼
MediaSoup Bridge (Node.js)    ← app/mediasoup-worker/
    │
    ▼
MediaSoup Worker (C++ 子进程)
    │
    ▼
WebRTC 媒体 (UDP)
```

## 启动 Worker

```bash
cd app/mediasoup-worker
pnpm install
pnpm start    # 默认监听 3012 端口
```

### Docker 部署

```yaml
mediasoup-worker:
  build:
    context: ..
    dockerfile: deploy/mediasoup-worker/Dockerfile
  ports:
    - "3012:3012"              # HTTP Bridge
    - "40000-40100:40000-40100/udp"  # WebRTC 媒体
  environment:
    PORT: 3012
    RTC_MIN_PORT: 40000
    RTC_MAX_PORT: 40100
    LISTEN_IP: 0.0.0.0
    ANNOUNCED_IP: ${MEDIASOUP_ANNOUNCED_IP:-127.0.0.1}
```

```bash
docker compose -f deploy/docker-compose.yml \
  --profile mediasoup --profile app up -d --build
```

## 环境变量配置

```env
SFU_PROVIDER=mediasoup
MEDIASOUP_BRIDGE_URL=http://localhost:3012
MEDIASOUP_HOST=localhost:3012
```

## 特点

| 特性 | 说明 |
|------|------|
| 灵活性 | 最高。完全控制编码、转发策略、 simulcast/SVC |
| 复杂度 | 需要自己实现信令逻辑（GOSpeak 已封装）|
| 性能 | 优秀。纯 C++ 核心，Node.js 管理面 |
| 协议 | 自定义 JSON 协议 over HTTP |
| 功能覆盖 | 基础 Token/Room API 可用，高级管理（踢人/禁言）需额外实现 |

## 注意事项

1. MediaSoup 需要大量 UDP 端口（默认 40000-40100），防火墙需放行
2. `MEDIASOUP_ANNOUNCED_IP` 须设为客户端可达的 IP
3. 相比 SRS/LiveKit，MediaSoup 的 Provider 功能覆盖较少
4. 适合对 WebRTC 有深入理解、需要高度定制的场景
