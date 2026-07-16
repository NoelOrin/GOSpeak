> **Execution order superseded by** [`2026-07-16-bot-platform-unified.md`](./2026-07-16-bot-platform-unified.md). Keep this file as detailed appendix; do not run in parallel as a second source of truth.

# Bot 实时 ASR（本地 + 云端）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 GOSpeak Bot 以静默旁听者身份加入语音房间，订阅实时音频流，通过统一 ASR Provider 抽象同时支持本地模型与云厂商流式识别，并把 partial/final 结果作为插件事件与房间字幕输出。

**Architecture:** 在现有 `packages/bot` 插件/事件体系上新增 **媒体旁听层 + 音频流水线 + ASR Provider 层**。Bot 先用 JWT 走现有 `/api/v1/signal/token` 入房；MVP 用 LiveKit Node SDK（`@livekit/rtc-node`）订阅远端音频并解码为 16k mono PCM；`ASRManager` 按房间/说话人建立流式会话，路由到 `local-http`（FunASR / SenseVoice / faster-whisper 等本地服务）或云端 Provider（Deepgram / Azure / 阿里云）；结果通过 `EventType.OnSpeechPartial|OnSpeechFinal` 分发给插件，并可选经 Socket.IO/聊天接口广播字幕。多 SFU 不在 MVP 强行统一媒体解码，先把 LiveKit 打通，其余 SFU 后续按同一 `MediaListener` 接口扩展。

**Tech Stack:** TypeScript (packages/bot), `@livekit/rtc-node`, WebSocket/HTTP streaming ASR, vitest, 现有 Socket.IO 信令 + JWT Bot 体系

---

## Scope / Non-Goals

### In Scope
- Bot 实时旁听 + 流式 ASR
- 本地模型接入（通过本地 HTTP/WebSocket ASR 服务）
- 云厂商流式 API 接入（至少 Deepgram 完整；Azure/阿里云按同一接口落地）
- partial/final 事件、字幕插件、配置与权限
- LiveKit 作为首个媒体后端

### Out of Scope（明确不做）
- 不重写服务端 SFU 抽象
- 不做完整说话人分离 diarization（仅按 SFU identity 分轨）
- 不做离线整段录音转写主路径（可后续扩展）
- 不在本计划内实现全部 6 个 SFU 的 Node 媒体订阅
- 不把 ASR 逻辑塞进 Go 后端主路径（识别在 bot 进程/sidecar）

---

## Architecture

```text
[房间成员麦克风]
      │
      ▼
   LiveKit SFU
      │  subscribe remote audio
      ▼
 MediaListener (packages/bot)
      │  PCM 16k mono frames + identity
      ▼
 AudioPipeline (VAD/分轨缓冲，可选)
      │
      ▼
 ASRManager ──► ASRProvider
                ├─ LocalHttpASRProvider   (FunASR/SenseVoice/faster-whisper)
                ├─ DeepgramASRProvider
                ├─ AzureASRProvider
                └─ AliyunASRProvider
      │
      ▼
 EventBus: OnSpeechPartial / OnSpeechFinal
      │
      ├─ asr-caption 插件 → 房间字幕
      ├─ keyword-reply / moderation 可订阅
      └─ 日志 / kv 纪要
```

### Key Interfaces

```ts
// 媒体旁听
interface MediaListener {
  join(params: MediaJoinParams): Promise<void>;
  leave(room: string): Promise<void>;
  onAudioFrame(cb: (frame: AudioFrameEvent) => void): void;
  onTrackEnded(cb: (info: { room: string; identity: string }) => void): void;
  dispose(): Promise<void>;
}

// ASR 统一抽象（本地/云端同接口）
interface ASRProvider {
  readonly name: string;
  createSession(opts: ASRSessionOptions): Promise<ASRSession>;
}

interface ASRSession {
  write(frame: AudioFrameEvent): Promise<void>;
  end(): Promise<void>;
  onPartial(cb: (r: SpeechResult) => void): void;
  onFinal(cb: (r: SpeechResult) => void): void;
  onError(cb: (err: Error) => void): void;
}
```

### Config Surface（环境变量）

| 变量 | 说明 | 默认 |
|------|------|------|
| `GOSPEAK_ASR_ENABLED` | 总开关 | `false` |
| `GOSPEAK_ASR_PROVIDER` | `local-http` / `deepgram` / `azure` / `aliyun` | `local-http` |
| `GOSPEAK_ASR_LANGUAGE` | BCP-47 / 简写 | `zh` |
| `GOSPEAK_ASR_SAMPLE_RATE` | PCM 采样率 | `16000` |
| `GOSPEAK_ASR_BROADCAST_CAPTION` | 是否广播字幕 | `true` |
| `GOSPEAK_ASR_LOCAL_URL` | 本地 ASR 服务地址 | `ws://127.0.0.1:10095` |
| `GOSPEAK_ASR_LOCAL_MODE` | `websocket` / `http-chunk` | `websocket` |
| `GOSPEAK_ASR_DEEPGRAM_API_KEY` | Deepgram key | — |
| `GOSPEAK_ASR_DEEPGRAM_MODEL` | 模型名 | `nova-2` |
| `GOSPEAK_ASR_AZURE_KEY` / `GOSPEAK_ASR_AZURE_REGION` | Azure STT | — |
| `GOSPEAK_ASR_ALIYUN_ACCESS_KEY` / `SECRET` / `APP_KEY` | 阿里云 | — |
| `GOSPEAK_MEDIA_PROVIDER` | 媒体旁听后端 | `livekit` |

