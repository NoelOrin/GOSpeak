# 语音房间通知系统设计文档

> **项目**: GoRTC  
> **日期**: 2026-05-26  
> **版本**: v1.0  
> **状态**: 设计阶段

---

## 1. 概述

### 1.1 背景

GoRTC 是基于 LiveKit 的实时语音通信系统。用户在语音房间内需要实时感知其他成员的状态变化，包括：

- 谁加入了房间 / 谁离开了房间
- 当前房间总人数
- 谁开启了麦克风 / 谁静音了
- 谁正在说话（基于音量检测）

### 1.2 设计目标

| 目标 | 说明 |
|------|------|
| **实时性** | 状态变更应在 500ms 内同步到房间内所有成员 |
| **低耦合** | 通知系统独立于音视频流传输，作为元数据通道 |
| **可扩展** | 支持未来新增屏幕共享、录制状态等通知类型 |
| **轻量级** | 仅传输状态变更，不携带音视频数据 |

### 1.3 设计约束

- **推送方式**: Socket.IO WebSocket（复用现有信令通道）
- **最大并发**: 单房间 <= 50 人（小中型语音房间）
- **持久化**: 不持久化通知历史，仅实时通知

---

## 2. 总体架构

### 2.1 架构图

```
┌─────────────────┐      ┌──────────────┐      ┌──────────────┐
│   React Web     │◄────►│  Go Backend  │◄────►│   LiveKit    │
│   Frontend      │  WS   │   (Gin)      │ API  │   Server     │
└─────────────────┘      └──────────────┘      └──────────────┘
        ▲                                              │
        └──────────── 数据广播流 ◄─────────────────────┘
```

### 2.2 数据流分离

系统同时运行两条并行数据流：

| 通道 | 协议 | 用途 |
|------|------|------|
| **实时音视频流** | WebRTC (LiveKit SDK) | 语音传输 |
| **通知/信令** | Socket.IO WebSocket | 状态广播、控制信令 |

---

## 3. 数据模型

### 3.1 通知事件类型

```typescript
type NotificationType =
  | 'ROOM_PARTICIPANT_JOINED'      // 有人加入房间
  | 'ROOM_PARTICIPANT_LEFT'        // 有人离开房间
  | 'ROOM_PARTICIPANT_COUNT'       // 当前房间总人数
  | 'AUDIO_INPUT_CHANGED'          // 麦克风状态变化
  | 'AUDIO_OUTPUT_CHANGED'         // 扬声器状态变化
  | 'SPEAKING_STATUS_CHANGED'      // 正在说话状态
```

### 3.2 统一通知消息结构

```typescript
interface RoomNotification {
  type: NotificationType;
  roomId: string;
  timestamp: number;
  sender: {
    userId: string;
    identity: string;
    displayName?: string;
  };
  payload: NotificationPayload;
}
```

### 3.3 各类通知 Payload

**人数通知**

```typescript
interface ParticipantCountPayload {
  totalParticipants: number;
  activeSpeakers: number;     // 正在说话的人数
  mutedCount: number;         // 静音人数
}
```

**音频状态通知**

```typescript
interface AudioStatusPayload {
  trackSid?: string;          // LiveKit track ID
  isMuted: boolean;           // 是否静音
  isSpeaking: boolean;        // 是否正在说话
  volumeLevel: number;        // 音量等级 0-1
  sequence: number;             // 单调递增序列号（防乱序）
  deviceType?: 'microphone' | 'system' | 'headphone';
}
```

**说话者列表通知**

```typescript
interface SpeakingChangedPayload {
  speakers: Array<{
    identity: string;
    audioLevel: number;
  }>;
}
```

---

## 4. 后端设计

### 4.1 模块结构

在现有分层架构中新增 `internal/notification/` 模块：

```
internal/
├── signal/                 # 现有信令模块（不动）
├── livekit/                # 现有 LiveKit 客户端（不动）
├── notification/           # 新增通知模块
│   ├── hub.go              # 通知分发中心（依赖 signal.Hub）
│   ├── observer.go         # LiveKit Webhook 处理器
│   ├── broadcaster.go      # 广播封装（房间隔离）
│   └── types.go            # 通知消息结构体定义
```

### 4.2 LiveKit 状态监听策略

采用 **Webhook + 轮询兜底** 的混合方案：

#### 方案 A: Webhook 实时接收（主要）

```
LiveKit Server ──► Go Backend Webhook Endpoint
  participant_joined
  participant_left
  track_published       ← 麦克风开启
  track_unpublished     ← 麦克风关闭
```

**优点**: 实时、无轮询开销  
**缺点**: 需要 LiveKit 配置 Webhook URL

