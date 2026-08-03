# SFU 音视频引擎

GOSpeak 通过 **Provider 抽象层**支持多种 SFU（Selective Forwarding Unit）后端，运行时一键切换，无需修改代码。

## 支持的 SFU

| Provider | 类型 | 部署方式 | 成熟度 | 推荐场景 |
|----------|------|---------|--------|---------|
| **LiveKit** | 自建 / 云 | Docker / LiveKit Cloud | ⭐⭐⭐⭐⭐ | 全功能、最成熟 |
| **SRS** | 自建 | Docker | ⭐⭐⭐⭐ | 国产开源、高性能 |
| **Agora** | 云服务 | SDK 集成 | ⭐⭐⭐ | 不想管服务器 |
| **Cloudflare** | 云服务 | WHIP/WHEP + REST | ⭐⭐⭐ | 全球边缘节点 |

## Provider 接口

所有 SFU 后端实现同一个 Go 接口：

```go
type Provider interface {
    GenerateToken(room, identity string) (string, error)
    GenerateAdminToken() (string, error)
    ListRooms() ([]RoomSummary, error)
    ListParticipants(room string) ([]ParticipantSummary, error)
    MuteParticipant(room, identity, trackSid string, muted bool) error
    RemoveParticipant(room, identity string) error
    DeleteRoom(room string) error
    GetHost() string
}
```

流式 SFU（如 SRS）额外实现 `StreamProvider`，支持 WHIP/WHEP 协议。

## 架构示意

```
浏览器
  ├─ HTTP/WSS ──► Go Server ──► SFU Provider 抽象层
  │                                ├── LiveKit (自建/云)
  │                                ├── SRS (自建)
  │                                ├── Agora (云)
  │                                └── Cloudflare (云)
  └─ WebRTC ──► SFU 服务器 ──► 其他用户
```

## 配置方式

### 运行时切换

仅改环境变量 `SFU_PROVIDER` 及其对应配置：

```bash
# SRS
SFU_PROVIDER="srs"
SRS_HOST="localhost"
SRS_SECRET="..."

# LiveKit
SFU_PROVIDER="livekit"
LIVEKIT_HOST="ws://localhost:7880"
LIVEKIT_KEY="..."
LIVEKIT_SECRET="..."
```

### 持久化配置

通过 API `/api/v1/sfu/config` 可运行时更新 SFU 配置并持久化到数据库：

```
POST /api/v1/sfu/update-config
Content-Type: application/json
Authorization: Bearer <admin-token>

{
  "provider": "srs",
  "config": { ... }
}
```

## 自建 vs 云服务对比

| 方面 | 自建 (SRS/LiveKit) | 云 (Agora/Cloudflare) |
|------|--------------------|-----------------------|
| 数据控制 | ✅ 完全自主 | ❌ 经过第三方 |
| 费用 | 仅服务器成本 | 按分钟/带宽计费 |
| 维护 | 需自己运维 | 零运维 |
| 扩展 | 受限于服务器 | 弹性伸缩 |
| 延迟 | 取决于部署位置 | 全球节点 |
| 公网 NAT | 需配 STUN/TURN | 内置穿透 |

## 选择指南

| 你的场景 | 推荐选项 |
|----------|---------|
| 游戏语音、完全自控 | **LiveKit**（完整功能）或 **SRS**（轻量）|
| 快速搭建、功能全面 | **LiveKit** Docker |
| 不想管基础设施 | **Agora**（国内）或 **Cloudflare**（海外）|
| 本地开发测试 | **LiveKit** 或 **SRS** Docker |

## 各 SFU 配置文档

- [LiveKit 自建 →](/sfu/livekit-selfhost)
- [LiveKit 云服务 →](/sfu/livekit-cloud)
- [SRS 自建 →](/sfu/srs-selfhost)
- [Agora 云服务 →](/sfu/agora-cloud)
- [Cloudflare 云服务 →](/sfu/cloudflare)
- [成熟度对比 →](/sfu/comparison)
