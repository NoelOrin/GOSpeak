# Tauri 移动端混合 WebRTC 实施计划

> **Status (2026-08-13):** 🔴 未启动 (2026-08-13) — 仓库无任何 tauri 代码; 唯一未动工的大计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在复用 `@gospeak/web` 的前提下，为 GOSpeak 增加 Tauri 2 移动端壳与原生 WebRTC 媒体层，实现 Android/iOS 前台与后台 LiveKit 语音，同时保持 Web 端行为不变。

**Architecture:** WebView 承载 UI 与前台 WS 信令；原生 Tauri 插件承载 LiveKit 媒体、Android 前台服务与 iOS 后台音频；JS 通过 `NativeLiveKitSFUClient` 实现现有 `SFUClient` 接口；服务端为移动后台语音会话增加宽限期清理，解决 WebView 挂起导致 WS 断开后被移出房间的问题。

**Tech Stack:** Tauri 2 · Rust · Kotlin · Swift · LiveKit Android/iOS SDK · SolidJS · TypeScript · Vite · Go/Gin

## Global Constraints

- 不改写 `@gospeak/web` 的业务结构；非 Tauri、非 LiveKit 场景必须走现有 Web SFU 客户端。
- Web 端构建、测试、页面行为不能回归。
- 第一版原生媒体只支持 LiveKit；Agora、SRS、Cloudflare 在移动端继续使用 Web 客户端。
- 不新增推送通知、App Store 自动化、Go 后端打包进 App 等范围外能力。
- 代码中不加非必要注释，不使用 emoji。
- `app/server/internal/signal/hub.go` 与 `message_bridge.go` 已有未提交改动，实施时先阅读 diff，增量合并，不得覆盖用户改动。

---

## Phase 0: 工具链与 Tauri 壳

### Task 0.1: 验证并补齐移动端工具链

**Files:**
- 无代码修改

**Interfaces:**
- 本机需要：Rust/cargo、Android SDK + NDK、JDK、完整 Xcode、`ANDROID_HOME`、`JAVA_HOME`。

- [ ] Step 1: 验证以下命令，缺失项先安装再继续
  ```bash
  rustc --version
  cargo --version
  xcodebuild -version
  java -version
  adb version
  echo "$ANDROID_HOME"
  ```
- [ ] Step 2: 记录当前环境缺口并在任务收尾时更新文档

### Task 0.2: 增加 Tauri 依赖与脚本

**Files:**
- Modify: `app/web/package.json`

**Interfaces:**
- Consumes: 现有 pnpm workspace。
- Produces: `tauri` 相关 npm 脚本与 `@tauri-apps/cli`、`@tauri-apps/api` 依赖。

- [ ] Step 1: 在 `app/web` 增加依赖
  ```bash
  pnpm --filter @gospeak/web add -D @tauri-apps/cli
  pnpm --filter @gospeak/web add @tauri-apps/api
  ```
- [ ] Step 2: 在 `app/web/package.json` 增加脚本
  ```json
  "tauri": "tauri",
  "mobile:android:dev": "tauri android dev",
  "mobile:android:build": "tauri android build",
  "mobile:ios:dev": "tauri ios dev",
  "mobile:ios:build": "tauri ios build"
  ```
- [ ] Step 3: 运行 `pnpm --filter @gospeak/web check` 确认 package 改动无回归

### Task 0.3: 初始化 `src-tauri`

**Files:**
- Add: `app/web/src-tauri/`（CLI 生成）

**Interfaces:**
- Consumes: `app/web/dist`、`app/web/index.html`。
- Produces: Tauri 配置、Rust 入口、capabilities、icons、Android/iOS 生成工程。

- [ ] Step 1: 在 `app/web` 执行
  ```bash
  pnpm --filter @gospeak/web exec tauri init
  ```
- [ ] Step 2: 设置 `tauri.conf.json` 关键字段
  ```json
  {
    "productName": "GOSpeak",
    "identifier": "com.gospeak.app",
    "build": {
      "beforeDevCommand": "pnpm dev",
      "devUrl": "http://localhost:3000",
      "beforeBuildCommand": "pnpm build",
      "frontendDist": "../dist"
    }
  }
  ```
- [ ] Step 3: 确认 `src-tauri/capabilities/default.json` 包含 `core:default` 与 `event:default`
- [ ] Step 4: 运行 `pnpm --filter @gospeak/web exec tauri build`（桌面壳）验证壳与构建产物可加载
- [ ] Step 5: 提交
  ```bash
  git add app/web/package.json app/web/pnpm-lock.yaml app/web/src-tauri
  git commit -m "feat(web): add tauri mobile shell"
  ```

