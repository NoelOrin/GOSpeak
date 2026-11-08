# AGENTS.md — hooks

## Structure

`src/hooks/` 现在只放与具体业务解耦的通用 hook：

```
hooks/
├── media.ts              # 音频设备枚举（getAudioDevices，getUserMedia 授权后 enumerateDevices）
├── useAuthParallax.ts    # 登录/注册页鼠标视差（lerp 缓动写 --px/--py，尊重 prefers-reduced-motion）
├── useUpload.ts          # 统一文件上传（presign → PUT 直传 / POST → confirm，含进度与错误态）
├── useErrorHandler.ts    # 返回一个 (error: unknown) => void 函数，供 Promise.catch / 事件回调把异步错误重抛到渲染阶段，交给最近的 ErrorBoundary
└── useErrorHandler.test.tsx
```

语音会话不在 `hooks/` 里，已收敛到 `components/room/` 的分层结构（见下）：

```
components/room/
├── VoiceSessionProvider.tsx   # 常驻宿主（React Context），装配会话/音频桥/提示音并向下提供 context
├── hooks/
│   ├── useVoiceSession.ts     # 唯一进房编排入口：token query、会话 phase、abort/teardown、UI state
│   ├── useRoomAudioBridge.ts  # SFUClient ↔ handler_audio：麦克风发布、主音量/静音、输出设备
│   └── useRoomSounds.ts       # 进入/离开提示音（由 presence 与会话 phase 驱动）
├── session/
│   ├── providers.ts           # 各 SFU provider 的 VoiceProviderAdapter（连接目标、串行策略、joinKey）
│   ├── runVoiceJoin.ts        # 进房执行 + 同 joinKey 并发合并（inFlightJoins）+ VoiceJoinAbortError
│   ├── voiceSessionTypes.ts   # Session / VoicePhase / VoiceProviderAdapter 等领域类型
│   └── voiceSessionUi.ts      # 重连 toast 等 UI 副作用
└── services/
    ├── loadSfuClient.ts       # provider chunk 预热 + createSFUClient 动态加载
    └── sfuSession.ts          # 从 JoinTokenResponse 解析 provider / connectTarget
```

## Current state

`hooks/` 只保留通用 UI/工具 hook；房间媒体能力全部在 `components/room/` 下按 UI / 编排 / provider 适配 / SFU 加载分层。SFU 会话由常驻 `VoiceSessionProvider` 持有，视图组件通过 context 消费——切到文字频道不会卸载会话、不会中断通话。`useVoiceSession.ts` 是锁定的编排入口，provider 专属的连接目标、token 字段等改动应落在 `session/providers.ts`、`session/runVoiceJoin.ts` 或 `packages/sfu-client/*`，不要扩散进编排 hook。

## Key Hooks

### media.ts
`getAudioDevices()`：先 `getUserMedia` 拿到授权，再 `enumerateDevices()` 归类出 `audioinputs` / `audiooutputs`，供 `audioDeviceStore` 使用。

### useAuthParallax.ts
登录/注册页鼠标视差，目标点 lerp 缓动后写入 `--px` / `--py`，各层按 `--depth` 取位移；`prefers-reduced-motion` 时直接跳过。

### useUpload.ts
统一上传：按 `category` 走 presign → PUT 直传（S3）或 POST（Local）→ confirm，返回 `public_url` / `object_key`，并暴露 `uploading` / `progress` / `error`。

Local 模式的中转上传**必须用原生 `fetch`**（见 `api/storage.ts#uploadFile`）：axios 实例默认 `Content-Type: application/json` 会把 `FormData` 序列化成 JSON，而手动设置 `multipart/form-data` 又会缺少 `boundary` 导致服务端 `c.FormFile` 解析失败；交给浏览器生成 `boundary` 才正确。

### useErrorHandler.ts
签名：`const handleError = useErrorHandler(): (error: unknown) => void`——返回一个函数，供 `Promise.catch` / 事件回调调用，把异步错误重抛到渲染阶段交给最近的边界（React 错误边界本身捕获不到异步错误，需此 hook 重抛）。

### 语音会话分层职责
- `useVoiceSession`：`useQuery` 拉 join token、维护会话阶段与 teardown，调用 `runVoiceJoin` 完成进房。
- `providers.ts` / `runVoiceJoin.ts`：provider 适配与并发进房合并，隔离 provider 差异。
- `loadSfuClient.ts` / `sfuSession.ts`：动态加载 SFU client、解析连接信息。
- `useRoomAudioBridge` / `useRoomSounds`：把会话接到音频处理与提示音，属于编排层，不进入 SFU 抽象层。

## Audio Pipeline

```
SFUClient (@gospeak/sfu-client)
  → useRoomAudioBridge → handler_audio (AudioContext / speakingStore)
  → setVolumeByIdentity / setMasterVolume / setMasterMuted / setAudioOutputDevice
  → 输出设备
```

`handler_audio/` 独立于 hooks，负责 AudioContext 生命周期、按 identity 的音量调节（`setVolumeByIdentity`）、活跃说话者（`speakingStore`）与通知音（`notificationSounds`）。进入/离开提示音由 `useRoomSounds` 集中在房间编排层处理：他人进出由信令 presence 驱动，自己进入由媒体会话 `media_ready/ready` 驱动，SFUClient 只负责媒体会话生命周期。

## Evolution direction

- 页面 / 视图层统一依赖 `@gospeak/sfu-client`（`SFUClient` / `SFUProvider` / `createSFUClient`），不直接引用 LiveKit 等 provider 专属类型。
- hook 层负责把房间页状态、`socketStore`、`handler_audio` 与 SFU client 实例装配起来；编排逻辑锁定在 `useVoiceSession`，适配逻辑下沉到 `session/` 与 `services/`。
- 远端音轨、活跃说话者、麦克风控制等输出统一为 provider-agnostic 结构；新增 provider 只需在 `session/providers.ts` 加 adapter、在 `services/loadSfuClient.ts` 加 preloader，不改编排 hook。