#### 方案 B: 定时轮询（兜底/心跳）

```go
// 每隔 N 秒轮询，作为 Webhook 失效时的兜底
func (s *NotificationService) pollRoomState() {
    for {
        participants := livekitClient.ListParticipants(roomID)
        // 对比前后状态哈希，检测变化
        if stateHash != lastStateHash {
            broadcaster.BroadcastStateUpdate(roomID, participants)
        }
        time.Sleep(30 * time.Second)  // 心跳间隔
    }
}
```

**用途**: 
- Webhook 失效时的降级方案
- 定时全量同步，修复状态漂移

### 4.3 核心组件职责

#### observer.go — Webhook 入口

接收 LiveKit Webhook，转换为内部通知事件：

- `participant_joined` -> 广播 `ROOM_PARTICIPANT_JOINED` + 更新人数
- `participant_left` -> 广播 `ROOM_PARTICIPANT_LEFT` + 更新人数
- `track_published` (MICROPHONE) -> 广播 `AUDIO_INPUT_CHANGED` (isMuted=false)
- `track_unpublished` (MICROPHONE) -> 广播 `AUDIO_INPUT_CHANGED` (isMuted=true)

#### broadcaster.go — 广播逻辑

```go
// 房间内广播（可选排除发送者本人）
func (b *Broadcaster) BroadcastToRoom(
    roomID string, 
    excludeSocketID string, 
    msg Notification
) {
    b.signalHub.To(roomID).
        Except(excludeSocketID).
        Emit("room:notification", msg)
}

// 全量状态同步（新用户加入时推送）
func (b *Broadcaster) SendFullState(
    socketID string, 
    roomID string
) {
    participants := b.livekitClient.ListParticipants(roomID)
    state := buildFullState(participants)
    b.signalHub.To(socketID).Emit("room:state_sync", state)
}
```

---

## 5. 前端设计

### 5.1 状态管理架构

```
┌──────────────────────────────┐
│     useRoomNotification()    │  ← 统一 React Hook
│        (packages/web)        │
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│  NotificationStore (Zustand) │
│  - notifications[]           │
│  - participantCount          │
│  - activeSpeakers[]          │
│  - audioStates Map           │
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│   Socket.IO Event Listener   │
│   (复用现有 useSocket hook)  │
└──────────────────────────────┘
```

### 5.2 前端事件处理流程

```
新用户加入房间
    │
    ▼
[LiveKit] connect(room, token)
    │
    ▼
[Socket.IO] connect() + join room
    │
    ├─► 接收 `room:state_sync`（全量当前状态）
    │       ├─ 人数
    │       ├─ 每个人音频状态
    │       └─ 当前说话者列表
    │
    └─► 此后接收 `room:notification`（增量变更）
```

### 5.3 核心 Hook 设计

```typescript
function useRoomNotifications(room: Room) {
  const [participantCount, setParticipantCount] = useState(0);
  const [audioStates, setAudioStates] = useState<Map<string, AudioState>>(new Map());
  const [activeSpeakers, setActiveSpeakers] = useState<string[]>([]);

  useEffect(() => {
    const socket = getSocket();

    // 1. 全量同步（新用户加入时）
    socket.on('room:state_sync', (state: RoomState) => {
      setParticipantCount(state.participantCount);
      setAudioStates(new Map(state.audioStates));
      setActiveSpeakers(state.activeSpeakers);
    });

    // 2. 增量通知
    socket.on('room:notification', (notif: Notification) => {
      switch (notif.type) {
        case 'PARTICIPANT_JOINED':
          setParticipantCount(c => c + 1);
          break;
        case 'PARTICIPANT_LEFT':
          setParticipantCount(c => c - 1);
          setAudioStates(prev => {
            const next = new Map(prev);
            next.delete(notif.sender.identity);
            return next;
          });
          break;
        case 'AUDIO_CHANGED':
          setAudioStates(prev => {
            const next = new Map(prev);
            next.set(notif.sender.identity, notif.payload);
            return next;
          });
          break;
        case 'SPEAKING_CHANGED':
          setActiveSpeakers(notif.payload.speakers.map(s => s.identity));
          break;
      }
    });

    return () => {
      socket.off('room:state_sync');
      socket.off('room:notification');
    };
  }, [room.name]);

  // 3. 本地音频变更上报
  useEffect(() => {
    room.on(RoomEvent.LocalTrackPublished, (pub) => {
      if (pub.track.source === TrackSource.MICROPHONE) {
        socket.emit('audio:changed', {
          isMuted: pub.track.isMuted,
          isSpeaking: false,
          sequence: generateSeq()
        });
      }
    });

    room.on(RoomEvent.LocalTrackUnpublished, () => {
      socket.emit('audio:changed', { 
        isMuted: true, 
        isSpeaking: false,
        sequence: generateSeq()
      });
    });
  }, [room]);

  return { participantCount, audioStates, activeSpeakers };
}
```

