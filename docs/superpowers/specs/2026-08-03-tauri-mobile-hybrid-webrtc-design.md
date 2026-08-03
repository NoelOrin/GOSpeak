# Tauri 移动端混合 WebRTC 方案设计

> 日期：2026-08-03
> 状态：待评审

## 背景与目标

GOSpeak 需要移动端 App，同时满足：

- 复用现有 `@gospeak/web`（SolidJS + Vite + TanStack Router），Web 端继续可用。
- 移动端通过 Tauri 2 打包 Android 与 iOS。
- 切后台、锁屏后，语音仍能持续（LiveKit 优先）。

纯 Tauri WebView 方案无法保证后台语音，因为 WebRTC 跑在 WebView 的 JS 引擎里，切后台后 WebView 可能被系统挂起。本设计采用混合架构：UI 与信令逻辑留在 WebView，媒体平面迁移到原生 WebRTC SDK。

## 总体架构

```text
WebView（@gospeak/web）
├── UI / 状态 / 房间管理 / 禁言策略 / 音量计算
├── WS 信令（前台连接，room membership / mute / kick / active speakers）
└── SFUClient 接口
        ↓ Tauri IPC（invoke / events）
原生媒体插件（tauri-plugin-gospeak-media）
├── Android：LiveKit Android SDK + 前台服务
└── iOS：LiveKit iOS SDK + AVAudioSession / 后台音频
        ↓
LiveKit SFU
```

JS 侧新增 `NativeLiveKitSFUClient`，实现现有 `SFUClient` 接口，把 `joinRoom`、`setMicEnabled`、远端音轨、活跃发言者、断线重连等映射为 Tauri IPC 调用与原生事件。业务层 `runVoiceJoin`、`handler_audio`、`useVoiceSession` 不需要重写。

## 关键设计决策

### 1. 媒体桥插在 `SFUClient` 之后

现有 `packages/sfu-client` 已提供统一接口：

- `joinRoom` / `leaveRoom` / `setMicEnabled`
- `onRemoteAudioTrack` / `onRemoteAudioTrackRemoved`
- `onActiveSpeakers` / `onDisconnected` / `onReconnecting` / `onReconnected`
- `getExistingRemoteAudioTracks`

原生客户端实现同一接口，UI 层无感知。非 Tauri、非 LiveKit 场景继续走现有 Web SFU 客户端，保证 Web 不回归。

### 2. 远端音轨通过 `NativeRemoteAudioTrack` 适配

`handler_audio` 目前依赖 `RemoteAudioTrackLike.attach()` 返回 `HTMLMediaElement` 并调用 `play()`、`setVolume()`、`setSinkId()`。

原生实现约定：

- `attach()` 返回一个隐藏的 `HTMLAudioElement` 占位节点，满足现有挂载逻辑；真实音频由原生 SDK 直接播放。
- `setVolume()` 通过 IPC 设置原生侧该参与者的增益。
- `RemoteAudioTrackLike` 增加可选 `setOutputDevice?(deviceId)`，`handler_audio` 优先调用它，否则回退到元素 `setSinkId`。
- `detach()` 通过 IPC 停止该参与者的原生播放并释放资源。

### 3. 原生媒体只覆盖 LiveKit

第一版原生媒体层只支持 LiveKit：

- Android 使用 LiveKit Android SDK。
- iOS 使用 LiveKit iOS SDK。
- Agora、SRS、Cloudflare 在移动端继续使用现有 Web SFU 客户端（前台可用，后台语音不承诺）。
- 后续如需要，Agora 可加官方原生 SDK；SRS/Cloudflare 基于原生 WebRTC 实现 WHIP/WHEP。

### 4. 后台语音需要服务端宽限会话

当前 `Hub.OnDisconnect` 会立即清理房间成员并异步执行 SFU cleanup。移动端切后台时 WebView 挂起，JS WS 会断开，若沿用当前逻辑，用户会被移出房间、媒体被清理。

设计增加“移动后台语音会话”：

- 移动端在 `ROOM_JOIN_SFU` 请求中携带 `mobile_background_voice=true`。
- Hub 为该成员标记后台语音会话。
- WS 意外断开时，不立即清理，而是进入 `VOICE_BACKGROUND_GRACE`（默认 10 分钟）宽限期。
- 宽限期内客户端重连并重新 `ROOM_JOIN_SFU`，取消清理并恢复会话。
- 宽限期结束仍未重连，按现有逻辑清理。
- 显式 `ROOM_LEAVE`、切房、踢出仍立即清理。