---

## File Structure

### Create

| File | Responsibility |
|------|----------------|
| `packages/bot/src/media/types.ts` | `AudioFrameEvent` / `MediaJoinParams` / `MediaListener` |
| `packages/bot/src/media/livekitListener.ts` | LiveKit Node 旁听实现 |
| `packages/bot/src/media/index.ts` | media 导出 |
| `packages/bot/src/asr/types.ts` | ASR 类型、SpeechResult、事件 payload |
| `packages/bot/src/asr/provider.ts` | `ASRProvider` / `ASRSession` 接口 |
| `packages/bot/src/asr/manager.ts` | 房间级会话路由、identity 分轨、启停 |
| `packages/bot/src/asr/factory.ts` | 按配置创建 provider |
| `packages/bot/src/asr/providers/localHttp.ts` | 本地 FunASR/SenseVoice/faster-whisper |
| `packages/bot/src/asr/providers/deepgram.ts` | Deepgram 流式 |
| `packages/bot/src/asr/providers/azure.ts` | Azure 流式 |
| `packages/bot/src/asr/providers/aliyun.ts` | 阿里云流式 |
| `packages/bot/src/asr/index.ts` | asr 导出 |
| `packages/bot/src/plugins/builtin/asr-caption/index.ts` | 字幕/开关命令插件 |
| `packages/bot/src/media/livekitListener.test.ts` | 媒体旁听单测（mock room） |
| `packages/bot/src/asr/manager.test.ts` | 会话路由单测 |
| `packages/bot/src/asr/providers/localHttp.test.ts` | 本地 provider 协议单测 |
| `packages/bot/src/asr/providers/deepgram.test.ts` | Deepgram 帧/事件映射单测 |
| `packages/bot/src/plugins/builtin/asr-caption/asr-caption.test.ts` | 插件命令单测 |
| `docs/superpowers/specs/2026-07-16-bot-realtime-asr-design.md` | 设计摘要（可选同步） |

### Modify

| File | Responsibility |
|------|----------------|
| `packages/bot/package.json` | 加 `@livekit/rtc-node`、ws、相关类型依赖 |
| `packages/bot/src/core/types.ts` | 新增 speech 事件类型 |
| `packages/bot/src/core/context.ts` | 暴露 `ctx.asr` 可选能力 |
| `packages/bot/src/runtime/apiClient.ts` | `getSFUToken` 补 `provider` 字段类型 |
| `packages/bot/src/runtime/botRunner.ts` | 入房后启动 media+asr，离房清理 |
| `packages/bot/src/runtime/socketClient.ts` | 如需广播 `speech:*` 事件则加 emit helper |
| `packages/bot/src/plugins/builtin/index.ts` | 导出 asr-caption |
| `packages/bot/README.md` | 文档：ASR 配置与插件 |
| `packages/bot/.env.example` | ASR 环境变量 |
| `app/server/internal/model/permission.go` | Bot 白名单可加 `room:read` 已够用；如需独立权限再加 `asr:use`（本计划默认不强制服务端权限变更） |

---

## Phased Delivery

| Phase | 产出 | 可验证 |
|------|------|--------|
| P0 | 类型 + MediaListener 接口 + LiveKit 旁听 | bot 入房后能收到 PCM frame 日志 |
| P1 | ASR 抽象 + local-http + manager | 本地模型能吐 partial/final |
| P2 | Deepgram + Azure + Aliyun | 切换 env 即可换云端 |
| P3 | asr-caption 插件 + BotRunner 接线 | 房间出现实时字幕 |
| P4 | 测试/文档/降级策略 | CI 绿，README 可复现 |

---

## Tasks

### Task 1: 扩展事件类型与 SpeechResult

**Files:**
- Modify: `packages/bot/src/core/types.ts`
- Test: `packages/bot/src/bot.test.ts`（若有事件枚举断言则同步）

- [ ] **Step 1: 写失败测试（事件枚举存在）**

在 `packages/bot/src/asr/types.test.ts` 新建：

```ts
import { describe, expect, it } from "vitest";
import { EventType } from "../core/types";

describe("speech events", () => {
  it("defines partial and final speech events", () => {
    expect(EventType.OnSpeechPartial).toBe("OnSpeechPartial");
    expect(EventType.OnSpeechFinal).toBe("OnSpeechFinal");
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run:
```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/asr/types.test.ts
```
Expected: FAIL（`OnSpeechPartial` 不存在）

- [ ] **Step 3: 扩展 `EventType` 与事件 payload**

修改 `packages/bot/src/core/types.ts`：

```ts
export enum EventType {
  OnBotLoaded = "OnBotLoaded",
  AdapterMessage = "AdapterMessage",
  OnMessageReceived = "OnMessageReceived",
  OnMessageSent = "OnMessageSent",
  OnRoomCreated = "OnRoomCreated",
  OnRoomJoined = "OnRoomJoined",
  OnRoomUpdated = "OnRoomUpdated",
  OnRoomLeft = "OnRoomLeft",
  OnMemberStateChanged = "OnMemberStateChanged",
  OnSpeechPartial = "OnSpeechPartial",
  OnSpeechFinal = "OnSpeechFinal",
  OnPluginLoaded = "OnPluginLoaded",
  OnPluginUnloaded = "OnPluginUnloaded",
  OnPluginError = "OnPluginError",
}

