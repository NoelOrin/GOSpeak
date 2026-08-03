# AGENTS.md — hooks (SFU media layer)

## Structure

```
hooks/
├── livekit/              # 历史 LiveKit 封装，逐步收敛到统一 SFU client 抽象
│   ├── index.ts          # 统一导出
│   ├── createRoom.ts     # Room 实例创建与连接
│   ├── roomAction.ts     # 房间操作（加入/离开）
│   ├── useSubcribeTrack.ts # 音轨订阅管理
│   └── useToken.ts       # Token 获取
├── media.ts              # 媒体设备相关
└── useTitle.ts           # 页面标题
```

## Current state

当前 `hooks/livekit/` 仍服务于主房间链路，但它应被视为历史实现层，而不是未来唯一入口。新的房间媒体能力应优先通过统一 SFU client 抽象接入，hooks 只保留页面装配职责。

## Key Hooks

### createRoom.ts
创建 LiveKit Room 实例，配置 WebRTC 连接参数。当前仍用于 LiveKit provider，但不应继续向页面层扩散 LiveKit 专属类型。

### useSubcribeTrack.ts
订阅远端音轨，处理音频播放。与 `handler_audio` 模块配合完成音量控制。

### roomAction.ts
房间操作封装（加入/离开的副作用处理）。

## Audio Pipeline

```
LiveKit Room → Track Subscribed → handler_audio (AudioContext) → 音量控制 → 输出
```

`handler_audio/` 模块独立于 hooks，负责 AudioContext 生命周期和按 identity 的音量调节（`setVolumeByIdentity`）。

## Evolution direction

- 页面层统一依赖 `@gospeak/sfu-client`
- hook 层负责把房间页状态、socketStore、audio handler 与 SFU client 实例装配起来
- 远端音轨、活跃说话者、麦克风控制等输出统一为 provider-agnostic 结构