### 5.4 UI 组件映射

| 状态 | UI 组件 | 展示方式 |
|------|---------|---------|
| 人数 | `<RoomHeader />` | 顶部角标 `👥 12` |
| 麦克风开启 | `<UserListItem />` | 麦克风图标高亮 |
| 麦克风静音 | `<UserListItem />` | 麦克风图标灰色 + 斜杠 |
| 正在说话 | `<UserListItem />` | 头像边框绿色脉冲动画 |
| 音量等级 | `<VolumeBar />` | 动态高度柱状图 |
| 用户加入 | `<NotificationToast />` | 底部弹出 "Alice 加入了房间" |
| 用户离开 | `<NotificationToast />` | 底部弹出 "Bob 离开了房间" |

---

## 6. 关键实现细节

### 6.1 说话检测（Speaking Detection）

**前端主导方案**（推荐，减少后端压力）：

```typescript
// 订阅 LiveKit 音量事件
room.on(RoomEvent.ActiveSpeakersChanged, (speakers) => {
  // 节流：仅当说话者列表变化时才上报，间隔 200ms
  const currentIds = speakers.map(s => s.identity).sort().join(',');
  if (lastSpeakerIds !== currentIds) {
    socket.emit('speaking:changed', { 
      speakers: speakers.map(s => ({
        identity: s.identity,
        audioLevel: s.audioLevel
      }))
    });
    lastSpeakerIds = currentIds;
  }
});
```

**为什么前端主导？**
- LiveKit SDK 已提供本地音量分析
- 避免后端处理大量音频数据
- 减少网络传输（仅传输身份列表，不传输音频采样）

### 6.2 状态竞态与乱序处理

**问题**: 用户快速切换静音，消息乱序到达。

**方案**: 乐观更新 + 序列号去重

```typescript
// 本地立即更新 UI（乐观）
setIsMuted(true);

// 上报后端，携带序列号
const seq = generateSeq();
socket.emit('audio:changed', { isMuted: true, seq });

// 收到广播时检查序列号
socket.on('room:notification', (notif) => {
  const existing = audioStates.get(notif.sender.identity);
  if (existing && existing.sequence > notif.payload.sequence) {
    return; // 忽略过期消息
  }
  // 应用更新
  setAudioStates(prev => new Map(prev).set(
    notif.sender.identity, 
    notif.payload
  ));
});
```

### 6.3 人数统计策略

| 场景 | 触发方式 | 广播内容 |
|------|---------|---------|
| 新用户加入 | LiveKit Webhook `participant_joined` | 更新后人数 + 新用户信息 |
| 用户离开 | LiveKit Webhook `participant_left` | 更新后人数 + 离开用户身份 |
| 页面刷新 | 前端主动请求 | `GET /api/v1/signal/participants` |
| 心跳兜底 | 后端每 30s | 全量人数广播 |

---

## 7. 时序图

### 7.1 用户加入房间

```
┌────────┐    ┌──────────┐    ┌──────────┐    ┌────────┐
│ Client │    │ LiveKit  │    │ Go Backend│    │ Others │
└───┬────┘    └────┬─────┘    └────┬─────┘    └───┬────┘
    │              │               │              │
    │ connect()    │               │              │
    │─────────────►│               │              │
    │              │               │              │
    │              │ participant_joined            │
    │              │──────────────►│              │
    │              │               │              │
    │              │               │ join Socket Room
    │              │               │──────►        │
    │              │               │              │
    │              │               │ Broadcast: PARTICIPANT_JOINED
    │              │               │──────────────►│
    │              │               │              │
    │              │               │ SendFullState │
    │              │               │─────────────►│
    │              │               │ (room:state_sync)
    │              │               │              │
    │◄─────────────┴──────────────┴──────────────┘
    │  更新 UI：人数 + 用户列表
```

### 7.2 音频状态变更

```
用户 A 点击静音按钮
    │
    ▼
[Client A] localTrack.setMuted(true)
    │
    ▼
[Client A] socket.emit('audio:changed', {isMuted: true, seq: 5})
    │
    ▼
[Socket.IO] ──► [Go Hub] ──► [房间广播]
    │
    ▼
[Go Backend] BroadcastToRoom(roomID, exclude=A)
    │
    ├─► [Client B] 收到 room:notification
    │       ▼
    │   更新 UI：A 的麦克风图标变灰
    │
    └─► [Client C] 收到 room:notification
            ▼
        更新 UI：A 的麦克风图标变灰
```

