> **Execution order superseded by** [`2026-07-16-bot-platform-unified.md`](./2026-07-16-bot-platform-unified.md). Keep this file as detailed appendix; do not run in parallel as a second source of truth.

# Bot 通用 SFU 旁听 + 指定房间 + Speech 事件 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `@gospeak/bot` 增加通用 SFU 媒体旁听能力，支持用户指定监听房间，入房后自动旁听，并对外发出 `OnSpeechPartial` / `OnSpeechFinal` 事件（由旁听音频管线驱动；ASR 可插拔）。

**Architecture:** 在 bot 进程内新增 `MediaListenService`：配置层决定“监听哪些房间”；`BotRunner` 启动后按配置自动 join 信令房 + SFU 旁听；旁听层通过 `SFUListenAdapter` 统一多 SFU 的“订阅远端音频 → PCM frame”差异；PCM 交给 `SpeechPipeline`（默认可接 mock/passthrough 或后续 ASR provider），产出 `SpeechEvent` 并注入现有 `EventBus`。现有 `packages/sfu-client` 面向浏览器 DOM，不直接复用；bot 侧新建 Node 友好的旁听适配层，接口语义对齐 SFU 抽象（join/leave/onRemoteAudio/provider）。

**Tech Stack:** TypeScript (`packages/bot`), vitest, Socket.IO client, 各 SFU Node/旁路 SDK（LiveKit 先落地，其余 adapter 接口预留并逐步实现）

---

## Scope

### In Scope
1. 新增 `OnSpeechPartial` / `OnSpeechFinal` 事件类型与 payload
2. `BotRunner` 入房自动旁听接线
3. **通用 SFU 旁听媒体层**（统一接口 + 多 provider adapter）
4. **用户指定监听房间**（env / config / 运行时命令）

### Out of Scope
- 完整云 ASR / 本地模型 provider 细节（可后续接 `SpeechPipeline`）
- 浏览器端字幕 UI
- 改 Go 后端 SFU 主链路
- 说话人分离 diarization（按 SFU identity 分轨即可）

---

## Product Behavior

### 用户指定监听房间
支持三种来源，优先级从高到低：

1. **运行时命令**（热更新）
   - `/listen add <room>`
   - `/listen remove <room>`
   - `/listen list`
   - `/listen clear`
2. **启动配置** `BotConfig.listenRooms: string[]`
3. **环境变量** `GOSPEAK_LISTEN_ROOMS=room-a,room-b`

规则：
- 启动时合并 env + config，去重
- 运行时命令变更写入内存 + `ctx.kv`（进程内持久），重启后仍以 env/config 为准（除非后续加文件持久化；本计划不做文件持久化）
- 新增房间：自动信令 join + SFU 旁听
- 移除房间：停止旁听 + 信令 leave + 关闭 speech session
- 空列表：不监听任何房间（除非插件/代码显式 `join`）

### 入房自动旁听
当房间进入“应监听集合”时：

```text
shouldListen(room)
  → socket join (signaling)
  → getSFUToken(room)
  → SFUListenAdapter.join(provider, token, serverUrl, ...)
  → onAudioFrame → SpeechPipeline
  → EventBus emit OnSpeechPartial / OnSpeechFinal
```

离开监听集合时反向清理。

---

## Architecture

```text
                    ┌─────────────────────────┐
  env/config/cmd →  │ ListenRoomRegistry      │
                    │ (desired rooms set)     │
                    └───────────┬─────────────┘
                                │ reconcile()
                                ▼
                    ┌─────────────────────────┐
                    │ MediaListenService      │
                    │ - desired vs active     │
                    │ - join/leave lifecycle  │
                    └───────────┬─────────────┘
                                │
              ┌─────────────────┼─────────────────┐
              ▼                 ▼                 ▼
      Socket signaling   SFUListenRouter    SpeechPipeline
      (room:join/leave)  (by provider)      (frame→speech)
                                │
              ┌─────────────────┼──────────────────┐
              ▼                 ▼                  ▼
        LiveKitAdapter   MediaSoupAdapter*   Srs/CF/Agora/Daily*
                                │
                                ▼
                        AudioFrameEvent
                                │
                                ▼
                     OnSpeechPartial/Final
                                │
                                ▼
                     Plugin EventBus / logs
```

