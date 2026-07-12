# Provider 成熟度对比

## 功能覆盖矩阵

| 功能 | LiveKit | SRS | MediaSoup | Agora | Daily |
|------|---------|-----|-----------|-------|-------|
| `GenerateToken` | ✅ | ✅ (WHIP Bearer) | ✅ | ✅ | ✅ |
| `GenerateAdminToken` | ✅ | ✅ | ✅ | ❌ | ❌ |
| `ListRooms` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ListParticipants` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `MuteParticipant` | ✅ 服务端强制 | ❌ (前端自行停止) | ❌ | ❌ | ❌ |
| `RemoveParticipant` | ✅ | ✅ (KickParticipant) | ❌ | ❌ | ❌ |
| `DeleteRoom` | ✅ | ✅ | ✅ | ❌ | ❌ |
| `GetHost` | ✅ | ✅ | ✅ | ✅ | ✅ |
| Webhook | ✅ | ✅ (HTTP Hooks) | ❌ | ❌ | ❌ |

## 成熟度评级

| Provider | 评级 | 说明 |
|----------|------|------|
| **LiveKit** | ⭐⭐⭐⭐⭐ | 最完整。所有 Provider 接口均可工作，Webhook 集成完善 |
| **SRS** | ⭐⭐⭐⭐ | 核心功能完整。Mute 不支持服务端强制，但 WHIP/WHEP 协议设计干净 |
| **MediaSoup** | ⭐⭐⭐ | Token + 基础房间 API 可用，高级管理功能缺失 |
| **Agora** | ⭐⭐⭐ | Token 生成和基础查询工作，踢人/禁言未完整实现 |
| **Daily** | ⭐⭐⭐ | Token 和房间查询可用，管理功能有限 |

## 信令 vs SFU 分工

GOSpeak 对某些操作采用**信令层优先策略**（详见 `internal/signal/AGENTS.md`）：

| 操作 | 信令层 | SFU 层 |
|------|--------|--------|
| `RemoveParticipant` | ✅ 删除在线成员记录 + 广播 | ✅ LiveKit/SRS 才调 |
| `ListRooms` + `ListParticipants` | 失败时返回空数组 `[]` | ✅ 有则返回 |
| `Mute*` | `BroadcastMute` 广播 | ❌ 不调 |

## 选择建议

### 全功能需求（推荐）

```env
SFU_PROVIDER=livekit   # 踢人、禁言、Webhook 全套
```

### 国产/自建

```env
SFU_PROVIDER=srs   # 国产、高性能，GOSpeak 对 SRS 支持完善
```

### 不需要踢人/禁言

```env
SFU_PROVIDER=mediasoup  # 高度可定制
```

### 零运维

```env
SFU_PROVIDER=agora   # 国内用户
SFU_PROVIDER=daily   # 海外用户
```