---

## 8. 扩展性设计

### 8.1 未来可扩展的通知类型

| 通知类型 | 触发条件 | 用途 |
|---------|---------|------|
| `SCREEN_SHARE_CHANGED` | 用户开始/停止屏幕共享 | 显示共享标识 |
| `RECORDING_CHANGED` | 房间录制状态变更 | 显示录制中提示 |
| `ROOM_LOCKED` | 房主锁定/解锁房间 | 控制加入权限 |
| `CHAT_MESSAGE` | 用户发送文字消息 | 复用同一通道 |
| `HAND_RAISED` | 用户举手 | 会议场景 |

### 8.2 多实例扩展（未来）

当前单实例架构可直接通过内存广播。若后端需要多实例部署：

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Go App    │     │   Redis     │     │   Go App    │
│  Instance 1 │◄───►│  Pub/Sub   │◄───►│  Instance 2 │
└──────┬──────┘     └─────────────┘     └──────┬──────┘
       │                                        │
       ▼                                        ▼
  [Socket.IO]                             [Socket.IO]
   Clients A-B                            Clients C-D
```

- 引入 Redis Pub/Sub 作为跨实例消息总线
- 各实例订阅 `room:{roomID}:notification` 频道
- 收到消息后转发给本地连接的 Socket.IO 客户端

---

## 9. 决策记录 (ADR)

### ADR-001: 使用 Socket.IO 而非新建 WebSocket 通道

- **决策**: 复用现有 `internal/signal/hub.go` 的 Socket.IO 通道
- **理由**: 零额外基础设施成本，已有房间隔离机制，前端已有成熟 Hook
- **影响**: 通知与信令共享连接，需避免阻塞

### ADR-002: 前端主导说话检测

- **决策**: 使用 LiveKit SDK `ActiveSpeakersChanged` 事件，前端上报
- **理由**: 避免后端处理音频流，减少服务器计算压力
- **影响**: 说话检测准确性依赖客户端实现，可能存在短暂延迟

### ADR-003: 不持久化通知历史

- **决策**: 通知不写入数据库，仅内存广播
- **理由**: 语音房间是实时在场场景，历史通知价值低
- **影响**: 用户断线重连后通过 `room:state_sync` 恢复视图，而非消息回放

### ADR-004: 混合状态监听策略

- **决策**: Webhook 为主 + 轮询兜底
- **理由**: Webhook 提供实时性，轮询提供可靠性
- **影响**: 需要配置 LiveKit Webhook URL，否则降级为纯轮询

---

## 10. 接口规范

### 10.1 Socket.IO 事件列表

| 事件名 | 方向 | 描述 |
|--------|------|------|
| `room:state_sync` | Server -> Client | 全量状态同步（加入房间时） |
| `room:notification` | Server -> Client | 增量通知事件 |
| `audio:changed` | Client -> Server | 本地音频状态变更上报 |
| `speaking:changed` | Client -> Server | 说话者列表变更上报 |

### 10.2 HTTP API 扩展

现有 API 已提供：

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/signal/participants` | 查询房间参与者列表（状态恢复用） |
| GET | `/api/v1/signal/rooms` | 查询房间列表 |

---

## 11. 风险评估

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|---------|
| Webhook 丢失导致状态漂移 | 中 | 中 | 30s 轮询兜底 + 新用户全量同步 |
| 高频静音切换导致消息风暴 | 低 | 中 | 前端节流 200ms + 序列号去重 |
| 50+ 人大房间广播性能问题 | 低 | 高 | 未来引入 Redis Pub/Sub 多实例支持 |
| Socket.IO 连接断开状态残留 | 中 | 低 | LiveKit Webhook `participant_left` 自动清理 |

---

## 12. 下一步行动

1. **后端实现**
   - 创建 `internal/notification/` 模块
   - 实现 Webhook 接收端点 `/api/v1/livekit/webhook`
   - 实现广播逻辑与全量同步逻辑

2. **前端实现**
   - 创建 `useRoomNotifications()` Hook
   - 更新 `<UserList />` / `<RoomHeader />` 组件绑定状态
   - 实现 `<NotificationToast />` 组件

3. **配置**
   - LiveKit 配置 Webhook URL 指向后端
   - 环境变量配置心跳轮询间隔

4. **测试**
   - 多用户并发加入/离开场景测试
   - 快速静音切换消息顺序测试
   - Webhook 失效降级测试

---

*文档结束*