### Task 0.4: 移动端地址配置

**Files:**
- Modify: `app/web/src/api/apiClient.ts:228`
- Modify: `app/web/src/socket/wsClient.ts:31`
- Add: `app/web/.env.mobile`
- Modify: `app/web/.env.example`

**Interfaces:**
- Consumes: `VITE_API_BASE_URL`、`VITE_SOCKET_URL`。
- Produces: 移动端可连接远程 Go 后端；Web 默认同源行为不变。

- [ ] Step 1: `apiClient.ts` 默认地址改为
  ```ts
  export default new APIClient(import.meta.env.VITE_API_BASE_URL || "/");
  ```
- [ ] Step 2: `wsClient.ts` 在空地址时优先使用 `import.meta.env.VITE_SOCKET_URL`，仍为空才回退 `window.location.host`
- [ ] Step 3: 新增 `.env.mobile` 示例
  ```bash
  VITE_API_BASE_URL=https://gospeak.example.com
  VITE_SOCKET_URL=wss://gospeak.example.com/ws
  VITE_SFU_PROVIDER=livekit
  ```
- [ ] Step 4: 移动端构建脚本加载 `.env.mobile`，Web 构建不加载
- [ ] Step 5: 验证 Web 构建与测试
  ```bash
  pnpm --filter @gospeak/web build
  pnpm --filter @gospeak/web test
  pnpm --filter @gospeak/web check
  ```
- [ ] Step 6: 提交

---

## Phase 1: JS 侧原生媒体桥

### Task 1.1: 定义桥协议类型

**Files:**
- Add: `app/web/src/mobile/bridge/types.ts`

**Interfaces:**
- Consumes: `@gospeak/sfu-client/types` 的 `SFUClient`、`SFUDisconnectInfo`。
- Produces: Tauri command/event 的强类型载荷。

- [ ] Step 1: 定义 `MediaJoinPayload`、`MediaVolumePayload`、`MediaEventMap` 等类型
- [ ] Step 2: 类型字段与设计文档第 5 节保持一致

### Task 1.2: 实现 Tauri 桥封装

**Files:**
- Add: `app/web/src/mobile/bridge/nativeBridge.ts`

**Interfaces:**
- Consumes: `@tauri-apps/api/core.invoke`、`@tauri-apps/api/event.listen`。
- Produces: `joinMedia`、`leaveMedia`、`setMic`、`setVolume`、`setOutputDevice`、`onMediaEvent`。

- [ ] Step 1: 封装 command 调用，统一错误为带业务信息的 `Error`
- [ ] Step 2: 封装 event 订阅，返回取消函数
- [ ] Step 3: 增加 `isNativeMediaAvailable()`，非 Tauri 环境直接返回 false

### Task 1.3: 平台检测与客户端工厂分流

**Files:**
- Add: `app/web/src/mobile/sfu/isTauriMobile.ts`
- Modify: `app/web/src/components/room/services/loadSfuClient.ts`

**Interfaces:**
- Consumes: 现有 `createSFUClient`。
- Produces: Tauri 移动端 + LiveKit 返回 `NativeLiveKitSFUClient`，其他情况返回现有 Web 客户端。

- [ ] Step 1: `isTauriMobile()` 通过 `window.__TAURI_INTERNALS__` 与平台标识判断
- [ ] Step 2: `loadSfuClient` 分流
  ```ts
  if (isTauriMobile() && provider === "livekit") {
    return createNativeLiveKitClient(options);
  }
  return createSFUClient(provider, options);
  ```
- [ ] Step 3: 保证 `preloadSfuClient` 对 Web 与移动端都可用

### Task 1.4: 实现 `NativeRemoteAudioTrack`

**Files:**
- Add: `app/web/src/mobile/sfu/nativeRemoteAudioTrack.ts`
- Modify: `packages/sfu-client/src/types.ts`
- Modify: `app/web/src/handler_audio/index.ts`

**Interfaces:**
- Implements: `RemoteAudioTrackLike`。
- Consumes: 原生 remote track 事件与音量 IPC。
- Produces: 隐藏 `HTMLAudioElement` 占位、原生音量控制、原生输出设备控制。

- [ ] Step 1: 在 `RemoteAudioTrackLike` 增加可选方法
  ```ts
  setOutputDevice?(deviceId: string): void;
  ```
- [ ] Step 2: `NativeRemoteAudioTrack.attach()` 返回隐藏 `HTMLAudioElement`
- [ ] Step 3: `setVolume` / `setOutputDevice` 调用原生桥
- [ ] Step 4: `detach()` 调用原生停止播放并释放
- [ ] Step 5: `handler_audio` 的 `applySinkId` 优先调用 `track.setOutputDevice`，无该方法才走元素 `setSinkId`