export interface SpeechResult {
  room: string;
  identity: string;
  text: string;
  isFinal: boolean;
  confidence?: number;
  language?: string;
  provider: string;
  startedAt?: number;
  endedAt?: number;
}

export interface SpeechEvent {
  eventType: EventType.OnSpeechPartial | EventType.OnSpeechFinal;
  room: RoomRef;
  speaker: MemberRef;
  text: string;
  isFinal: boolean;
  confidence?: number;
  language?: string;
  provider: string;
  timestamp: number;
}

export type BotEvent =
  | MessageEvent
  | RoomEvent
  | MemberStateEvent
  | SpeechEvent
  | PluginErrorEvent
  | LifecycleEvent;
```

- [ ] **Step 4: 复跑测试**

Run:
```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/asr/types.test.ts
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add packages/bot/src/core/types.ts packages/bot/src/asr/types.test.ts
git commit -m "feat(bot): add speech event types for realtime ASR"
```

---

### Task 2: MediaListener 抽象 + LiveKit 旁听

**Files:**
- Create: `packages/bot/src/media/types.ts`
- Create: `packages/bot/src/media/livekitListener.ts`
- Create: `packages/bot/src/media/index.ts`
- Create: `packages/bot/src/media/livekitListener.test.ts`
- Modify: `packages/bot/package.json`

- [ ] **Step 1: 添加依赖**

```bash
cd /Users/noelorin/GOSpeak/packages/bot
pnpm add @livekit/rtc-node
pnpm add -D @types/node
```

- [ ] **Step 2: 定义媒体类型**

`packages/bot/src/media/types.ts`:

```ts
export interface MediaJoinParams {
  room: string;
  identity: string;
  token: string;
  serverUrl: string;
  provider?: string;
}

export interface AudioFrameEvent {
  room: string;
  identity: string;
  pcm16: Int16Array;
  sampleRate: number;
  channels: number;
  timestamp: number;
}

export interface MediaListener {
  join(params: MediaJoinParams): Promise<void>;
  leave(room: string): Promise<void>;
  onAudioFrame(cb: (frame: AudioFrameEvent) => void): void;
  onTrackEnded(cb: (info: { room: string; identity: string }) => void): void;
  listActiveIdentities(room: string): string[];
  dispose(): Promise<void>;
}
```

- [ ] **Step 3: 写 LiveKit listener 失败测试（mock Room 注入）**

`packages/bot/src/media/livekitListener.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import { LiveKitMediaListener } from "./livekitListener";

describe("LiveKitMediaListener", () => {
  it("emits pcm frames from subscribed audio track callback", async () => {
    const frames: string[] = [];
    const listener = new LiveKitMediaListener({
      targetSampleRate: 16000,
      // 测试注入：不真实连网
      createRoom: () => {
        return {
          on: vi.fn(),
          connect: vi.fn(async () => undefined),
          disconnect: vi.fn(async () => undefined),
          remoteParticipants: new Map(),
        } as any;
      },
    });

    listener.onAudioFrame((f) => frames.push(`${f.room}:${f.identity}:${f.pcm16.length}`));

    // 直接调用内部测试钩子
    (listener as any).handlePcmFrame({
      room: "lobby",
      identity: "alice",
      pcm16: new Int16Array([1, 2, 3, 4]),
      sampleRate: 16000,
      channels: 1,
      timestamp: Date.now(),
    });

    expect(frames[0]).toBe("lobby:alice:4");
  });
});
```

- [ ] **Step 4: 实现 `LiveKitMediaListener`**

要点：
- `Room.connect(serverUrl, token)`
- 监听 `TrackSubscribed`（audio only）
- 从 `AudioStream` / frame 回调拿到 Int16 PCM
- 若采样率不是 16k，做简单重采样（线性插值 MVP 可接受）
- 多房间用 `Map<room, Room>`
- 不 publish 本地 track（纯旁听）

`packages/bot/src/media/livekitListener.ts` 核心骨架：

```ts
import { Room, RoomEvent, Track, AudioStream } from "@livekit/rtc-node";
import type { AudioFrameEvent, MediaJoinParams, MediaListener } from "./types";

export interface LiveKitMediaListenerOptions {
  targetSampleRate?: number;
  createRoom?: () => Room;
}

export class LiveKitMediaListener implements MediaListener {
  private rooms = new Map<string, Room>();
  private active = new Map<string, Set<string>>(); // room -> identities
  private frameCb?: (frame: AudioFrameEvent) => void;
  private endedCb?: (info: { room: string; identity: string }) => void;
  private opts: LiveKitMediaListenerOptions;

  constructor(opts: LiveKitMediaListenerOptions = {}) {
    this.opts = opts;
  }

  onAudioFrame(cb: (frame: AudioFrameEvent) => void): void {
    this.frameCb = cb;
  }

  onTrackEnded(cb: (info: { room: string; identity: string }) => void): void {
    this.endedCb = cb;
  }

  listActiveIdentities(room: string): string[] {
    return [...(this.active.get(room) ?? [])];
  }

