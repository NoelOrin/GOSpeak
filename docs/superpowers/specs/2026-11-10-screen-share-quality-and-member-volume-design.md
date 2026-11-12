# 共享屏幕画质与他人语音音量：快速调节 + 设置项 + 持久化

**状态：** 已批准（2026-11-10）

## 背景

房间内目前有两处调节体验缺口：

1. **共享屏幕画质不可调**。`packages/sfu-client` 的 LiveKit 实现写死 `getDisplayMedia` 默认 1080p + `ScreenSharePresets.h1080fps15` 档编码，共享者无法在「清晰」与「省带宽/省 CPU」之间取舍；弱网或多人观看时没有降档手段。
2. **他人语音音量调节不持久、不可靠**。`voiceChatStore.otherMemberState` 已经落 IndexedDB，但持久值只在 `MemberCard` 渲染时通过 `useEffect` 调 `setVolumeByIdentity` 应用。用户切到文字频道、或成员卡片因共享画面让位而卸载后，已订阅的远端音频 track 仍按旧值播放；刷新页面后要等卡片重新出现才恢复音量。

目标：共享画质与他人音量都能「快速调节」，同一份设置在设置页也有入口，且全部持久化、重连/刷新后自动生效。

## 决策

### 1. 画质只调发送端

接收端 `setVideoQuality` 不纳入本次范围：LiveKit 的 `adaptiveStream` + `dynacast` 已按订阅端视口自动选层，再叠一层手动档位只会与自适应打架。发送端改档对全房间生效，收益更直接。

### 2. 四档，切换即时生效

| 档位 | 采集分辨率 | 码率 | 帧率 | contentHint |
|------|-----------|------|------|-------------|
| 原画 | 不限制（`getDisplayMedia` 原始尺寸） | 7 Mbps | 30 | detail |
| 1080p | 1920×1080 | 4 Mbps | 30 | detail |
| 720p | 1280×720 | 2 Mbps | 30 | detail |
| 480p | 854×480 | 800 kbps | 15 | motion |

- **开始共享**：`setScreenShareEnabled(true)` 时把当前档位转成 `ScreenShareCaptureOptions.resolution` + `TrackPublishOptions.screenShareEncoding`。
- **共享中改档**：不重新走 `getDisplayMedia`（否则会再弹一次窗口选择框），而是对已存在的本地 track 执行
  1. `mediaStreamTrack.applyConstraints({ width: { max }, height: { max }, frameRate: { max } })` 改采集分辨率；
  2. 对 `LocalVideoTrack.sender` 的 `encodings` 逐层重设 `maxBitrate`（顶层为档位码率，下层按 4 的幂次等比缩小），`setParameters` 应用。
  两者都是即时生效的，共享会话不中断。Safari 对 display capture 的 `applyConstraints` 支持不完整，失败时只降级为「下次共享生效」并 `console.warn`，不打断共享。

### 3. 持久化：新增独立 store

`app/web/src/stores/screenShareSettingsStore.ts`，沿用 `audioDeviceStore` 的「顶层 await 恢复 + `subscribe` 防抖落盘」模式：

- 存储键 `screenShareSettingsStore`（idb-keyval），只存 `{ quality }`。
- 恢复在模块加载时完成（顶层 await），消费方读到的第一帧就是持久值，避免「先按默认档共享、再被纠正」的闪烁。
- 设置页与状态条下拉读写同一个 store，双向同步。

他人音量继续用已有的 `voiceChatStore.otherMemberState`（已持久化），本次只补「应用时机」。

### 4. 应用时机：handler_audio 增加成员状态解析器

`handler_audio` 是远端音频 track 的唯一入口。新增 `setMemberStateResolver`：

- 模块内默认解析器直接读 `voiceChatStore` 的 `otherMemberState[identity]`（`outputVolume` / `isMute`），`setupAudioHandler` 时注册。
- `onTrackSubscribed` 在算 `effectiveVolume` 之前先按解析器写入 `volumes` / `mutedIdentities`，因此**不依赖任何组件是否挂载**。
- `cleanupAudioHandler` 清空 resolver 之外的运行时状态（沿用现有逻辑）。

### 5. UI 入口

| 位置 | 内容 | 说明 |
|------|------|------|
| `VoiceStatusBar` 共享按钮左侧 | 画质下拉（原画/1080p/720p/480p） | 仅 `canShareScreen` 时渲染；常驻左下，任意页面都能调；共享中改档即时生效 |
| 设置 → 音频 tab | 新增「屏幕共享」Section：画质 `Select` + 说明 | 与状态条共用 store |
| 房间成员卡片（他人） | 音量行：静音按钮 + 滑块 + 百分比（点百分比回 100%） | 替换现有「音量」小字 + 无反馈滑块；静音时仍可见（便于就地解除） |

设置页不重复放「他人音量」滑块：成员音量天然是「针对某个具体成员」的操作，放在成员卡片上语义更直接（用户已确认）。

## 数据流

```
状态条下拉 / 设置页 Select
        │
        ▼
screenShareSettingsStore.quality  ──(subscribe, debounce 200ms)──▶ IndexedDB
        │
        ▼  (VoiceStatusBar effect，client 或 quality 变化)
SFUClient.setScreenShareQuality(quality)
        │  未共享：记下，下次 setScreenShareEnabled 用
        ▼  共享中：applyConstraints + sender.setParameters（即时）
LiveKit LocalVideoTrack

成员卡片滑块 / 静音按钮
        │
        ▼
voiceChatStore.otherMemberState[identity]  ──(subscribe, debounce 200ms)──▶ IndexedDB
        │                                            │
        ▼ setVolumeByIdentity / setMutedByIdentity   ▼ handler_audio 默认 resolver
handler_audio（即时作用于已订阅 track）        新 track 订阅时自动套用
```

## 错误处理

- 改档失败（`applyConstraints` 不支持 / `setParameters` 抛错）：不弹 toast（用户只是拖了个档位），`console.warn` 记录，保持原画质继续共享。
- 开始共享失败沿用现有逻辑：`NotAllowedError`（用户取消选择框）静默；其他错误 toast。
- 持久化读写失败：`console.error`，功能降级为内存态，不阻断调节。

## 测试

- `packages/sfu-client/src/screen-share-quality.test.ts`（node 环境）：四档预设的采集分辨率/码率/帧率映射、默认档、档位顺序稳定。
- `app/web/src/stores/screenShareSettingsStore.test.ts`：默认值、写入后落盘、模块重新加载后恢复（mock `idb-keyval`）。
- `app/web/src/handler_audio/index.test.ts` 增补：resolver 在 track 订阅时把持久音量/静音套用到 `setVolume`，且不覆盖用户当次的即时修改。
- 现有 `roomDetail.test.tsx` / `ScreenShareView.test.tsx` 保持通过。

## 非目标

- 接收端画质档位（`RemoteTrackPublication.setVideoQuality`）。
- 共享系统音频、摄像头画质。
- 服务端参与画质协商或带宽预算（纯客户端发送参数）。
