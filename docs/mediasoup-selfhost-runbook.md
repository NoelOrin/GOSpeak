# MediaSoup 自部署端到端 Runbook

注意: mediasoup-worker 是独立 Node 进程,Go 后端经 HTTP bridge 与之通信。浏览器 WebRTC 媒体直连 worker 的 RTC 端口(40000-49999/udp),不经 Go 后端。`ANNOUNCED_IP` 必须为浏览器可达地址 — dev 单机用 127.0.0.1,LAN 部署用宿主内网 IP,否则 ICE candidate 不可达。

dev 环境(浏览器与 docker 同宿主)。LAN 部署见末节。

信令(WS `sfu:*` 事件)走现有 WS 连接,与 LiveKit/SRS 共用同一 WS 通道,无额外代理。

## 1. 起 mediasoup-worker

```bash
# 方式 A: docker(取消 deploy/docker-compose.example.yml mediasoup-worker 注释块后)
docker compose -f deploy/docker-compose.example.yml up -d mediasoup-worker
curl -s http://localhost:3012/health   # 期望 {"ok":true,...}

# 方式 B: 本地 node(开发调试,默认端口 3012;若需 3001 显式设 PORT=3001)
cd packages/mediasoup-worker
pnpm install
ANNOUNCED_IP=127.0.0.1 LISTEN_IP=0.0.0.0 PORT=3012 pnpm start
```

> 本地 node 启动若不设 `PORT` 环境变量,默认监听 3012(与 docker 一致)。旧文档偶有 3001 属历史值,以 `index.ts` 中 `PORT || 3012` 为准。

## 2. 后端切 mediasoup

编辑 `app/server/.env.dev`:
- 注释 `SFU_PROVIDER="livekit"` 行
- 设 `SFU_PROVIDER="mediasoup"`
- 确认 `MEDIASOUP_BRIDGE_URL="http://localhost:3012"`(默认值已对,端口 3012)

启动:
```bash
pnpm dev:server
```

## 3. 前端切 mediasoup

新建 `app/web/.env.local`(已 gitignore):
```
VITE_SFU_PROVIDER=mediasoup
```

启动:
```bash
pnpm dev:web
```

## 4. 双向音频验证

1. 浏览器 A 开 `http://localhost:<vite端口>/room/<room-id>`,授权麦克风,加入。
2. 浏览器 B(不同 profile 或机)同房间加入。
3. A 发声 → B 听到;B 发声 → A 听到。
4. A 离开(关 tab)→ B 侧 `onRemoteAudioTrackRemovedCb` 触发,A 音轨停止播放,AudioContext 释放。
5. 服务端 mute 验证:调 `POST /api/v1/.../mute` mute A → B 听不到 A;unmute 恢复。
6. active speaker:仅 A 发声 → B 侧 `onActiveSpeakersCb(["A"])`;无人发声 → `[]`。

## 5. LAN 部署

- worker `ANNOUNCED_IP` 设宿主内网 IP(如 192.168.1.10)。
- 宿主防火墙放行 3012/tcp + 40000-49999/udp。
- 浏览器与 worker 不同机时,RTC udp 必须可达 ANNOUNCED_IP。

## 6. mac UDP 异常注意

mac 上 docker 的 udp 转发在某些 Docker Desktop 版本表现异常(RTP 丢包/无音频)。若遇:
- 优先用方式 B(本地 node worker),绕过 docker udp。
- 或升级 Docker Desktop 至最新版。
- 排查:浏览器 chrome://webrtc-internals 看 ICE candidate 连通性。
