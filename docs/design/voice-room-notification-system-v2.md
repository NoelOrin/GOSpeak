# 语音房间通知系统 — LiveKit 原生方案

> **项目**: GoRTC  
> **日期**: 2026-05-26  
> **版本**: v2.0 (LiveKit Native)  
> **状态**: 设计阶段  
> **演进**: 从 v1.0 Socket.IO/SSE 方案演进至纯 LiveKit 原生方案

---

## 1. 概述

### 1.1 设计演进

| 版本 | 方案 | 连接方式 | 状态 |
|------|------|---------|------|
| v1.0 | Socket.IO / SSE | WebSocket / HTTP SSE | 已废弃 |
| **v2.0** | **LiveKit Data Channel** | **WebRTC 原生** | **当前方案** |

**演进原因**: LiveKit 已内置参与者状态同步机制，无需维护额外的 Socket.IO Hub 或 SSE 连接池。

### 1.2 核心思路

利用 LiveKit 的以下原生能力实现状态广播：

- **Track 事件**: `TrackMuted` / `TrackUnmuted` — 检测麦克风静音状态
- **Data Channel**: `publishData()` / `DataReceived` — 参与者间消息广播
- **Webhook**: `track_published` / `track_unpublished` — 后端被动感知

**结果**: 前端仅需维护 WebRTC 单一连接，后端仅需 Webhook 接收。

---

## 2. 总体架构

### 2.1 架构图

```
┌─────────────────────────────────────────────┐
│                LiveKit 房间                   │
│                                             │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐ │
│  │ 用户 A  │◄──►│  SFU   │◄──►│ 用户 B  │ │
│  │(WebRTC) │    │ Server  │    │(WebRTC) │ │
│  └────┬────┘    └────┬────┘    └────┬────┘ │
│       │              │              │      │
│       │  Data Channel│              │      │
│       │  (音频状态)   │              │      │
│       └──────────────►│◄─────────────┘      │
│                      │                      │
│       ◄──────────────┘                    │
│       (Webhook 通知后端)                   │
└─────────────────────────────────────────────┘
```

### 2.2 数据通路

| 通路 | 协议 | 方向 | 用途 |
|------|------|------|------|
| **主通路** | WebRTC Data Channel | P2P via SFU | 音频状态实时广播 |
| **辅助通路** | LiveKit Webhook | SFU → Go Backend | 后端业务感知、日志 |
| **信令通路** | WebSocket (Socket.IO) | Client ↔ Go Backend | 房间加入令牌、房间列表（已有，不动） |

**关键决策**: 通知广播不再走 Socket.IO，仅保留 Socket.IO 用于房间管理信令。

---

## 3. 方案一：Data Channel 广播（推荐）

LiveKit 内置 WebRTC Data Channel，支持参与者间可靠消息广播。

### 3.1 广播端（前端）

```typescript
// 当本地音频状态变化时广播给房间所有人
function broadcastAudioStatus(room: Room, isMuted: boolean) {
  const payload = {
    type: 'AUDIO_STATUS',
    identity: room.localParticipant.identity,
    isMuted,
    timestamp: Date.now(),
    sequence: seq++  // 单调递增，防乱序
  };

  // 通过 LiveKit Data Channel 可靠传输
  room.localParticipant.publishData(
    new TextEncoder().encode(JSON.stringify(payload)),
    DataPacket_Kind.RELIABLE
  );
}
```

### 3.2 订阅端（前端）

```typescript
// 监听本地麦克风变化 → 触发广播
room.on(RoomEvent.TrackMuted, (pub) => {
  if (pub.track?.source === TrackSource.MICROPHONE) {
    broadcastAudioStatus(room, true);
  }
});

room.on(RoomEvent.TrackUnmuted, (pub) => {
  if (pub.track?.source === TrackSource.MICROPHONE) {
    broadcastAudioStatus(room, false);
  }
});

// 接收房间内其他人的状态广播
room.on(RoomEvent.DataReceived, (payload, kind, participant) => {
  try {
    const msg = JSON.parse(new TextDecoder().decode(payload));

    switch (msg.type) {
      case 'AUDIO_STATUS':
        updateUserAudioIcon(msg.identity, msg.isMuted);
        break;
      case 'SPEAKING_STATUS':
        updateSpeakingIndicator(msg.identity, msg.speakers);
        break;
    }
  } catch (e) {
    // 忽略非 JSON 数据
  }
});
```