\* 非 LiveKit adapter：本计划先落接口 + unsupported 明确报错；LiveKit 做可运行实现。MediaSoup/SRS 作为第二批可选实现任务，不阻塞主链路。

---

## Key Types

```ts
// packages/bot/src/core/types.ts
export enum EventType {
  // ...existing
  OnSpeechPartial = "OnSpeechPartial",
  OnSpeechFinal = "OnSpeechFinal",
  OnListenRoomChanged = "OnListenRoomChanged", // 可选，便于插件感知
}

export interface SpeechEvent {
  eventType: EventType.OnSpeechPartial | EventType.OnSpeechFinal;
  room: RoomRef;
  speaker: MemberRef;
  text: string;
  isFinal: boolean;
  confidence?: number;
  language?: string;
  provider: string; // speech provider name, e.g. "passthrough" | "asr:deepgram"
  mediaProvider?: string; // sfu provider, e.g. "livekit"
  timestamp: number;
}

export interface AudioFrameEvent {
  room: string;
  identity: string;
  pcm16: Int16Array;
  sampleRate: number;
  channels: number;
  timestamp: number;
  mediaProvider: string;
}
```

```ts
// packages/bot/src/media/listenTypes.ts
export type SFUProviderName =
  | "livekit"
  | "mediasoup"
  | "srs"
  | "agora"
  | "daily"
  | "cloudflare";

export interface SFUListenJoinParams {
  room: string;
  identity: string;
  token: string;
  serverUrl: string;
  provider: SFUProviderName;
  // provider extras from /signal/token
  stream?: string;
  streamToken?: string;
  clientInfo?: Record<string, unknown>;
  socket?: unknown; // mediasoup 需要
}

export interface SFUListenAdapter {
  readonly provider: SFUProviderName;
  join(params: SFUListenJoinParams): Promise<void>;
  leave(room: string): Promise<void>;
  onAudioFrame(cb: (frame: AudioFrameEvent) => void): void;
  onTrackEnded(cb: (info: { room: string; identity: string }) => void): void;
  listActiveIdentities(room: string): string[];
  dispose(): Promise<void>;
}
```

---

## File Structure

### Create
| File | Responsibility |
|------|----------------|
| `packages/bot/src/media/listenTypes.ts` | 旁听通用类型 |
| `packages/bot/src/media/listenRegistry.ts` | 用户指定房间集合（desired rooms） |
| `packages/bot/src/media/listenService.ts` | 对账 desired/active，驱动 join/leave |
| `packages/bot/src/media/sfuListenRouter.ts` | 按 provider 选择 adapter |
| `packages/bot/src/media/adapters/livekitListenAdapter.ts` | LiveKit 可运行旁听 |
| `packages/bot/src/media/adapters/unsupportedListenAdapter.ts` | 未实现 provider 的统一错误 |
| `packages/bot/src/media/adapters/index.ts` | adapter 导出 |
| `packages/bot/src/media/index.ts` | media 模块导出 |
| `packages/bot/src/media/pcmStream.ts` | 进程内 PCM 流实现（PcmStreamHub） |
| `packages/bot/src/media/pcmStream.test.ts` | 代码内 PCM 读接口测试 |
| `packages/bot/src/speech/types.ts` | SpeechPipeline 接口 |
| `packages/bot/src/speech/passthroughPipeline.ts` | 开发用/可测 pipeline（可产假 final 或仅透传钩子） |
| `packages/bot/src/speech/speechBusBridge.ts` | SpeechResult → EventBus SpeechEvent |
| `packages/bot/src/plugins/builtin/listen-manager/index.ts` | `/listen` 命令插件 |
| `packages/bot/src/media/listenRegistry.test.ts` | 房间集合测试 |
| `packages/bot/src/media/listenService.test.ts` | 对账逻辑测试 |
| `packages/bot/src/media/adapters/livekitListenAdapter.test.ts` | mock 帧回调测试 |
| `packages/bot/src/runtime/botRunner.listen.test.ts` | BotRunner 自动旁听接线测试 |
| `packages/bot/src/plugins/builtin/listen-manager/listen-manager.test.ts` | 命令测试 |

