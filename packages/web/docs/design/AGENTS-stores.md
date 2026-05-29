# AGENTS.md — stores

## Architecture

全局状态使用 SolidJS `createRoot` + `createSignal` 单例模式。每个 store 文件导出一个在模块顶层创建的单例对象，组件直接 import 使用，无需 Provider。

```ts
// 标准模式
import { createSignal, createRoot } from "solid-js";

const store = createRoot(() => {
  const [state, setState] = createSignal(initialValue);
  return { state, setState };
});

export default store;
```

## Stores

### socketStore.ts
Socket.IO 连接与房间状态管理。
- 连接/断开 Socket.IO
- 房间 CRUD（创建/加入/离开/列表）
- 成员列表同步（`members: MemberInfo[]`）
- 选中房间状态（`selectedRoomInfo`）
- 事件常量 `EVENTS`（与后端 `signal/events.go` 一致）

### userStore.ts
当前用户信息（登录状态、用户名等）。

### themeStore.ts
主题切换（深色/浅色模式，class 策略）。

### audioDeviceStore.ts
音频输入/输出设备列表与选择。

### voiceChatStore.ts
语音聊天状态（静音、音量等本地状态）。