  async join(params: MediaJoinParams): Promise<void> {
    if (this.rooms.has(params.room)) return;
    const room = this.opts.createRoom?.() ?? new Room();
    this.active.set(params.room, new Set());

    room.on(RoomEvent.TrackSubscribed, async (track, _pub, participant) => {
      if (track.kind !== Track.Kind.KIND_AUDIO) return;
      this.active.get(params.room)?.add(participant.identity);
      // 从 AudioStream 读帧，转 Int16 mono 16k，调用 handlePcmFrame
      // 实现时注意：track ended / participant disconnected 清理
    });

    room.on(RoomEvent.TrackUnsubscribed, (_track, _pub, participant) => {
      this.active.get(params.room)?.delete(participant.identity);
      this.endedCb?.({ room: params.room, identity: participant.identity });
    });

    await room.connect(params.serverUrl, params.token);
    this.rooms.set(params.room, room);
  }

  async leave(roomName: string): Promise<void> {
    const room = this.rooms.get(roomName);
    if (!room) return;
    await room.disconnect();
    this.rooms.delete(roomName);
    this.active.delete(roomName);
  }

  async dispose(): Promise<void> {
    for (const name of [...this.rooms.keys()]) {
      await this.leave(name);
    }
  }

  /** @internal test hook */
  handlePcmFrame(frame: AudioFrameEvent): void {
    this.frameCb?.(frame);
  }
}
```

> 实现细节以 `@livekit/rtc-node` 当前 API 为准：优先使用官方 `AudioStream` 迭代 PCM；若 API 字段名有差异，以包内类型定义为准，不改接口语义。

- [ ] **Step 5: 导出**

`packages/bot/src/media/index.ts`:

```ts
export * from "./types";
export * from "./livekitListener";
```

- [ ] **Step 6: 跑单测**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/media/livekitListener.test.ts
```
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add packages/bot/package.json packages/bot/pnpm-lock.yaml packages/bot/src/media
git commit -m "feat(bot): add LiveKit media listener for realtime ASR"
```

---

### Task 3: ASR Provider 抽象 + Manager

**Files:**
- Create: `packages/bot/src/asr/types.ts`
- Create: `packages/bot/src/asr/provider.ts`
- Create: `packages/bot/src/asr/manager.ts`
- Create: `packages/bot/src/asr/manager.test.ts`
- Create: `packages/bot/src/asr/index.ts`

- [ ] **Step 1: 定义 provider 接口**

`packages/bot/src/asr/provider.ts`:

```ts
import type { AudioFrameEvent } from "../media/types";
import type { SpeechResult } from "../core/types";

export interface ASRSessionOptions {
  room: string;
  identity: string;
  language?: string;
  sampleRate: number;
}

export interface ASRSession {
  write(frame: AudioFrameEvent): Promise<void>;
  end(): Promise<void>;
  onPartial(cb: (r: SpeechResult) => void): void;
  onFinal(cb: (r: SpeechResult) => void): void;
  onError(cb: (err: Error) => void): void;
}

export interface ASRProvider {
  readonly name: string;
  createSession(opts: ASRSessionOptions): Promise<ASRSession>;
}
```

- [ ] **Step 2: 写 Manager 测试（按 identity 复用 session）**

```ts
import { describe, expect, it, vi } from "vitest";
import { ASRManager } from "./manager";
import type { ASRProvider, ASRSession } from "./provider";

function mockProvider(): ASRProvider {
  const sessions = new Map<string, ASRSession>();
  return {
    name: "mock",
    async createSession(opts) {
      const key = `${opts.room}:${opts.identity}`;
      const partials: Array<(r: any) => void> = [];
      const finals: Array<(r: any) => void> = [];
      const session: ASRSession = {
        write: vi.fn(async () => undefined),
        end: vi.fn(async () => undefined),
        onPartial: (cb) => partials.push(cb),
        onFinal: (cb) => finals.push(cb),
        onError: vi.fn(),
      };
      sessions.set(key, session);
      return session;
    },
  };
}

describe("ASRManager", () => {
  it("creates one session per room identity and forwards frames", async () => {
    const provider = mockProvider();
    const manager = new ASRManager({ provider, sampleRate: 16000, language: "zh" });
    const frame = {
      room: "lobby",
      identity: "alice",
      pcm16: new Int16Array(320),
      sampleRate: 16000,
      channels: 1,
      timestamp: Date.now(),
    };
    await manager.handleFrame(frame);
    await manager.handleFrame(frame);
    // 第二次不应再 createSession
    expect((provider as any)._created ?? 1).toBeTruthy();
  });
});
```

> 实现时在 mock provider 内用计数器断言 `createSession` 只调用 1 次。

- [ ] **Step 3: 实现 `ASRManager`**

`packages/bot/src/asr/manager.ts` 职责：
- key = `${room}::${identity}`
- `handleFrame`：无 session 则 create，并桥接 partial/final 到外部 callback
- `closeIdentity` / `closeRoom` / `dispose`
- 错误时结束该 session 并允许下次重建

```ts
export interface ASRManagerOptions {
  provider: ASRProvider;
  sampleRate: number;
  language?: string;
}

export class ASRManager {
  private sessions = new Map<string, ASRSession>();
  private opts: ASRManagerOptions;
  private partialCb?: (r: SpeechResult) => void;
  private finalCb?: (r: SpeechResult) => void;

  constructor(opts: ASRManagerOptions) {
    this.opts = opts;
  }

  onPartial(cb: (r: SpeechResult) => void) { this.partialCb = cb; }
  onFinal(cb: (r: SpeechResult) => void) { this.finalCb = cb; }

  private key(room: string, identity: string) {
    return `${room}::${identity}`;
  }