### Modify
| File | Responsibility |
|------|----------------|
| `packages/bot/src/core/types.ts` | Speech 事件 |
| `packages/bot/src/core/context.ts` | 暴露 `ctx.listen` |
| `packages/bot/src/runtime/botRunner.ts` | config + 自动旁听生命周期 |
| `packages/bot/src/runtime/apiClient.ts` | getSFUToken 补齐 provider 等字段 |
| `packages/bot/src/main.ts` | 读取 `GOSPEAK_LISTEN_ROOMS` |
| `packages/bot/src/plugins/builtin/index.ts` | 导出 listen-manager |
| `packages/bot/package.json` | LiveKit Node 依赖 |
| `packages/bot/.env.example` | 监听房间/开关变量 |
| `packages/bot/README.md` | 使用说明 |

---

## In-Process PCM Stream Interface

旁听帧通过代码接口暴露给同进程消费者（ASR / 录制 / 插件），**不是 HTTP API**。

| 入口 | 说明 |
|------|------|
| `PcmStream` | 只读接口：`subscribe` / `subscribeFiltered` / `open` |
| `PcmStreamReader` | 读取会话：`onFrame` + `async iterator` + `close` |
| `PcmStreamSink` | 写入接口：`publish(frame)` |
| `PcmStreamHub` | 默认实现，读写一体 |
| `runner.pcmStream` | BotRunner 暴露的只读入口 |
| `runner.pcmHub.publish` | 旁听 adapter 写入入口 |

---

## Config Surface

| 变量 / 字段 | 说明 | 默认 |
|-------------|------|------|
| `GOSPEAK_LISTEN_ROOMS` | 逗号分隔房间名 | 空 |
| `GOSPEAK_LISTEN_ENABLED` | 总开关 | `true`（有房间才实际工作） |
| `GOSPEAK_LISTEN_AUTO_SFU` | join 时是否自动 SFU 旁听 | `true` |
| `GOSPEAK_SPEECH_ENABLED` | 是否启用 speech 事件产出 | `true` |
| `BotConfig.listenRooms` | 代码配置房间列表 | `[]` |
| `BotConfig.listenEnabled` | 代码开关 | `true` |

---

## Tasks

### Task 1: 新增 Speech 事件类型

**Files:**
- Modify: `packages/bot/src/core/types.ts`
- Create: `packages/bot/src/core/speechEvents.test.ts`

- [ ] **Step 1: 写失败测试**

```ts
import { describe, expect, it } from "vitest";
import { EventType } from "./types";

describe("speech event types", () => {
  it("exposes partial and final speech events", () => {
    expect(EventType.OnSpeechPartial).toBe("OnSpeechPartial");
    expect(EventType.OnSpeechFinal).toBe("OnSpeechFinal");
  });
});
```

- [ ] **Step 2: 跑测确认失败**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/core/speechEvents.test.ts
```
Expected: FAIL

- [ ] **Step 3: 改 `types.ts`**

在 `EventType` 增加：
```ts
OnSpeechPartial = "OnSpeechPartial",
OnSpeechFinal = "OnSpeechFinal",
OnListenRoomChanged = "OnListenRoomChanged",
```

新增：
```ts
export interface SpeechEvent {
  eventType: EventType.OnSpeechPartial | EventType.OnSpeechFinal;
  room: RoomRef;
  speaker: MemberRef;
  text: string;
  isFinal: boolean;
  confidence?: number;
  language?: string;
  provider: string;
  mediaProvider?: string;
  timestamp: number;
}

export interface ListenRoomChangedEvent {
  eventType: EventType.OnListenRoomChanged;
  rooms: string[];
  action: "add" | "remove" | "set" | "clear";
  room?: string;
  timestamp: number;
}

export type BotEvent =
  | MessageEvent
  | RoomEvent
  | MemberStateEvent
  | SpeechEvent
  | ListenRoomChangedEvent
  | PluginErrorEvent
  | LifecycleEvent;