### Task 1.5: 实现 `NativeLiveKitSFUClient`

**Files:**
- Add: `app/web/src/mobile/sfu/nativeLiveKitClient.ts`

**Interfaces:**
- Implements: `SFUClient`。
- Consumes: `nativeBridge.ts`、`NativeRemoteAudioTrack`。
- Produces: 与现有 Web LiveKit 客户端等价的媒体会话生命周期。

- [ ] Step 1: `joinRoom` 调用 `media_join`，成功后注册原生事件
- [ ] Step 2: `leaveRoom` / `destroy` 调用 `media_leave` 并清理事件订阅
- [ ] Step 3: `setMicEnabled` 调用 `media_set_mic`
- [ ] Step 4: 原生事件映射为 `onRemoteAudioTrack`、`onRemoteAudioTrackRemoved`、`onActiveSpeakers`、`onDisconnected`、`onReconnecting`、`onReconnected`
- [ ] Step 5: `getExistingRemoteAudioTracks` 通过 `media_get_state` 补齐 join 竞态

### Task 1.6: JS 桥单元测试

**Files:**
- Add: `app/web/src/mobile/__tests__/nativeBridge.test.ts`
- Add: `app/web/src/mobile/__tests__/nativeLiveKitClient.test.ts`
- Add: `app/web/src/mobile/__tests__/nativeRemoteAudioTrack.test.ts`

**Interfaces:**
- Mock `@tauri-apps/api/core` 与 `@tauri-apps/api/event`。

- [ ] Step 1: 覆盖 command 调用参数与错误透传
- [ ] Step 2: 覆盖事件订阅、取消与重复订阅
- [ ] Step 3: 覆盖 join 竞态 `getExistingRemoteAudioTracks`
- [ ] Step 4: 运行验证
  ```bash
  pnpm --filter @gospeak/web test
  pnpm --filter @gospeak/web check
  ```
- [ ] Step 5: 提交

---

## Phase 2: 服务端移动后台语音宽限会话

### Task 2.1: 信令进房携带移动端标记

**Files:**
- Modify: `app/web/src/stores/socketStore.ts:477`

**Interfaces:**
- Consumes: `isTauriMobile()`。
- Produces: `ROOM_JOIN_SFU` 请求携带 `mobile_background_voice`。

- [ ] Step 1: `joinRoomSFU` 增加可选参数或由调用方注入 `mobile_background_voice`
- [ ] Step 2: Tauri 移动端时置为 `true`，Web 端不携带或为 `false`
- [ ] Step 3: 更新现有 socket 测试，Web 默认载荷不变

### Task 2.2: Hub 支持后台语音标记

**Files:**
- Modify: `app/server/internal/signal/hub.go`
- Modify: `app/server/internal/signal/types.go`（如已有对应结构）
- Modify: `app/server/internal/config/config.go`

**Interfaces:**
- Consumes: `ROOM_JOIN_SFU` 的 `mobile_background_voice` 字段。
- Produces: `MemberInfo` 增加 `BackgroundVoice`；Hub 保存宽限期配置。

- [ ] Step 1: 阅读 `hub.go` 当前未提交 diff，理解现有 `OnDisconnect` 与 cleanup 流程
- [ ] Step 2: 解析 `ROOM_JOIN_SFU` 标记并写入成员状态
- [ ] Step 3: 新增 `VOICE_BACKGROUND_GRACE` 配置，默认 `10m`
- [ ] Step 4: 显式 `ROOM_LEAVE` 与踢出仍立即清理

### Task 2.3: 断线进入宽限期

**Files:**
- Modify: `app/server/internal/signal/hub.go`

**Interfaces:**
- Consumes: 成员 `BackgroundVoice` 标记。
- Produces: WS 意外断开时延迟 SFU cleanup 与成员清理。

- [ ] Step 1: `OnDisconnect` 对 `BackgroundVoice=false` 保持现有逻辑不变
- [ ] Step 2: 对 `BackgroundVoice=true` 启动定时清理；宽限期内重连取消
- [ ] Step 3: 定时清理与现有 `cleanupPublisher` 复用同一路径
- [ ] Step 4: 宽限期内成员仍显示在房间内，禁言/踢人继续直接作用于 SFU

### Task 2.4: 服务端单元测试

**Files:**
- Modify: `app/server/internal/signal/hub_test.go`
- Add: `app/server/internal/signal/hub_background_voice_test.go`