  async handleFrame(frame: AudioFrameEvent): Promise<void> {
    const k = this.key(frame.room, frame.identity);
    let session = this.sessions.get(k);
    if (!session) {
      session = await this.opts.provider.createSession({
        room: frame.room,
        identity: frame.identity,
        language: this.opts.language,
        sampleRate: this.opts.sampleRate,
      });
      session.onPartial((r) => this.partialCb?.(r));
      session.onFinal((r) => this.finalCb?.(r));
      session.onError(async () => {
        await this.closeIdentity(frame.room, frame.identity);
      });
      this.sessions.set(k, session);
    }
    await session.write(frame);
  }

  async closeIdentity(room: string, identity: string): Promise<void> {
    const k = this.key(room, identity);
    const s = this.sessions.get(k);
    if (!s) return;
    await s.end();
    this.sessions.delete(k);
  }

  async closeRoom(room: string): Promise<void> {
    const prefix = `${room}::`;
    for (const k of [...this.sessions.keys()]) {
      if (k.startsWith(prefix)) {
        await this.sessions.get(k)?.end();
        this.sessions.delete(k);
      }
    }
  }

  async dispose(): Promise<void> {
    for (const s of this.sessions.values()) await s.end();
    this.sessions.clear();
  }
}
```

- [ ] **Step 4: 跑测 + commit**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/asr/manager.test.ts
git add packages/bot/src/asr
git commit -m "feat(bot): add ASR manager and provider interfaces"
```

---

### Task 4: Local HTTP/WebSocket ASR Provider（本地模型）

**Files:**
- Create: `packages/bot/src/asr/providers/localHttp.ts`
- Create: `packages/bot/src/asr/providers/localHttp.test.ts`

**协议约定（本地 sidecar 统一适配）：**

Bot → Local ASR（WebSocket）
```json
{ "type": "start", "language": "zh", "sample_rate": 16000 }
// binary frames: raw PCM s16le mono
{ "type": "end" }
```

Local ASR → Bot
```json
{ "type": "partial", "text": "你好", "confidence": 0.8 }
{ "type": "final", "text": "你好世界", "confidence": 0.93 }
{ "type": "error", "message": "..." }
```

> FunASR runtime / SenseVoice / faster-whisper 服务若协议不同，在 sidecar 内适配到上述统一协议，避免 bot 侧为每个本地模型写分支。

- [ ] **Step 1: 写协议解析单测**

```ts
import { describe, expect, it } from "vitest";
import { parseLocalASRMessage } from "./localHttp";

describe("parseLocalASRMessage", () => {
  it("parses partial and final", () => {
    expect(parseLocalASRMessage('{"type":"partial","text":"hi"}')).toEqual({
      type: "partial",
      text: "hi",
    });
    expect(parseLocalASRMessage('{"type":"final","text":"hello"}').type).toBe("final");
  });
});
```

- [ ] **Step 2: 实现 `LocalHttpASRProvider`**

```ts
import WebSocket from "ws";
import type { ASRProvider, ASRSession, ASRSessionOptions } from "../provider";
import type { SpeechResult } from "../../core/types";

export function parseLocalASRMessage(raw: string): { type: string; text?: string; confidence?: number; message?: string } {
  return JSON.parse(raw);
}

export class LocalHttpASRProvider implements ASRProvider {
  readonly name = "local-http";
  constructor(private url: string) {}

  async createSession(opts: ASRSessionOptions): Promise<ASRSession> {
    const ws = new WebSocket(this.url);
    // wait open
    // send start
    // write(): send binary pcm
    // on message -> partial/final callbacks
    // end(): send end + close
    throw new Error("implement");
  }
}
```

实现要求：
- 连接失败 → `onError`
- `write` 在 socket 未 open 时入队，open 后 flush
- `end` 幂等
- `SpeechResult.provider = "local-http"`

- [ ] **Step 3: 补依赖**

```bash
cd /Users/noelorin/GOSpeak/packages/bot
pnpm add ws
pnpm add -D @types/ws
```

- [ ] **Step 4: 测试 + commit**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/asr/providers/localHttp.test.ts
git add packages/bot/src/asr/providers packages/bot/package.json
git commit -m "feat(bot): add local-http ASR provider for FunASR/SenseVoice"
```

---

### Task 5: Deepgram 流式 Provider

**Files:**
- Create: `packages/bot/src/asr/providers/deepgram.ts`
- Create: `packages/bot/src/asr/providers/deepgram.test.ts`

- [ ] **Step 1: 映射测试**

Deepgram live 结果：
- `is_final=false` → partial
- `is_final=true` → final

```ts
import { describe, expect, it } from "vitest";
import { mapDeepgramMessage } from "./deepgram";

describe("mapDeepgramMessage", () => {
  it("maps interim and final transcripts", () => {
    const partial = mapDeepgramMessage({
      type: "Results",
      is_final: false,
      channel: { alternatives: [{ transcript: "ni hao", confidence: 0.7 }] },
    });
    expect(partial).toMatchObject({ type: "partial", text: "ni hao" });

    const fin = mapDeepgramMessage({
      type: "Results",
      is_final: true,
      channel: { alternatives: [{ transcript: "你好", confidence: 0.95 }] },
    });
    expect(fin).toMatchObject({ type: "final", text: "你好" });
  });
});
```

- [ ] **Step 2: 实现 WebSocket live session**

连接：
```
wss://api.deepgram.com/v1/listen?model=nova-2&encoding=linear16&sample_rate=16000&channels=1&interim_results=true&language=zh
```
Header: `Authorization: Token ${API_KEY}`

- binary: PCM frames
- close: 发送空 JSON `{"type":"CloseStream"}`（按 Deepgram 文档）

- [ ] **Step 3: 测试 + commit**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/asr/providers/deepgram.test.ts
git add packages/bot/src/asr/providers/deepgram.ts packages/bot/src/asr/providers/deepgram.test.ts
git commit -m "feat(bot): add Deepgram streaming ASR provider"
```