后台期间服务端对禁言/踢人的执行不依赖客户端 WS，仍由服务端直接操作 SFU，因此后台语音可被继续管理。

### 5. 地址配置与后端 Origin

- `apiClient` 默认地址改为 `import.meta.env.VITE_API_BASE_URL || "/"`。
- 移动端构建注入 `VITE_API_BASE_URL` 与 `VITE_SOCKET_URL`。
- 部署端配置 `CORS_ORIGIN` 与 `WS_ALLOWED_ORIGINS`，放行 Tauri Android Origin `http://tauri.localhost` 与 iOS Origin `tauri://localhost`。

## 原生桥协议

命令（JS → Native）：

- `media_join { token, serverUrl, room, identity, audioOptions }`
- `media_leave`
- `media_set_mic { enabled }`
- `media_set_participant_volume { identity, volume }`
- `media_set_output_device { deviceId }`
- `media_get_state`

事件（Native → JS）：

- `media:remote_track_added { identity }`
- `media:remote_track_removed { identity }`
- `media:active_speakers { identities }`
- `media:disconnected { reason, unrecoverable }`
- `media:reconnecting`
- `media:reconnected`
- `media:local_speaking { speaking }`

## 权限与系统配置

Android Manifest：

- `INTERNET`
- `RECORD_AUDIO`
- `FOREGROUND_SERVICE`
- `FOREGROUND_SERVICE_MICROPHONE`
- `POST_NOTIFICATIONS`
- `WAKE_LOCK`

iOS Info.plist：

- `NSMicrophoneUsageDescription`
- `UIBackgroundModes` 包含 `audio`
- 内网调试时增加 `NSLocalNetworkUsageDescription`
- 非 HTTPS 调试环境需要 ATS 例外

## 数据流

前台进房：

1. JS 通过现有 API 获取 JoinToken。
2. `loadSfuClient` 检测 Tauri 移动端 + LiveKit，创建 `NativeLiveKitSFUClient`。
3. JS 调用 `media_join`，原生 SDK 连接 LiveKit 并启用麦克风。
4. `setupAudioHandler` 注册原生事件回调。
5. JS 继续通过 WS 完成 `ROOM_JOIN` 与 `ROOM_JOIN_SFU`，并携带后台语音标记。

切后台：

1. WebView 挂起，JS WS 断开。
2. 原生媒体引擎保持 LiveKit 连接。
3. 服务端收到 WS 断开，进入宽限期，不清理成员与媒体。
4. 后台语音继续，禁言/踢人由服务端直接作用于 SFU。

回前台：

1. WebView 恢复，JS WS 重连。
2. 重新执行房间信令加入，恢复成员状态。
3. 原生媒体客户端上报 `media:reconnected` 或当前状态，UI 同步。

## 错误处理

- 麦克风权限拒绝：原生层返回权限错误，JS 展示明确提示并阻止进房。
- `VITE_API_BASE_URL` / `VITE_SOCKET_URL` 缺失：移动端构建阶段直接报错，避免运行时指向 `tauri.localhost`。
- 原生媒体连接失败：按现有进房失败流程回滚，回退 Web SFU 客户端仅在非 LiveKit 场景使用，不在同一会话内自动降级。
- 后台宽限期超时：服务端按正常断线清理；客户端回前台后重新进房即可。

## 测试与验收

- Web 回归：`pnpm --filter @gospeak/web build`、`check`、现有 vitest 全通过。
- JS 桥单元测试：mock Tauri `invoke` / `listen`，覆盖 `NativeLiveKitSFUClient` 与 `NativeRemoteAudioTrack`。
- 服务端单测：宽限期清理、重连取消清理、显式离开立即清理、Web 旧行为不变。
- Android 真机：前台进房、说话、听音、禁言/踢人、切后台 10 分钟语音不断、回前台状态恢复。
- iOS 真机：前台进房、锁屏后语音持续、回前台恢复、麦克风权限拒绝提示。
- 打包验收：Android APK/AAB 可安装，iOS archive 可签名。

## 范围外

- Agora / SRS / Cloudflare 的原生媒体实现。
- 推送通知、App Store 自动化发布、Go 后端打包进 App。
- 后台消息实时推送（后台仅保持语音，房间事件在回前台后同步）。
- Flutter 重写移动端。
- 原生信令完整代理（第一版用服务端宽限会话替代）。
