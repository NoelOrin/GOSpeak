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
实时信令编排 facade（不是所有逻辑的容器）。

内部组合：
- `socket/client.ts`：transport
- `socket/tabLock.ts`：单标签页独占
- `socket/roomState.ts`：房间/成员纯状态变换
- `socket/providerReload.ts`：SFU 热切换刷新
- `socket/types.ts`：领域类型

对外仍导出 `socketStore` 单例与类型，调用方默认继续从 `@/stores/socketStore` 导入。

### userStore.ts
当前用户信息（登录状态、用户名等）。

### themeStore.ts
主题切换（深色/浅色模式，class 策略）。

### audioDeviceStore.ts
音频输入/输出设备列表与选择。

### voiceChatStore.ts
语音聊天状态（静音、音量等本地状态）。