```

- [ ] **Step 4: 跑测通过并提交**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/core/speechEvents.test.ts
git add packages/bot/src/core/types.ts packages/bot/src/core/speechEvents.test.ts
git commit -m "feat(bot): add OnSpeechPartial/OnSpeechFinal event types"
```

---

### Task 2: ListenRoomRegistry（用户指定房间）

**Files:**
- Create: `packages/bot/src/media/listenTypes.ts`
- Create: `packages/bot/src/media/listenRegistry.ts`
- Create: `packages/bot/src/media/listenRegistry.test.ts`

- [ ] **Step 1: 写测试**

```ts
import { describe, expect, it } from "vitest";
import { ListenRoomRegistry } from "./listenRegistry";

describe("ListenRoomRegistry", () => {
  it("parses env rooms and supports add/remove/list/clear", () => {
    const reg = new ListenRoomRegistry({
      initialRooms: ListenRoomRegistry.parseRoomList("a, b,,a"),
    });
    expect(reg.list().sort()).toEqual(["a", "b"]);
    expect(reg.add("c")).toBe(true);
    expect(reg.add("c")).toBe(false); // already exists
    expect(reg.remove("a")).toBe(true);
    expect(reg.list().sort()).toEqual(["b", "c"]);
    reg.clear();
    expect(reg.list()).toEqual([]);
  });

  it("set replaces whole collection", () => {
    const reg = new ListenRoomRegistry({ initialRooms: ["x"] });
    reg.set(["y", "z"]);
    expect(reg.list().sort()).toEqual(["y", "z"]);
  });
});
```

- [ ] **Step 2: 实现 Registry**

```ts
export class ListenRoomRegistry {
  private rooms = new Set<string>();
  private listeners = new Set<(rooms: string[], change: ListenChange) => void>();

  constructor(opts?: { initialRooms?: string[] }) {
    for (const r of opts?.initialRooms ?? []) this.rooms.add(r);
  }

  static parseRoomList(raw?: string): string[] {
    if (!raw) return [];
    return [...new Set(raw.split(",").map((s) => s.trim()).filter(Boolean))];
  }

  list(): string[] { return [...this.rooms]; }
  has(room: string): boolean { return this.rooms.has(room); }

  add(room: string): boolean {
    const name = room.trim();
    if (!name || this.rooms.has(name)) return false;
    this.rooms.add(name);
    this.emit({ action: "add", room: name });
    return true;
  }

  remove(room: string): boolean {
    if (!this.rooms.delete(room)) return false;
    this.emit({ action: "remove", room });
    return true;
  }

  set(rooms: string[]): void {
    this.rooms = new Set(rooms.map((r) => r.trim()).filter(Boolean));
    this.emit({ action: "set" });
  }

  clear(): void {
    this.rooms.clear();
    this.emit({ action: "clear" });
  }

  onChange(cb: (rooms: string[], change: ListenChange) => void): () => void {
    this.listeners.add(cb);
    return () => this.listeners.delete(cb);
  }

  private emit(change: ListenChange) {
    const rooms = this.list();
    for (const cb of this.listeners) cb(rooms, change);
  }
}

export type ListenChange =
  | { action: "add"; room: string }
  | { action: "remove"; room: string }
  | { action: "set" }
  | { action: "clear" };
```

- [ ] **Step 3: 测试 + commit**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/media/listenRegistry.test.ts
git add packages/bot/src/media
git commit -m "feat(bot): add listen room registry for user-selected rooms"
```

---

### Task 3: 通用 SFU 旁听接口 + Router

**Files:**
- Create: `packages/bot/src/media/sfuListenRouter.ts`
- Create: `packages/bot/src/media/adapters/unsupportedListenAdapter.ts`
- Create: `packages/bot/src/media/adapters/index.ts`
- Create: `packages/bot/src/media/sfuListenRouter.test.ts`

- [ ] **Step 1: Unsupported adapter**

```ts
export class UnsupportedListenAdapter implements SFUListenAdapter {
  constructor(public readonly provider: SFUProviderName) {}
  async join(): Promise<void> {
    throw new Error(`SFU listen adapter not implemented: ${this.provider}`);
  }
  async leave(): Promise<void> {}
  onAudioFrame(): void {}
  onTrackEnded(): void {}
  listActiveIdentities(): string[] { return []; }
  async dispose(): Promise<void> {}
}
```

- [ ] **Step 2: Router**

```ts
export class SFUListenRouter {
  private adapters = new Map<SFUProviderName, SFUListenAdapter>();