- [ ] Step 1: 覆盖 Web 旧行为：断线立即清理
- [ ] Step 2: 覆盖移动端断线后宽限期内不清理
- [ ] Step 3: 覆盖宽限期内重连取消清理
- [ ] Step 4: 覆盖宽限期超时后清理
- [ ] Step 5: 覆盖显式离开立即清理
- [ ] Step 6: 运行
  ```bash
  pnpm --filter @gospeak/server test
  ```
- [ ] Step 7: 提交

---

## Phase 3: Android 原生媒体插件

### Task 3.1: 原生插件可行性验证

**Files:**
- Add: `app/web/src-tauri/plugins/gospeak-media/`（Rust 插件骨架）

**Interfaces:**
- Consumes: Tauri 2 插件 API。
- Produces: 可注册的本地 Tauri 插件与 Android/iOS 宿主入口。

- [ ] Step 1: 验证 Tauri v2 本地插件结构、`build.rs`、Android/iOS 模板是否能编译
- [ ] Step 2: 验证 Android 前台服务能否由插件启动，并记录依赖与限制
- [ ] Step 3: 将结论补充到设计文档风险节

### Task 3.2: Android LiveKit 媒体实现

**Files:**
- Add: `app/web/src-tauri/plugins/gospeak-media/android/`（Kotlin）
- Modify: `app/web/src-tauri/Cargo.toml`

**Interfaces:**
- Implements: 设计文档第 5 节命令与事件协议。
- Consumes: LiveKit Android SDK。
- Produces: 原生 join/leave/mic、远端音轨事件、活跃发言者、单成员音量、输出设备。

- [ ] Step 1: 引入 LiveKit Android SDK 依赖
- [ ] Step 2: 实现 `media_join`：创建 `Room`、连接、启用麦克风
- [ ] Step 3: 实现 `media_leave` / `media_set_mic`
- [ ] Step 4: 实现远端音频轨添加/移除事件
- [ ] Step 5: 实现 active speakers、reconnecting/reconnected/disconnected 事件
- [ ] Step 6: 实现 `media_set_participant_volume` 与 `media_set_output_device`
- [ ] Step 7: 实现 `media_get_state` 返回已订阅参与者

### Task 3.3: Android 前台服务

**Files:**
- Modify: `app/web/src-tauri/plugins/gospeak-media/android/`（Kotlin）
- Modify: `app/web/src-tauri/gen/android/` 生成配置

**Interfaces:**
- Consumes: Android `Service` 生命周期。
- Produces: 进房后启动前台服务，切后台保持媒体引擎运行。

- [ ] Step 1: 实现 `GospeakVoiceService`
- [ ] Step 2: 进房时 `startForeground`，离开时停止
- [ ] Step 3: 配置通知渠道与 Android 13+ 通知权限
- [ ] Step 4: 处理音频焦点变化（来电、其他 App 播放）

### Task 3.4: Android 权限与构建验证

**Files:**
- Modify: `app/web/src-tauri/gen/android/` 对应 Manifest
- Modify: `app/web/.env.mobile`

**Interfaces:**
- Produces: 可安装 APK，具有麦克风与前台服务权限。

- [ ] Step 1: 加入 `INTERNET`、`RECORD_AUDIO`、`FOREGROUND_SERVICE`、`FOREGROUND_SERVICE_MICROPHONE`、`POST_NOTIFICATIONS`、`WAKE_LOCK`
- [ ] Step 2: 运行
  ```bash
  pnpm --filter @gospeak/web exec tauri android build
  ```
- [ ] Step 3: 真机验证前台进房、说话、听音、禁言/踢人
- [ ] Step 4: 真机验证切后台 10 分钟语音持续，回前台状态恢复
- [ ] Step 5: 提交

---

## Phase 4: iOS 原生媒体插件

### Task 4.1: iOS LiveKit 媒体实现

**Files:**
- Add: `app/web/src-tauri/plugins/gospeak-media/ios/`（Swift）
- Modify: `app/web/src-tauri/Cargo.toml`

**Interfaces:**
- Implements: 与 Android 相同的命令/事件协议。
- Consumes: LiveKit iOS SDK。

- [ ] Step 1: 引入 LiveKit iOS SDK
- [ ] Step 2: 实现 join/leave/mic、远端音轨事件、活跃发言者、音量与输出设备
- [ ] Step 3: 实现 `media_get_state`

### Task 4.2: iOS 音频会话与后台模式

**Files:**
- Modify: `app/web/src-tauri/plugins/gospeak-media/ios/`
- Modify: `app/web/src-tauri/gen/apple/` 对应 Info.plist