### 3.3 全量同步（新用户加入时）

```typescript
room.on(RoomEvent.Connected, () => {
  const micPub = room.localParticipant.getTrackPublication(
    TrackSource.MICROPHONE
  );

  // 加入时广播一次自己的当前状态
  broadcastAudioStatus(room, micPub?.isMuted ?? true);
});
```

---

## 4. 方案二：Participant Attributes（简化版）

LiveKit 支持参与者自定义属性，属性变化自动同步给房间所有人。

### 4.1 设置属性

```typescript
// 静音时更新属性
await room.localParticipant.setAttributes({
  audioMuted: 'true',
  audioLevel: '0',
  isSpeaking: 'false'
});

// 取消静音时更新属性
await room.localParticipant.setAttributes({
  audioMuted: 'false',
  audioLevel: '0.5',
  isSpeaking: 'true'
});
```

### 4.2 监听属性变化

```typescript
room.on(
  RoomEvent.ParticipantAttributesChanged,
  (changedAttributes, participant) => {

    if (changedAttributes.audioMuted !== undefined) {
      updateUserAudioIcon(
        participant.identity,
        changedAttributes.audioMuted === 'true'
      );
    }

    if (changedAttributes.isSpeaking !== undefined) {
      updateSpeakingIndicator(
        participant.identity,
        changedAttributes.isSpeaking === 'true'
      );
    }
  }
);
```

### 4.3 方案对比

| 维度 | Data Channel | Participant Attributes |
|------|-------------|----------------------|
| **实时性** | 极高（毫秒级） | 稍慢（百毫秒，SFU 同步） |
| **复杂度** | 需手动编解码 | 自动同步，API 简单 |
| **大小限制** | ~64KB/条 | 单个属性值较小 |
| **事件粒度** | 可批量发送多条 | 每次 set 触发一次全量 |
| **后端感知** | 需机器人参与者加入房间 | Webhook 不触发属性变化 |
| **适用场景** | 高频状态变更 | 低频元数据同步 |

**推荐**: 音频状态用 **Data Channel**（高频），用户元数据用 **Attributes**（低频）。

---

## 5. 后端闭环（Webhook）

前端通过 Data Channel 广播，后端通过 LiveKit Webhook 被动感知。

### 5.1 Webhook 配置

```bash
# LiveKit 服务器环境变量
LIVEKIT_WEBHOOK_URL=https://your-go-backend.com/api/v1/livekit/webhook
```

### 5.2 Go 后端接收端

```go
func (h *SignalHandler) LivekitWebhook(c *gin.Context) {
  event := new(livekit.WebhookEvent)
  if err := c.BindJSON(event); err != nil {
    pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
    return
  }

  switch event.Event {
  case "participant_joined":
    log.Printf("[Webhook] %s joined room %s", 
      event.Participant.Identity, 
      event.Room.Name)
    // 可触发业务逻辑：更新房间在线人数统计

  case "participant_left":
    log.Printf("[Webhook] %s left room %s", 
      event.Participant.Identity, 
      event.Room.Name)
    // 清理用户状态

  case "track_published":
    if event.Track.Source == livekit.TrackSource_MICROPHONE {
      log.Printf("[Webhook] %s unmuted", event.Participant.Identity)
      // 例如：记录用户发言时长
    }

  case "track_unpublished":
    if event.Track.Source == livekit.TrackSource_MICROPHONE {
      log.Printf("[Webhook] %s muted", event.Participant.Identity)
    }
  }

  pkg.Success(c, "ok")
}
```

### 5.3 时序图

```
用户 A 点击静音
    │
    ▼
[Client A] localTrack.setMuted(true)
    │
    ├─► LiveKit SDK 触发 TrackMuted
    │       ▼
    │   publishData({type: 'AUDIO_STATUS', isMuted: true})
    │       │
    │       ▼
    │   ┌─────────────┐
    └──►│  LiveKit SFU  │
        └──────┬───────┘
               │ 路由给房间内所有人
               ├─► [Client B] DataReceived → 更新 UI
               │
               └─► [Client C] DataReceived → 更新 UI
               │
               ▼
        Webhook: track_unpublished
               │
               ▼
        [Go Backend] 记录日志/统计
```

---

## 6. 前端封装 Hook