---

### Task 6: Azure + 阿里云 Provider

**Files:**
- Create: `packages/bot/src/asr/providers/azure.ts`
- Create: `packages/bot/src/asr/providers/aliyun.ts`
- Create: `packages/bot/src/asr/providers/azure.test.ts`
- Create: `packages/bot/src/asr/providers/aliyun.test.ts`

- [ ] **Step 1: Azure Speech 流式**

使用 Azure Cognitive Services Speech SDK **或** REST/WebSocket 协议二选一：
- 推荐：`microsoft-cognitiveservices-speech-sdk`（Node 可用）
- 会话级 push stream 写入 PCM
- `recognizing` → partial，`recognized` → final

- [ ] **Step 2: 阿里云 NLS 流式**

使用阿里云智能语音交互流式识别 WebSocket：
- 鉴权：AccessKey/AppKey 生成 token（或预置 token env）
- 发送 start 指令 + PCM binary
- 解析中间/最终结果帧

- [ ] **Step 3: 两者都实现同一 `ASRProvider` 接口**

不得把云厂商字段泄漏到 Manager/插件层。

- [ ] **Step 4: 单测至少覆盖消息映射函数**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/asr/providers/azure.test.ts src/asr/providers/aliyun.test.ts
git add packages/bot/src/asr/providers
git commit -m "feat(bot): add Azure and Aliyun streaming ASR providers"
```

---

### Task 7: Provider Factory + 配置加载

**Files:**
- Create: `packages/bot/src/asr/factory.ts`
- Create: `packages/bot/src/asr/config.ts`
- Create: `packages/bot/src/asr/factory.test.ts`
- Modify: `packages/bot/.env.example`
- Modify: `packages/bot/src/main.ts`（若从 env 组装 runner config）

- [ ] **Step 1: config 解析**

```ts
export type ASRProviderName = "local-http" | "deepgram" | "azure" | "aliyun";

export interface ASRConfig {
  enabled: boolean;
  provider: ASRProviderName;
  language: string;
  sampleRate: number;
  broadcastCaption: boolean;
  localUrl?: string;
  deepgramApiKey?: string;
  deepgramModel?: string;
  azureKey?: string;
  azureRegion?: string;
  aliyunAccessKey?: string;
  aliyunSecret?: string;
  aliyunAppKey?: string;
}

export function loadASRConfig(env: NodeJS.ProcessEnv = process.env): ASRConfig {
  return {
    enabled: env.GOSPEAK_ASR_ENABLED === "true",
    provider: (env.GOSPEAK_ASR_PROVIDER as ASRProviderName) || "local-http",
    language: env.GOSPEAK_ASR_LANGUAGE || "zh",
    sampleRate: Number(env.GOSPEAK_ASR_SAMPLE_RATE || 16000),
    broadcastCaption: env.GOSPEAK_ASR_BROADCAST_CAPTION !== "false",
    localUrl: env.GOSPEAK_ASR_LOCAL_URL || "ws://127.0.0.1:10095",
    deepgramApiKey: env.GOSPEAK_ASR_DEEPGRAM_API_KEY,
    deepgramModel: env.GOSPEAK_ASR_DEEPGRAM_MODEL || "nova-2",
    azureKey: env.GOSPEAK_ASR_AZURE_KEY,
    azureRegion: env.GOSPEAK_ASR_AZURE_REGION,
    aliyunAccessKey: env.GOSPEAK_ASR_ALIYUN_ACCESS_KEY,
    aliyunSecret: env.GOSPEAK_ASR_ALIYUN_SECRET,
    aliyunAppKey: env.GOSPEAK_ASR_ALIYUN_APP_KEY,
  };
}
```

- [ ] **Step 2: factory**

```ts
export function createASRProvider(cfg: ASRConfig): ASRProvider {
  switch (cfg.provider) {
    case "local-http":
      return new LocalHttpASRProvider(cfg.localUrl!);
    case "deepgram":
      if (!cfg.deepgramApiKey) throw new Error("GOSPEAK_ASR_DEEPGRAM_API_KEY required");
      return new DeepgramASRProvider({ apiKey: cfg.deepgramApiKey, model: cfg.deepgramModel! });
    case "azure":
      if (!cfg.azureKey || !cfg.azureRegion) throw new Error("Azure ASR config incomplete");
      return new AzureASRProvider({ key: cfg.azureKey, region: cfg.azureRegion });
    case "aliyun":
      return new AliyunASRProvider({
        accessKey: cfg.aliyunAccessKey!,
        secret: cfg.aliyunSecret!,
        appKey: cfg.aliyunAppKey!,
      });
    default:
      throw new Error(`unknown ASR provider: ${cfg.provider}`);
  }
}
```

- [ ] **Step 3: `.env.example` 补齐 ASR 变量**

- [ ] **Step 4: 测试 + commit**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/asr/factory.test.ts
git add packages/bot/src/asr packages/bot/.env.example
git commit -m "feat(bot): add ASR config and provider factory"
```

