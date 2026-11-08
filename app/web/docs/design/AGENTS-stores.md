# AGENTS.md — stores

## Architecture

全局状态使用 **zustand** 模块级单例，无需 Provider。每个 store 采用**双形态**：

- `useXxxStore`：zustand 的 React hook，组件内用 selector 订阅（`useXxxStore((s) => s.xxx)`），只订阅需要的字段，避免整对象订阅引发无关重渲染。
- `xxxStore`：命令式门面，供非 React 上下文（路由 `beforeLoad`、拦截器、WebSocket 回调、工具函数）调用，内部统一走 `useXxxStore.getState()`。门面里的状态读取用函数形式（`xxx: () => ...`），保证每次拿到最新 state；action 直接绑定 `useXxxStore.getState().xxx`。

示例简化自 `src/stores/userStore.ts`：

```ts
import { create } from "zustand";

interface UserState {
  user: UserInfo | null;
  login: (u: UserInfo) => void;
  clearAuth: () => void;
}

// 形态一：React 组件内订阅
export const useUserStore = create<UserState>((set, get) => ({
  user: readCachedUser(),
  login: (u) => {
    localStorage.setItem(STORAGE_USER, JSON.stringify(u)); // 持久化手写，不用 zustand 中间件
    set({ user: u });
  },
  clearAuth: () => {
    localStorage.removeItem(STORAGE_USER);
    set({ user: null });
  },
}));

// 形态二：非 React 侧命令式门面
export const userStore = {
  /** 非 React 侧读取（路由守卫、拦截器等）；组件内请用 useUserStore */
  user: () => useUserStore.getState().user,
  isLoggedIn: () => !!useUserStore.getState().user,
  login: useUserStore.getState().login,
  clearAuth: useUserStore.getState().clearAuth,
};

export default userStore;
```

约定：

- 组件内一律 selector 订阅；禁止在组件里用 `getState()` 读状态后再指望自动刷新。
- 跨模块的非响应式读取一律经 `xxxStore` 门面或 `useXxxStore.getState()`，不要另建一套缓存。
- 纯本地 UI 状态的 store（如 `textRoomUiStore`、`workspaceUiStore`、`themeStore`）通常只暴露 `useXxxStore` hook，不必再包门面；需要跨模块命令式访问的 store 才提供双形态。

## Stores

实际文件（配套 `*.test.ts` / `*.spec.ts` 就近放置）：

| 文件 | 职责 |
|------|------|
| `socketStore.ts` | 实时信令编排 facade，内置连接/进房/离房等动作，导出 `useSocketStore` + `socketStore` 门面 + 领域类型 |
| `userStore.ts` | 当前用户与会话（登录态、profile、`ensureSession` 过期驱动的懒续期） |
| `themeStore.ts` | 主题切换（深色/浅色） |
| `audioDeviceStore.ts` | 音频输入/输出设备列表与选择 |
| `voiceChatStore.ts` | 本地语音状态（静音、音量），导出 `useVoiceChatStore` / `useVoiceChatActions` |
| `chatStore.ts` | 会话与文字房间消息状态 |
| `domainStore.ts` | 域数据与当前选中域 |
| `guestStore.ts` | 访客态 |
| `textRoomUiStore.ts` | 文字房间 UI 状态 |
| `workspaceUiStore.ts` | 工作区舞台（语音/文字）UI 状态 |

### socketStore.ts
实时信令编排 facade（不是所有逻辑的容器）。对外仍导出 `useSocketStore` 单例 hook、`socketStore` 命令式门面（`connected()`、`members()`、`joinRoom()`、`selectRoom()` 等）与类型，调用方默认继续从 `@/stores/socketStore` 导入。

内部组合 `src/socket/` 下的纯 TS 模块（不属于 stores）：

- `wsClient.ts`：transport
- `events.ts` / `socketEvents.ts`：事件常量与服务端事件绑定
- `roomState.ts`：房间/成员纯状态变换
- `providerReload.ts`：SFU 热切换刷新
- `tabLock.ts`：单标签页独占
- `types.ts`：领域类型

私聊事件通过 `chatStore` 注册的回调消费，避免两个 store 模块互相 import 形成环依赖（见 `setPrivateNewHandler`）。