**Interfaces:**
- Produces: 锁屏/切后台持续语音。

- [ ] Step 1: 配置 `AVAudioSession` 为 `playAndRecord`，启用扬声器与蓝牙路由
- [ ] Step 2: Info.plist 增加 `NSMicrophoneUsageDescription` 与 `UIBackgroundModes=audio`
- [ ] Step 3: 处理来电打断与音频会话中断恢复

### Task 4.3: iOS 构建与真机验证

- [ ] Step 1: 运行
  ```bash
  pnpm --filter @gospeak/web exec tauri ios build
  ```
- [ ] Step 2: 真机验证前台进房、锁屏后语音持续、回前台恢复
- [ ] Step 3: 验证麦克风权限拒绝时给出明确提示
- [ ] Step 4: 提交

---

## Phase 5: 生命周期同步与错误处理

### Task 5.1: 前后台恢复同步

**Files:**
- Modify: `app/web/src/components/room/hooks/useVoiceSession.ts`
- Modify: `app/web/src/stores/socketStore.ts`

**Interfaces:**
- Consumes: 原生媒体事件与现有 socket 重连。
- Produces: 回前台后自动重连 WS、重新加入房间信令并恢复状态。

- [ ] Step 1: 监听 WebView `visibilitychange` 与 Tauri 生命周期事件
- [ ] Step 2: 回前台时若原生媒体仍连接，触发 socket 重连与 `ROOM_JOIN_SFU` 续约
- [ ] Step 3: 若原生媒体已断开，走现有断线清理与重新进房流程
- [ ] Step 4: 确保切房时立即 `media_leave` 且服务端不做宽限保留

### Task 5.2: 原生事件与 UI 状态映射

**Files:**
- Modify: `app/web/src/mobile/sfu/nativeLiveKitClient.ts`

**Interfaces:**
- Produces: 断线、重连、活跃发言者与现有 `handler_audio`、`speakingStore` 完全一致。

- [ ] Step 1: `media:disconnected` 区分可恢复与不可恢复
- [ ] Step 2: 不可恢复断连（重复身份、被移除、房间删除）立即清理，不重连
- [ ] Step 3: 可恢复断连触发 `onReconnecting` / `onReconnected`

### Task 5.3: 配置与权限错误提示

**Files:**
- Modify: `app/web/src/mobile/sfu/nativeLiveKitClient.ts`
- Modify: `app/web/vite.config.ts`

**Interfaces:**
- Produces: 移动端构建缺少地址时快速失败；权限/连接错误有用户可见提示。

- [ ] Step 1: 移动端构建前校验 `VITE_API_BASE_URL` 与 `VITE_SOCKET_URL`
- [ ] Step 2: 原生权限错误映射为中文提示并阻止进房
- [ ] Step 3: 补充 vitest 覆盖错误映射

### Task 5.4: 回归与提交

- [ ] Step 1: 运行
  ```bash
  pnpm --filter @gospeak/web test
  pnpm --filter @gospeak/web check
  pnpm --filter @gospeak/web build
  pnpm --filter @gospeak/server test
  ```
- [ ] Step 2: 提交

---

## Phase 6: 打包与验收

### Task 6.1: Android 打包与签名

**Files:**
- Add: 签名配置文档（不提交密钥）
- Modify: 打包相关说明

- [ ] Step 1: 配置 Android release keystore
- [ ] Step 2: 产出 APK 与 AAB
- [ ] Step 3: 记录签名命令与 CI 接入点

### Task 6.2: iOS 打包与签名

- [ ] Step 1: 配置 Apple Development/Distribution 证书
- [ ] Step 2: 产出 archive 与 IPA
- [ ] Step 3: 验证后台音频模式符合 App Store 要求

### Task 6.3: 文档与最终验收

**Files:**
- Add: `docs/mobile/tauri-mobile-build.md`
- Modify: `README.md`

- [ ] Step 1: 记录工具链安装、构建命令、环境变量、Origin 配置、权限说明
- [ ] Step 2: 执行最终验收矩阵
  - Web 登录/进房/语音/消息不回归
  - Android 前台语音与后台 10 分钟语音
  - iOS 前台语音与锁屏语音
  - 禁言/踢人后台生效
  - 非 LiveKit provider 回退 Web 客户端
- [ ] Step 3: 提交

---

## 验收标准

- Web 端构建、测试、行为不回归。
- Android/iOS 可产出安装包。
- LiveKit 语音在移动端前台与后台均可用。
- 后台期间服务端仍能执行禁言/踢人。
- 回前台后房间状态、成员、活跃发言者恢复一致。
- 非 Tauri 环境不受影响。
