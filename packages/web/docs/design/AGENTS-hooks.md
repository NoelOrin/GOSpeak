# AGENTS.md — hooks (LiveKit)

## Structure

```
hooks/
├── livekit/              # LiveKit 封装
│   ├── index.ts          # 统一导出
│   ├── createRoom.ts     # Room 实例创建与连接
│   ├── roomAction.ts     # 房间操作（加入/离开）
│   ├── useSubcribeTrack.ts # 音轨订阅管理
│   └── useToken.ts       # Token 获取
├── media.ts              # 媒体设备相关
└── useTitle.ts           # 页面标题
```

## Key Hooks

### createRoom.ts
创建 LiveKit Room 实例，配置 WebRTC 连接参数。返回 `RoomReturnType`，包含 `room` 实例和 `joinRoom`/`leaveRoom` 方法。

### useSubcribeTrack.ts
订阅远端音轨，处理音频播放。与 `handler_audio` 模块配合完成音量控制。

### roomAction.ts
房间操作封装（加入/离开的副作用处理）。

## Audio Pipeline

```
LiveKit Room → Track Subscribed → handler_audio (AudioContext) → 音量控制 → 输出
```

`handler_audio/` 模块独立于 hooks，负责 AudioContext 生命周期和按 identity 的音量调节（`setVolumeByIdentity`）。