```typescript
// hooks/useLivekitAudioState.ts
import { useState, useEffect } from 'react';
import {
  Room, RoomEvent, TrackSource, DataPacket_Kind,
  type TrackPublication, type Participant
} from 'livekit-client';

interface AudioState {
  isMuted: boolean;
  isSpeaking: boolean;
  audioLevel: number;
  timestamp: number;
}

interface SpeakingPayload {
  type: 'SPEAKING_STATUS';
  speakers: string[];
  audioLevels: Record<string, number>;
}

interface AudioStatusPayload {
  type: 'AUDIO_STATUS';
  isMuted: boolean;
  sequence: number;
}

export function useLivekitAudioState(room: Room | undefined) {
  const [audioStates, setAudioStates] = useState<Map<string, AudioState>>(new Map());
  const [activeSpeakers, setActiveSpeakers] = useState<string[]>([]);
  const [participantCount, setParticipantCount] = useState(0);
  let seqCounter = 0;

  useEffect(() => {
    if (!room) return;

    // ─── 1. 本地状态变更监听 → 广播 ───
    const handleTrackMuted = (pub: TrackPublication) => {
      if (pub.track?.source === TrackSource.MICROPHONE) {
        broadcast({ type: 'AUDIO_STATUS', isMuted: true, sequence: seqCounter++ });
      }
    };

    const handleTrackUnmuted = (pub: TrackPublication) => {
      if (pub.track?.source === TrackSource.MICROPHONE) {
        broadcast({ type: 'AUDIO_STATUS', isMuted: false, sequence: seqCounter++ });
      }
    };

    // ─── 2. 接收远程状态广播 ───
    const handleDataReceived = (
      payload: Uint8Array,
      kind: DataPacket_Kind,
      participant?: Participant
    ) => {
      try {
        const msg = JSON.parse(new TextDecoder().decode(payload));

        if (msg.type === 'AUDIO_STATUS') {
          setAudioStates(prev => {
            const next = new Map(prev);
            const existing = next.get(msg.identity);
            // 序列号去重：忽略过期消息
            if (existing && (existing as any).sequence > msg.sequence) {
              return prev;
            }
            next.set(msg.identity, {
              isMuted: msg.isMuted,
              isSpeaking: existing?.isSpeaking ?? false,
              audioLevel: existing?.audioLevel ?? 0,
              timestamp: msg.timestamp
            });
            return next;
          });
        }

        if (msg.type === 'SPEAKING_STATUS') {
          setActiveSpeakers(msg.speakers);
          // 同时更新各说话者的音量等级
          setAudioStates(prev => {
            const next = new Map(prev);
            for (const [identity, level] of Object.entries(msg.audioLevels)) {
              const existing = next.get(identity);
              if (existing) {
                next.set(identity, { ...existing, audioLevel: level as number });
              }
            }
            return next;
          });
        }
      } catch {
        // 忽略非 JSON 数据
      }
    };

    // ─── 3. LiveKit 原生说话检测 ───
    const handleActiveSpeakers = (speakers: Participant[]) => {
      const speakerIds = speakers.map(s => s.identity);
      const audioLevels: Record<string, number> = {};
      speakers.forEach(s => {
        audioLevels[s.identity] = s.audioLevel;
      });

      setActiveSpeakers(speakerIds);

      // 可选：广播说话者列表给非 LiveKit 客户端
      broadcast({
        type: 'SPEAKING_STATUS',
        speakers: speakerIds,
        audioLevels
      });
    };

    // ─── 4. 房间人数变化 ───
    const handleParticipantConnected = () => {
      setParticipantCount(room.numParticipants);
    };
    const handleParticipantDisconnected = () => {
      setParticipantCount(room.numParticipants);
      // 清理离开用户的状态
      setAudioStates(prev => {
        const next = new Map(prev);
        // Note: 需要知道具体谁离开，可通过 participant 事件获取
        return next;
      });
    };

    // ─── 5. 加入时全量广播自己的状态 ───
    const handleConnected = () => {
      setParticipantCount(room.numParticipants);
      const micPub = room.localParticipant.getTrackPublication(TrackSource.MICROPHONE);
      broadcast({
        type: 'AUDIO_STATUS',
        isMuted: micPub?.isMuted ?? true,
        sequence: seqCounter++
      });
    };

    // 注册事件
    room.on(RoomEvent.TrackMuted, handleTrackMuted);
    room.on(RoomEvent.TrackUnmuted, handleTrackUnmuted);
    room.on(RoomEvent.DataReceived, handleDataReceived);
    room.on(RoomEvent.ActiveSpeakersChanged, handleActiveSpeakers);
    room.on(RoomEvent.ParticipantConnected, handleParticipantConnected);
    room.on(RoomEvent.ParticipantDisconnected, handleParticipantDisconnected);
    room.on(RoomEvent.Connected, handleConnected);

    // 如果已经连接，立即执行
    if (room.state === 'connected') {
      handleConnected();
    }

    return () => {
      room.off(RoomEvent.TrackMuted, handleTrackMuted);
      room.off(RoomEvent.TrackUnmuted, handleTrackUnmuted);
      room.off(RoomEvent.DataReceived, handleDataReceived);
      room.off(RoomEvent.ActiveSpeakersChanged, handleActiveSpeakers);
      room.off(RoomEvent.ParticipantConnected, handleParticipantConnected);
      room.off(RoomEvent.ParticipantDisconnected, handleParticipantDisconnected);
      room.off(RoomEvent.Connected, handleConnected);
    };
  }, [room]);

  // 广播封装
  const broadcast = (payload: object) => {
    if (!room) return;
    const msg = {
      ...payload,
      identity: room.localParticipant.identity,
      timestamp: Date.now()
    };
    room.localParticipant.publishData(
      new TextEncoder().encode(JSON.stringify(msg)),
      DataPacket_Kind.RELIABLE
    );
  };

  return { audioStates, activeSpeakers, participantCount };
}
```