  constructor(private factory: (p: SFUProviderName) => SFUListenAdapter) {}

  get(provider: SFUProviderName): SFUListenAdapter {
    let a = this.adapters.get(provider);
    if (!a) {
      a = this.factory(provider);
      this.adapters.set(provider, a);
    }
    return a;
  }

  async dispose(): Promise<void> {
    for (const a of this.adapters.values()) await a.dispose();
    this.adapters.clear();
  }
}
```

- [ ] **Step 3: 测试 router 缓存同一 provider 实例**

- [ ] **Step 4: commit**

```bash
git add packages/bot/src/media
git commit -m "feat(bot): add generic SFU listen router and adapter interface"
```

---

### Task 4: LiveKit Listen Adapter（首个可运行通用实现）

**Files:**
- Modify: `packages/bot/package.json`
- Create: `packages/bot/src/media/adapters/livekitListenAdapter.ts`
- Create: `packages/bot/src/media/adapters/livekitListenAdapter.test.ts`

- [ ] **Step 1: 依赖**

```bash
cd /Users/noelorin/GOSpeak/packages/bot
pnpm add @livekit/rtc-node
```

- [ ] **Step 2: 实现要点**
- 纯旁听：不 publish mic
- `join` 用 token/serverUrl
- 只订阅 audio track
- 输出 `AudioFrameEvent`（目标 16k mono PCM）
- 支持多 room Map
- 测试通过注入 `createRoom` mock，不真实连网

- [ ] **Step 3: factory 映射**

```ts
export function createListenAdapter(provider: SFUProviderName): SFUListenAdapter {
  switch (provider) {
    case "livekit":
      return new LiveKitListenAdapter();
    default:
      return new UnsupportedListenAdapter(provider);
  }
}
```

- [ ] **Step 4: 测试 + commit**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/media/adapters/livekitListenAdapter.test.ts
git add packages/bot/package.json packages/bot/src/media
git commit -m "feat(bot): implement LiveKit SFU listen adapter"
```

---

### Task 5: MediaListenService（desired/active 对账）

**Files:**
- Create: `packages/bot/src/media/listenService.ts`
- Create: `packages/bot/src/media/listenService.test.ts`
- Create: `packages/bot/src/media/index.ts`

- [ ] **Step 1: 服务职责**

```ts
export interface MediaListenServiceDeps {
  registry: ListenRoomRegistry;
  router: SFUListenRouter;
  getJoinInfo: (room: string) => Promise<SFUListenJoinParams>;
  signalingJoin: (room: string) => Promise<void>;
  signalingLeave: (room: string) => Promise<void> | void;
  onAudioFrame: (frame: AudioFrameEvent) => void | Promise<void>;
  logger: Logger;
}

export class MediaListenService {
  private active = new Set<string>();
  private started = false;

  async start(): Promise<void> {
    this.started = true;
    this.registry.onChange(() => { void this.reconcile(); });
    await this.reconcile();
  }

  async reconcile(): Promise<void> {
    if (!this.started) return;
    const desired = new Set(this.registry.list());
    for (const room of desired) {
      if (!this.active.has(room)) await this.activate(room);
    }
    for (const room of [...this.active]) {
      if (!desired.has(room)) await this.deactivate(room);
    }
  }

  private async activate(room: string): Promise<void> {
    await this.deps.signalingJoin(room);
    const info = await this.deps.getJoinInfo(room);
    const adapter = this.deps.router.get(info.provider);
    adapter.onAudioFrame((f) => void this.deps.onAudioFrame(f));
    await adapter.join(info);
    this.active.add(room);
  }

  private async deactivate(room: string): Promise<void> {
    // leave all adapters that may own the room; MVP: current provider only
    // 简化：记录 room->provider 映射后定向 leave
    this.active.delete(room);
    await this.deps.signalingLeave(room);
  }
}
```