---

### Task 8: BotRunner 接线（入房旁听 → ASR → 事件）

**Files:**
- Modify: `packages/bot/src/runtime/botRunner.ts`
- Modify: `packages/bot/src/core/context.ts`
- Modify: `packages/bot/src/runtime/apiClient.ts`
- Create: `packages/bot/src/runtime/botRunner.asr.test.ts`

- [ ] **Step 1: 扩展 `getSFUToken` 返回类型**

```ts
async getSFUToken(
  room: string,
  identity: string,
): Promise<{ token: string; serverUrl: string; provider?: string; stream?: string }> {
  return this.request("POST", "/api/v1/signal/token", { room, identity });
}
```

- [ ] **Step 2: BotRunner 增加 ASR 生命周期**

在 `BotRunner` 中：
1. `start()` 时若 `asrConfig.enabled`：
   - 创建 `LiveKitMediaListener`
   - `createASRProvider` + `ASRManager`
   - `media.onAudioFrame -> asr.handleFrame`
   - `asr.onPartial/onFinal -> eventBus.emit(SpeechEvent)`
2. `joinRoom(name, { sfu: true })` 后：
   - 取 token/serverUrl/provider
   - **MVP 仅当 provider===livekit（或未返回时默认 livekit）才 media.join**
   - 其他 provider 打 warn 并跳过媒体旁听
3. `leaveRoom` / `stop`：
   - `asr.closeRoom` + `media.leave/dispose`

伪代码（写入 botRunner）：

```ts
if (this.asrEnabled) {
  const tokenInfo = await this.api.getSFUToken(roomName, identity);
  if (tokenInfo.provider && tokenInfo.provider !== "livekit") {
    this.logger.warn(`ASR media listener only supports livekit, got ${tokenInfo.provider}`);
  } else {
    await this.media.join({
      room: roomName,
      identity,
      token: tokenInfo.token,
      serverUrl: tokenInfo.serverUrl,
      provider: tokenInfo.provider ?? "livekit",
    });
  }
}
```

- [ ] **Step 3: 把 speech 事件送入现有 eventBus**

确保插件 `@On(EventType.OnSpeechFinal)` 可收到。

- [ ] **Step 4: context 可选暴露**

```ts
export interface ASRClient {
  isEnabled(): boolean;
  activeProvider(): string | null;
}

export interface BotContext {
  // ...existing
  readonly asr?: ASRClient;
}
```

- [ ] **Step 5: 单测用 mock media/asr，不断网**

- [ ] **Step 6: commit**

```bash
git add packages/bot/src/runtime packages/bot/src/core/context.ts
git commit -m "feat(bot): wire media listener and ASR into BotRunner"
```

---

### Task 9: asr-caption 内置插件

**Files:**
- Create: `packages/bot/src/plugins/builtin/asr-caption/index.ts`
- Create: `packages/bot/src/plugins/builtin/asr-caption/asr-caption.test.ts`
- Modify: `packages/bot/src/plugins/builtin/index.ts`

- [ ] **Step 1: 插件行为**

命令：
- `/asr on`：当前房间开启字幕广播（kv 记录）
- `/asr off`：关闭
- `/asr status`：显示 provider + 开关状态
- `/asr provider`：显示当前 provider（只读；切换走 env/重启，避免运行时热切换复杂度）

事件处理：
- 监听 `OnSpeechFinal`（默认只播 final，防刷屏）
- 可选配置 `broadcastPartial=false`
- 文案：`[字幕][alice] 你好世界`

```ts
@RegisterPlugin({
  name: "asr-caption",
  author: "gospeak",
  desc: "实时语音字幕",
  version: "1.0.0",
})
export class ASRCaptionPlugin extends Plugin {
  @Command("asr", { desc: "实时字幕开关" })
  async onAsr(event: MessageEvent): Promise<void> {
    const sub = event.rawCommand?.args[0] ?? "status";
    // on/off/status/provider
  }

  @On(EventType.OnSpeechFinal)
  async onFinal(event: SpeechEvent): Promise<void> {
    const enabled = await this.ctx.kv.get<boolean>(`asr:caption:${event.room.id}`);
    if (enabled === false) return;
    await this.ctx.chat.send(
      event.room.id,
      `[字幕][${event.speaker.identity}] ${event.text}`,
    );
  }
}
```

> 若项目聊天发送 API 尚未完全可用，可先 `logger.info` + 预留 `chat.send`；但计划要求优先走 `ctx.chat.send`。

- [ ] **Step 2: 导出插件**

修改 `packages/bot/src/plugins/builtin/index.ts` 导出 `ASRCaptionPlugin`。

