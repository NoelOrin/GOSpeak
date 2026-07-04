# 前端耦合优化 — 未完成项计划

> 基于 `docs/coupling-optimization-plan.md`，后端 Phase 1-3 已全部完成。
> 以下为前端剩余 3 项。

**范围**: `app/web` (React/TypeScript)

---

## 剩余任务

| # | 发现 | 严重度 | 原始阶段 | 文件 |
|---|------|--------|----------|------|
| 1 | apiClient 拦截器 import userStore 拿 token | 🔴 循环风险 | Phase 1.2 | `api/apiClient.ts` |
| 2 | socketStore 直接 import handler_audio | 🟡 耦合 | Phase 2.2 | `stores/socketStore.ts` |
| 3 | roomDetail 单组件依赖 7 个模块 | 🟡 臃肿 | Phase 3.1 | `room/roomDetail.tsx` |

---

### 1. apiClient token callback

**文件**: `app/web/src/api/apiClient.ts`
**问题**: apiClient 直接 import `userStore` 获取 token，造成 api 层 → store 层反向依赖，循环引用风险。
**修复**: apiClient 构造时接收 `getAccessToken: () => string | null` callback

```diff
- import { message } from "@/lib/utils/message";
- import userStore from "@/stores/userStore";
+ import { message } from "@/lib/utils/message";

- const API_BASE = import.meta.env.VITE_API_BASE || "";
- 
  interface PendingRequest {
    resolve: (value: unknown) => void;
    reject: (reason?: unknown) => void;
  }

+ type ApiClientOptions = {
+   getAccessToken: () => string | null;
+   onRefreshToken?: () => Promise<string | null>;
+ };
+ 
+ function createApiClient(options: ApiClientOptions) {
+   const { getAccessToken, onRefreshToken } = options;

  // ...existing code, replace userStore.getState().token with getAccessToken()
```

**改动量**: apiClient.ts + main.tsx 初始化传入 callback

**调用方**: `main.tsx` 或 App 初始化处传入

---

### 2. socketStore 音频解耦

**文件**: `app/web/src/stores/socketStore.ts`
**问题**: socketStore import `handler_audio` 播放音效，store 层承担副作用。
**修复**: 新建 `useRoomAudio` hook，监听 socket 的 member:joined/left 事件

```ts
// hooks/useRoomAudio.ts
import { useEffect } from "react";
import { useSocket } from "@/stores/socketStore";

export function useRoomAudio(roomName: string) {
  const socket = useSocket();

  useEffect(() => {
    if (!socket) return;
    const onJoined = () => { /* playJoinSound */ };
    const onLeft = () => { /* playLeaveSound */ };
    socket.on("member:joined", onJoined);
    socket.on("member:left", onLeft);
    return () => {
      socket.off("member:joined", onJoined);
      socket.off("member:left", onLeft);
    };
  }, [socket, roomName]);
}
```

**修改文件**: 
- 新建 `hooks/useRoomAudio.ts`
- 修改 `stores/socketStore.ts` — 删除 handler_audio import
- 调用方（roomDetail 或 RoomPage）使用 `useRoomAudio()`

---

### 3. roomDetail 拆 hooks

**文件**: `app/web/src/components/room/roomDetail.tsx`
**问题**: 单组件依赖 7 个模块，混合 token/SFU/session/audio/socket 逻辑。
**修复**: 拆为 3 个独立 hooks

| Hook | 职责 |
|------|------|
| `useRoomJoin(roomName)` | token 获取 + socket 加入 + SFU 连接 + 清理 |
| `useSFUSession(roomName)` | SFUClient 生命周期 + reconnect |
| `useRoomAudio(roomName)` | audio handler + 设备管理 |

```ts
// hooks/useRoomJoin.ts
export function useRoomJoin(roomName: string) {
  const [token, setToken] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  // 获取 token → socket 加入 → SFU 连接
  // 返回 { token, error, isJoined, leave }
}
```

```ts
// hooks/useSFUSession.ts  
export function useSFUSession(roomName: string, token: string | null) {
  const clientRef = useRef<SFUClient | null>(null);

  // token 就绪后 connect + listen events
  // 返回 { participants, isConnected }
}
```

```ts
// hooks/useRoomAudio.ts  
export function useRoomAudio(roomName: string) {
  // audio handler setup + 设备管理 + 音效播放
  // 返回 { isMuted, toggleMute, devices }
}
```

**修改文件**:
- 新建 `hooks/useRoomJoin.ts`
- 新建 `hooks/useSFUSession.ts`
- 新建/更新 `hooks/useRoomAudio.ts`
- 修改 `roomDetail.tsx` — 使用 3 个 hook 替代内联逻辑

---

## 执行顺序

1. apiClient token callback — 最快，单一文件改动
2. socketStore 音频解耦 — 中等，新建 hook + 删 import
3. roomDetail 拆 hooks — 最大，新建 3 个 hook + 重写 roomDetail

## 总亮点

- 3 项均为单一文件级改动，无跨子系统影响
- 每项可独立 PR 提交
- 后端已 100% 完成，无需等待