实现时必须维护 `roomProvider = Map<string, SFUProviderName>`，leave 才能落到正确 adapter。

- [ ] **Step 2: 单测覆盖**
- add room → activate 1 次
- remove room → deactivate 1 次
- reconcile 幂等（重复调用不重复 join）
- provider unsupported → 记录错误，不影响其他房间

- [ ] **Step 3: commit**

```bash
git add packages/bot/src/media
git commit -m "feat(bot): add media listen service with room reconcile"
```

---

### Task 6: SpeechPipeline + EventBus 桥接

**Files:**
- Create: `packages/bot/src/speech/types.ts`
- Create: `packages/bot/src/speech/passthroughPipeline.ts`
- Create: `packages/bot/src/speech/speechBusBridge.ts`
- Create: `packages/bot/src/speech/speechBusBridge.test.ts`

> 本计划先用可插拔 pipeline。默认 `PassthroughSpeechPipeline`：
> - 开发模式可关闭文本产出
> - 或在测试中手动 `emitPartial/emitFinal`
> - 后续 ASR 只需实现同一 `SpeechPipeline` 接口

```ts
export interface SpeechPipeline {
  handleFrame(frame: AudioFrameEvent): Promise<void>;
  closeIdentity(room: string, identity: string): Promise<void>;
  closeRoom(room: string): Promise<void>;
  dispose(): Promise<void>;
  onPartial(cb: (e: Omit<SpeechEvent, "eventType" | "timestamp"> & { isFinal: false }) => void): void;
  onFinal(cb: (e: Omit<SpeechEvent, "eventType" | "timestamp"> & { isFinal: true }) => void): void;
}
```

`speechBusBridge` 把 partial/final 转成：
```ts
eventBus.emit({
  eventType: isFinal ? EventType.OnSpeechFinal : EventType.OnSpeechPartial,
  ...
  timestamp: Date.now(),
})
```

- [ ] **Step 1: 测试 bridge 映射**
- [ ] **Step 2: 实现 + commit**

```bash
git add packages/bot/src/speech
git commit -m "feat(bot): bridge speech pipeline results to EventBus"
```

---

### Task 7: 接入 BotRunner（入房自动旁听）

**Files:**
- Modify: `packages/bot/src/runtime/botRunner.ts`
- Modify: `packages/bot/src/runtime/apiClient.ts`
- Modify: `packages/bot/src/core/context.ts`
- Create: `packages/bot/src/runtime/botRunner.listen.test.ts`

- [ ] **Step 1: 扩展 BotConfig**

```ts
export interface BotConfig {
  // existing fields...
  listenEnabled?: boolean;
  listenRooms?: string[];
  speechEnabled?: boolean;
}
```

- [ ] **Step 2: 扩展 getSFUToken 类型**

```ts
Promise<{
  token: string;
  serverUrl: string;
  provider?: string;
  stream?: string;
  streamToken?: string;
  // 允许透传其余 clientInfo
  [k: string]: unknown;
}>
```

- [ ] **Step 3: start() 中初始化**

顺序：
1. 原有 auth/api/socket/plugins
2. 创建 `ListenRoomRegistry({ initialRooms: config.listenRooms ?? [] })`
3. 创建 `SFUListenRouter(createListenAdapter)`
4. 创建 `SpeechPipeline` + bridge 到 eventBus
5. 创建 `MediaListenService`
6. `await listenService.start()` → 自动旁听配置房间

- [ ] **Step 4: stop() 清理**
- `listenService.stop()/dispose`
- `speechPipeline.dispose`
- `router.dispose`

- [ ] **Step 5: 暴露 context API**

```ts
export interface ListenClient {
  list(): string[];
  add(room: string): Promise<boolean>;
  remove(room: string): Promise<boolean>;
  clear(): Promise<void>;
}

export interface BotContext {
  // existing...
  readonly listen: ListenClient;
}
```