- [ ] **Step 3: 测试 + commit**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test -- src/plugins/builtin/asr-caption/asr-caption.test.ts
git add packages/bot/src/plugins/builtin
git commit -m "feat(bot): add asr-caption plugin for realtime subtitles"
```

---

### Task 10: README / 运维说明 / 端到端验收清单

**Files:**
- Modify: `packages/bot/README.md`
- Modify: `packages/bot/.env.example`

- [ ] **Step 1: README 增加章节**

内容必须包含：
1. 架构图（文字版即可）
2. 本地模型 sidecar 启动示例
3. Deepgram / Azure / 阿里云 env 示例
4. 启动 bot 并加入房间的步骤
5. 已知限制：MVP 仅 LiveKit 媒体旁听；其余 SFU 后续扩展

本地 sidecar 示例（FunASR 风格）：

```bash
# 示例：自备 ASR 服务，暴露统一 WS 协议
# ws://127.0.0.1:10095
export GOSPEAK_ASR_ENABLED=true
export GOSPEAK_ASR_PROVIDER=local-http
export GOSPEAK_ASR_LOCAL_URL=ws://127.0.0.1:10095
export GOSPEAK_TOKEN=... # bot jwt
pnpm --filter @gospeak/bot start
```

云端示例：

```bash
export GOSPEAK_ASR_ENABLED=true
export GOSPEAK_ASR_PROVIDER=deepgram
export GOSPEAK_ASR_DEEPGRAM_API_KEY=dg_xxx
```

- [ ] **Step 2: 手工 E2E 验收清单（写入 README）**

1. 后端 LiveKit 可用，创建房间
2. 浏览器用户 A 入房并说话
3. Bot 携带 JWT 启动，`joinRoom(room, { sfu: true })`
4. Bot 日志出现 audio frame / partial / final
5. `/asr on` 后房间出现 `[字幕][...] ...`
6. 切换 `GOSPEAK_ASR_PROVIDER=deepgram` 重启后仍可识别
7. Bot leave/stop 后无泄漏 session（日志无持续 reconnect 报错）

- [ ] **Step 3: 全量 bot 测试**

```bash
cd /Users/noelorin/GOSpeak/packages/bot && pnpm test
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add packages/bot/README.md packages/bot/.env.example
git commit -m "docs(bot): document realtime ASR local and cloud setup"
```

---

### Task 11: 多 SFU 扩展预留（不实现完整，只留接口位）

**Files:**
- Create: `packages/bot/src/media/factory.ts`
- Modify: `packages/bot/src/media/index.ts`

- [ ] **Step 1: media factory**

```ts
export function createMediaListener(provider: string): MediaListener {
  switch (provider) {
    case "livekit":
      return new LiveKitMediaListener();
    default:
      throw new Error(`media listener not implemented for provider: ${provider}`);
  }
}
```

- [ ] **Step 2: 在 README 写后续扩展顺序**

1. LiveKit（本计划）
2. MediaSoup（旁路 consumer）
3. SRS/Cloudflare（WHEP）
4. Agora/Daily（官方 Node/Electron SDK 评估）

- [ ] **Step 3: Commit**

```bash
git add packages/bot/src/media/factory.ts packages/bot/README.md
git commit -m "chore(bot): reserve media listener factory for multi-SFU ASR"
```

---

## Error Handling & Degradation

| 场景 | 行为 |
|------|------|
| ASR disabled | 完全不创建 media/asr |
| SFU provider 非 livekit | 入房信令成功，跳过旁听并 warn |
| ASR provider 鉴权失败 | `OnPluginError`/logger.error，不打崩 bot |
| 单 identity session 断开 | 删 session，下一帧重建 |
| 本地 sidecar 挂掉 | 指数退避重连（上限 30s），期间丢帧可接受 |
| 云厂商限流 | 记录 error，该房间临时降级为仅日志 |
| Bot stop | 强制 dispose media + asr |

## Privacy / Safety Notes

- Bot 旁听等于可听见房间全部语音，创建 bot 时权限应最小化（现有 `room:read` 足够拿 token）
- 默认 final 字幕上屏；partial 默认不上屏
- 日志默认不落全文，仅 debug 级别打印识别文本
- 云厂商场景需在部署文档标明音频出域风险

## Testing Strategy

1. **单元测试优先**：协议映射、manager 会话路由、插件开关
2. **组件测试**：mock LiveKit Room / mock WS ASR
3. **手工 E2E**：LiveKit + local-http 一条龙；Deepgram 一条龙
4. 不把真实云 key 写入测试；全部 mock

## Implementation Order (agent)

1. Task 1 事件类型
2. Task 2 LiveKit media
3. Task 3 ASR manager
4. Task 4 local-http
5. Task 5 Deepgram
6. Task 6 Azure/Aliyun
7. Task 7 factory/config
8. Task 8 BotRunner 接线
9. Task 9 caption 插件
10. Task 10 docs/E2E
11. Task 11 multi-SFU 预留

## Self-Review

### Spec coverage
- 本地模型：Task 4 + Task 7
- 云厂商：Task 5/6 + Task 7
- 实时旁听：Task 2 + Task 8
- 插件事件/字幕：Task 1 + Task 9
- 配置文档：Task 10
- 多 SFU 边界：Task 11 + Non-Goals

### Placeholder scan
- 无 TBD；LiveKit AudioStream 细节以包 API 为准，但接口与验收标准已固定

### Type consistency
- `SpeechResult` / `SpeechEvent` / `ASRSession` 字段在 Task 1/3 统一
- `getSFUToken` 补 `provider` 与后端 `SignalHandler.GetJoinToken` 对齐

### Integration continuity
- Task 8 依赖 Task 2/3/7
- Task 9 依赖 Task 1/8
- 每一 Task 都可独立提交并有测试门槛

---

## Suggested First Execution Command

```bash
cd /Users/noelorin/GOSpeak/packages/bot
pnpm test
```

然后按 Task 1 开始实施。