---

## 7. 方案演进对比

| 维度 | v1.0 Socket.IO/SSE | **v2.0 LiveKit Native** |
|------|-------------------|------------------------|
| **前端连接数** | 2 (WebRTC + WebSocket) | **1 (仅 WebRTC)** |
| **后端复杂度** | 高（需维护广播 Hub） | **低（仅 Webhook 接收）** |
| **实时性** | 低（后端中转） | **极高（SFU 直接路由）** |
| **可靠性** | 需手动实现重连 | **LiveKit SDK 自动重连** |
| **人数统计** | 后端维护 | **前端通过 numParticipants** |
| **扩展性** | 需改造广播逻辑 | **直接复用 LiveKit 生态** |
| **调试难度** | 中等 | **低（标准 WebRTC 工具）** |
| **后端感知** | 实时参与广播 | **Webhook 异步感知** |

---

## 8. 接口变更

### 8.1 新增 API

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/livekit/webhook` | 接收 LiveKit Webhook 事件 |

### 8.2 废弃 API

| 方法 | 路径 | 原因 |
|------|------|------|
| ~~WS/SSE~~ | ~~`/api/v1/signal/events`~~ | 不再需要通过 Socket.IO/SSE 广播通知 |

### 8.3 保留 API（不变）

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/signal/token` | 获取 LiveKit 加入令牌 |
| GET | `/api/v1/signal/rooms` | 查询房间列表 |
| GET | `/api/v1/signal/participants` | 查询房间参与者（状态恢复兜底） |
| WS | `/socket.io/*` | Socket.IO 信令（用于非通知类信令） |

---

## 9. 风险评估

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|---------|
| Data Channel 消息丢失 | 低 | 中 | 使用 `RELIABLE` 模式，非 `LOSSY` |
| Webhook 延迟/丢失 | 中 | 低 | 前端状态为准，Webhook 仅用于日志 |
| 大量消息导致 SFU 压力 | 低 | 中 | 节流：说话检测 200ms 防抖 |
| 旧客户端不兼容 | 中 | 高 | 保留 `GET /api/v1/signal/participants` 兜底 |

---

## 10. 下一步行动

1. **前端**
   - 创建 `hooks/useLivekitAudioState.ts`
   - 更新 `<UserList />` / `<RoomHeader />` 组件绑定状态
   - 移除 Socket.IO 通知相关代码（如果有）

2. **后端**
   - 新增 `/api/v1/livekit/webhook` 端点
   - 配置 LiveKit 服务器 Webhook URL
   - 移除 `internal/notification/` 广播模块（如 v1.0 已实现）

3. **配置**
   - LiveKit 服务器配置 `LIVEKIT_WEBHOOK_URL`

4. **测试**
   - 多用户静音/取消静音并发测试
   - Webhook 失效时前端独立性测试
   - 网络抖动后自动恢复测试

---

*文档结束 — LiveKit Native Design*