- [ ] **Step 6: 保留手动 `joinRoom`**
- 手动 `rooms.join(name, { sfu: true })` 不必然加入监听集合
- **自动旁听只跟 registry 走**
- 若希望手动 join 也旁听，可在 opts 加 `{ listen?: boolean }`（默认 false），本计划实现该可选参数：

```ts
async joinRoom(roomName: string, opts?: { sfu?: boolean; listen?: boolean })
```

当 `opts.listen === true` 时 `registry.add(roomName)`。

- [ ] **Step 7: 单测**
- config 带 `listenRooms: ["lobby"]` → start 后 activate 被调用
- stop 后 deactivate/dispose 被调用

- [ ] **Step 8: commit**

```bash
git add packages/bot/src/runtime packages/bot/src/core/context.ts
git commit -m "feat(bot): auto-listen configured rooms in BotRunner"
```

---

### Task 8: main/env 读取用户指定房间

**Files:**
- Modify: `packages/bot/src/main.ts`
- Modify: `packages/bot/.env.example`

- [ ] **Step 1: main.ts**

```ts
const LISTEN_ROOMS = ListenRoomRegistry.parseRoomList(
  process.env.GOSPEAK_LISTEN_ROOMS,
);
const LISTEN_ENABLED = process.env.GOSPEAK_LISTEN_ENABLED !== "false";

const runner = new BotRunner({
  // ...
  listenEnabled: LISTEN_ENABLED,
  listenRooms: LISTEN_ROOMS,
  speechEnabled: process.env.GOSPEAK_SPEECH_ENABLED !== "false",
});
```

- [ ] **Step 2: `.env.example`**

```env
# 指定 bot 自动旁听的房间（逗号分隔）
GOSPEAK_LISTEN_ROOMS=lobby,team-alpha
GOSPEAK_LISTEN_ENABLED=true
GOSPEAK_SPEECH_ENABLED=true
```

- [ ] **Step 3: commit**

```bash
git add packages/bot/src/main.ts packages/bot/.env.example
git commit -m "feat(bot): load listen rooms from environment"
```

---

### Task 9: listen-manager 插件（运行时指定房间）

**Files:**
- Create: `packages/bot/src/plugins/builtin/listen-manager/index.ts`
- Create: `packages/bot/src/plugins/builtin/listen-manager/listen-manager.test.ts`
- Modify: `packages/bot/src/plugins/builtin/index.ts`

- [ ] **Step 1: 命令**

```text
/listen add <room>
/listen remove <room>
/listen list
/listen clear
```

实现：

```ts
@RegisterPlugin({
  name: "listen-manager",
  author: "gospeak",
  desc: "指定 bot 旁听房间",
  version: "1.0.0",
})
export class ListenManagerPlugin extends Plugin {
  @Command("listen", { desc: "管理旁听房间" })
  async onListen(event: MessageEvent): Promise<void> {
    const [sub, room] = event.rawCommand?.args ?? [];
    switch (sub) {
      case "add":
        if (!room) return void this.ctx.chat.reply(event, "用法: /listen add <room>");
        await this.ctx.listen.add(room);
        await this.ctx.chat.reply(event, `已开始旁听: ${room}`);
        break;
      case "remove":
        if (!room) return void this.ctx.chat.reply(event, "用法: /listen remove <room>");
        await this.ctx.listen.remove(room);
        await this.ctx.chat.reply(event, `已停止旁听: ${room}`);
        break;
      case "clear":
        await this.ctx.listen.clear();
        await this.ctx.chat.reply(event, "已清空旁听房间");
        break;
      case "list":
      default:
        await this.ctx.chat.reply(
          event,
          `旁听房间: ${this.ctx.listen.list().join(", ") || "(空)"}`,
        );
    }
  }
}
```

- [ ] **Step 2: 测试命令分支**
- [ ] **Step 3: 导出 + commit**

```bash
git add packages/bot/src/plugins/builtin
git commit -m "feat(bot): add /listen commands for user-selected rooms"
```

---

### Task 10: README 与验收

**Files:**
- Modify: `packages/bot/README.md`

