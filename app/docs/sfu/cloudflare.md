# Cloudflare Realtime SFU 云服务

[Cloudflare Realtime](https://developers.cloudflare.com/realtime/) 是 Cloudflare 的全球边缘 WebRTC 平台，基于 WHIP/WHEP 协议提供音视频传输。GOSpeak 将其作为 SFU Provider 之一集成。

## 创建 Cloudflare 项目

1. 注册 [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. 进入 **Realtime** 应用
3. 创建应用，获取 **App ID** 和 **App Secret**

## 环境变量配置

```env
SFU_PROVIDER=cloudflare
CF_APP_ID=your-app-id
CF_APP_SECRET=your-app-secret
CF_STUN_URL=stun.cloudflare.com:3478
```

## 架构说明

Cloudflare 模式下，GOSpeak 通过 REST API 管理房间和 Token，媒体流通过 WHIP/WHEP 在浏览器和 Cloudflare 全球边缘节点之间直连：

```
浏览器
  ├─ HTTP/WSS ──► GOSpeak ──► Cloudflare REST API
  │                              └─ room / session / token
  ├─ WHIP/WHEP ──► Cloudflare Edge ──► 其他用户
  └─ WebRTC ──► Cloudflare Edge (全球节点)
```

## 特点

| 特性 | 说明 |
|------|------|
| 部署 | 零运维。Cloudflare 管理全球节点 |
| 延迟 | 全球 300+ 节点，就近接入 |
| 协议 | WHIP/WHEP 标准协议 |
| 计费 | 按分钟计费 |
| 穿透 | 内置 STUN/TURN，无需额外配置 |

## 功能覆盖

| 功能 | 支持状态 |
|------|---------|
| 生成会话 Token | ✅（返回 JSON 配置块：sessionId / appId / stunUrl）|
| 管理员 Token | ✅ |
| 列出房间 | ⚠️ 仅进程内内存（进程重启丢失）|
| 列出参与者 | ⚠️ 仅进程内内存 |
| 踢出参与者 | ✅ |
| 禁言 | ❌ Cloudflare 无服务端轨道静音 API |
| 删除房间/会话 | ✅ |
| WHIP/WHEP 推拉流 | ✅ |

## 限制

- **无原生 Token 体系**：`GenerateToken` 返回 JSON 配置块（sessionId/appId/stunUrl），非真实鉴权 Token
- **ListRooms / ListParticipants** 仅返回进程内缓存，非跨实例权威
- **Mute** 不支持（前端停推兜底）
- 适合全球部署、不想管理 SFU 服务器的场景