- [ ] **Step 1: 文档必须覆盖**
1. Speech 事件说明（`OnSpeechPartial` / `OnSpeechFinal`）
2. 通用 SFU 旁听架构与当前支持矩阵
3. 指定监听房间三种方式（env / config / `/listen`）
4. BotRunner 自动旁听生命周期
5. 限制：非 LiveKit provider 当前会明确报 unsupported

支持矩阵（写入 README）：

| Provider | 旁听适配 | 状态 |
|----------|----------|------|
| livekit | LiveKitListenAdapter | 本计划实现 |
| mediasoup | 预留 | unsupported |
| srs | 预留 | unsupported |
| agora | 预留 | unsupported |
| daily | 预留 | unsupported |
| cloudflare | 预留 | unsupported |

- [ ] **Step 2: 手工验收清单**

1. `.env` 设 `GOSPEAK_LISTEN_ROOMS=lobby`
2. 启动 bot，日志显示 activate `lobby`
3. LiveKit 房间有人说话时，media 层有 frame 日志
4. 若 speech pipeline 接了可产文本实现，插件可收到 `OnSpeechFinal`
5. `/listen add room-2` 后自动旁听 room-2
6. `/listen remove lobby` 后停止 lobby 旁听
7. 停止 bot 后无残留连接

- [ ] **Step 3: 全量测试**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test
```

- [ ] **Step 4: commit**

```bash
git add packages/bot/README.md
git commit -m "docs(bot): document SFU listen rooms and speech events"
```

---

### Task 11（可选第二批）: 扩展更多 SFU adapter

> 不阻塞主计划合并。每个 adapter 独立 PR。

顺序建议：
1. MediaSoup（复用 bot socket + worker 旁路 consumer）
2. SRS / Cloudflare（WHEP 拉流解码）
3. Agora / Daily（官方 Node/能力评估）

每个 adapter 必须：
- 实现 `SFUListenAdapter`
- 输出统一 `AudioFrameEvent`
- 有 mock 单测
- 在 README 矩阵更新状态

---

## Error Handling

| 场景 | 行为 |
|------|------|
| listen disabled | 不创建 MediaListenService |
| 房间 token 失败 | 该房间标记 failed，不影响其他房间；可重试 |
| provider unsupported | warn + 跳过该房间媒体旁听（信令是否 join 可配置，默认 join 信令、跳过媒体） |
| adapter 中途断连 | 触发该房间 deactivate 后再 schedule reconcile |
| speech pipeline 抛错 | 捕获并打日志，不终止 listen loop |
| `/listen add` 重复 | 回复“已在旁听” |

## Testing Strategy

1. Registry 纯逻辑单测
2. ListenService reconcile 单测（mock adapter/signaling）
3. LiveKit adapter mock 帧单测
4. BotRunner 集成单测（mock service）
5. listen-manager 命令单测
6. 手工 LiveKit E2E

## Implementation Order

1. Task 1 Speech 事件
2. Task 2 房间 Registry
3. Task 3 通用 SFU 接口/Router
4. Task 4 LiveKit adapter
5. Task 5 ListenService
6. Task 6 Speech bridge
7. Task 7 BotRunner 接线
8. Task 8 env/main
9. Task 9 `/listen` 插件
10. Task 10 文档验收
11. Task 11 其他 SFU（可选）

## Self-Review

### 需求覆盖
- `OnSpeechPartial` / `OnSpeechFinal` → Task 1 + Task 6
- 接入 BotRunner 入房自动旁听 → Task 5 + Task 7
- 通用 SFU 旁听媒体 → Task 3/4/5（接口通用，LiveKit 先落地）
- 用户指定监听房间 → Task 2 + Task 8 + Task 9

### 与旧 ASR 大计划关系
- 本计划聚焦 **旁听与事件骨架**
- 完整本地/云 ASR provider 可在 `SpeechPipeline` 上继续扩展，不再阻塞旁听能力

### 无占位
- 关键接口、配置、命令、测试与提交点均已给出
- 非 LiveKit provider 明确为 unsupported + 扩展任务，不是模糊 TBD

---

## First Execution Command

```bash
cd /Users/noelorin/GOSpeak/packages/bot
pnpm test
```

然后从 Task 1 开始。
