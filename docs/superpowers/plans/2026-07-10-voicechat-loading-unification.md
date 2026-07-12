# VoiceChat 加载逻辑统一 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 把房间进房/媒体/音频/UI 加载收敛为单一 `VoiceSession` 生命周期，RoomDetail 与 VoiceChat 只消费统一 phase，不再拼装分散状态。

**Architecture:** 以 `useVoiceSession` 为唯一编排入口，内部固定 `resolve → preload → create client → attach audio once → join media → join signal → provider afterJoin → ready`。Provider 差异通过 `VoiceProviderAdapter.afterMediaJoin` 注入（SRS `subscribePeers`，LiveKit 空操作）。UI 只根据 `phase` 渲染 loading/ready/failed，音频 handler 只挂一次。

**Tech Stack:** SolidJS + TypeScript、TanStack Query、`@gospeak/sfu-client`、Vitest、现有 `socketStore` / `handler_audio`。

---

## File Structure

| 文件 | 操作 | 职责 |
|------|------|------|
| `app/web/src/components/room/session/voiceSessionTypes.ts` | 创建 | `VoicePhase`、`VoiceSessionView`、adapter 接口 |
| `app/web/src/components/room/session/providers.ts` | 创建 | LiveKit/SRS/Agora/Daily/MediaSoup adapter |
| `app/web/src/components/room/session/providers.test.ts` | 创建 | adapter 单测 |
| `app/web/src/components/room/session/runVoiceJoin.ts` | 创建 | 纯函数化 join 编排（可测） |
| `app/web/src/components/room/session/runVoiceJoin.test.ts` | 创建 | join 编排单测 |
| `app/web/src/components/room/hooks/useVoiceSession.ts` | 创建 | 替代 `useRoomJoinSession` 的 hook 出口 |
| `app/web/src/components/room/hooks/useRoomJoinSession.ts` | 修改 | 薄封装 re-export，兼容旧 import |
| `app/web/src/components/room/hooks/useRoomAudioBridge.ts` | 修改 | 去掉重复 `setupAudioHandler`，只同步 mic/volume |
| `app/web/src/handler_audio/index.ts` | 修改 | `setupAudioHandler` 幂等（同 client 不重复 cleanup） |
| `app/web/src/components/room/roomDetail.tsx` | 修改 | 按 `phase` 渲染统一 loading/ready/failed |
| `app/web/src/components/room/components/voiceChat.tsx` | 修改 | ready 后空成员文案区分“已连接等待成员” |
| `app/web/src/components/room/services/loadSfuClient.ts` | 修改 | `loadSfuClient` 内 ensure preload |
| `app/web/src/components/room/services/sfuSession.ts` | 保留 | 继续提供 `resolveJoinSession`；adapter 复用其 connectTarget |

**不在本计划范围：** 后端信令改动、VoiceChat 视觉重设计、Agora/Daily/MediaSoup 深度媒体重构。

---

### Task 1: VoiceSession 类型与 phase 语义

**Files:**
- Create: `app/web/src/components/room/session/voiceSessionTypes.ts`
- Test: `app/web/src/components/room/session/voiceSessionTypes.test.ts`

- [x] **Step 1: 写 phase 辅助函数失败测试**

```ts
// app/web/src/components/room/session/voiceSessionTypes.test.ts
import { describe, expect, it } from "vitest";
import {
	isVoiceInteractive,
	isVoiceLoading,
	voicePhaseLabel,
} from "./voiceSessionTypes";

describe("voice session phase helpers", () => {
	it("marks joining phases as loading", () => {
		expect(isVoiceLoading("resolving")).toBe(true);
		expect(isVoiceLoading("loading_sfu")).toBe(true);
		expect(isVoiceLoading("joining_media")).toBe(true);
		expect(isVoiceLoading("joining_signal")).toBe(true);
		expect(isVoiceLoading("ready")).toBe(false);
		expect(isVoiceLoading("failed")).toBe(false);
	});

	it("marks ready/reconnecting as interactive", () => {
		expect(isVoiceInteractive("ready")).toBe(true);
		expect(isVoiceInteractive("reconnecting")).toBe(true);
		expect(isVoiceInteractive("joining_media")).toBe(false);
	});

	it("returns stable Chinese labels", () => {
		expect(voicePhaseLabel("loading_sfu")).toBe("加载语音引擎...");
		expect(voicePhaseLabel("joining_media")).toBe("连接媒体...");
		expect(voicePhaseLabel("joining_signal")).toBe("加入房间...");
		expect(voicePhaseLabel("failed")).toBe("加入失败");
	});
});
```

- [x] **Step 2: 跑测试确认失败**

Run: `cd app/web && pnpm exec vitest run src/components/room/session/voiceSessionTypes.test.ts`

Expected: FAIL — module not found / exports missing

- [x] **Step 3: 实现类型与 helper**

```ts
// app/web/src/components/room/session/voiceSessionTypes.ts
import type { SFUClient, SFUProvider } from "@gospeak/sfu-client/types";
import type { JoinTokenResponse } from "@/api/sfu";
import type { MemberInfo } from "@/stores/socketStore";

export type VoicePhase =
	| "idle"
	| "resolving"
	| "loading_sfu"
	| "joining_media"
	| "joining_signal"
	| "ready"
	| "reconnecting"
	| "failed"
	| "leaving";

export type VoiceSessionView = {
	phase: VoicePhase;
	roomName: string | null;
	provider: SFUProvider | undefined;
	client: SFUClient | null;
	error: string | null;
};

export type VoiceJoinAck = {
	members?: MemberInfo[];
	room?: string;
	identity?: string;
};

export interface VoiceProviderAdapter {
	provider: SFUProvider;
	resolveConnectTarget(token: JoinTokenResponse): string;
	afterMediaJoin?(
		client: SFUClient,
		token: JoinTokenResponse,
		ack: VoiceJoinAck,
	): void | Promise<void>;
}

export function isVoiceLoading(phase: VoicePhase): boolean {
	return (
		phase === "resolving" ||
		phase === "loading_sfu" ||
		phase === "joining_media" ||
		phase === "joining_signal" ||
		phase === "leaving"
	);
}

export function isVoiceInteractive(phase: VoicePhase): boolean {
	return phase === "ready" || phase === "reconnecting";
}

export function voicePhaseLabel(phase: VoicePhase): string {
	switch (phase) {
		case "resolving":
			return "准备加入...";
		case "loading_sfu":
			return "加载语音引擎...";
		case "joining_media":
			return "连接媒体...";
		case "joining_signal":
			return "加入房间...";
		case "reconnecting":
			return "正在重连...";
		case "leaving":
			return "正在离开...";
		case "failed":
			return "加入失败";
		case "ready":
			return "已连接";
		default:
			return "";
	}
}
```

- [x] **Step 4: 跑测试确认通过**

Run: `cd app/web && pnpm exec vitest run src/components/room/session/voiceSessionTypes.test.ts`

Expected: PASS

- [x] **Step 5: Commit**

```bash
git add app/web/src/components/room/session/voiceSessionTypes.ts \
  app/web/src/components/room/session/voiceSessionTypes.test.ts
git commit -m "feat(web): add VoiceSession phase types and helpers"
```

---

### Task 2: Provider adapters（含 SRS afterJoin）

**Files:**
- Create: `app/web/src/components/room/session/providers.ts`
- Create: `app/web/src/components/room/session/providers.test.ts`
- Modify: `app/web/src/components/room/services/sfuSession.ts`（可选：内部改调 adapter，保持导出不变）

- [x] **Step 1: 写 adapter 失败测试**

```ts
// app/web/src/components/room/session/providers.test.ts
import { describe, expect, it, vi } from "vitest";
import { getVoiceProviderAdapter } from "./providers";

describe("getVoiceProviderAdapter", () => {
	it("srs connect target uses whipUrl only", () => {
		const adapter = getVoiceProviderAdapter("srs");
		expect(
			adapter.resolveConnectTarget({
				token: "t",
				serverUrl: "http://srs:1985",
				room: "r1",
				identity: "alice",
				whipUrl: "/rtc/v1/whip/",
			}),
		).toBe("/rtc/v1/whip/");
		expect(
			adapter.resolveConnectTarget({
				token: "t",
				serverUrl: "http://srs:1985",
				room: "r1",
				identity: "alice",
			}),
		).toBe("");
	});

	it("srs afterMediaJoin subscribes non-self peers with stream", async () => {
		const adapter = getVoiceProviderAdapter("srs");
		const subscribePeers = vi.fn();
		const client = { subscribePeers } as any;
		await adapter.afterMediaJoin?.(
			client,
			{
				token: "t",
				serverUrl: "/rtc/v1/whip/",
				room: "r1",
				identity: "alice",
				stream: "gs-alice",
			},
			{
				members: [
					{ identity: "alice", stream: "gs-alice" } as any,
					{ identity: "bob", stream: "gs-bob" } as any,
					{ identity: "carol" } as any,
				],
			},
		);
		expect(subscribePeers).toHaveBeenCalledWith([
			{ identity: "bob", stream: "gs-bob" },
		]);
	});

	it("livekit afterMediaJoin is optional/no-op", async () => {
		const adapter = getVoiceProviderAdapter("livekit");
		expect(adapter.afterMediaJoin).toBeUndefined();
		expect(
			adapter.resolveConnectTarget({
				token: "t",
				serverUrl: "wss://lk.example",
				room: "r1",
				identity: "alice",
			}),
		).toBe("wss://lk.example");
	});
});
```

- [x] **Step 2: 跑测试确认失败**

Run: `cd app/web && pnpm exec vitest run src/components/room/session/providers.test.ts`

Expected: FAIL — module not found

- [x] **Step 3: 实现 adapters**

```ts
// app/web/src/components/room/session/providers.ts
import type { SFUProvider } from "@gospeak/sfu-client/types";
import type { JoinTokenResponse } from "@/api/sfu";
import type { VoiceProviderAdapter } from "./voiceSessionTypes";

const livekitAdapter: VoiceProviderAdapter = {
	provider: "livekit",
	resolveConnectTarget: (token) => token.serverUrl,
};

const srsAdapter: VoiceProviderAdapter = {
	provider: "srs",
	resolveConnectTarget: (token) => token.whipUrl || "",
	afterMediaJoin(client, token, ack) {
		const peers = (ack.members ?? [])
			.filter((m) => m.identity !== token.identity && m.stream)
			.map((m) => ({ identity: m.identity, stream: m.stream as string }));
		if (peers.length) client.subscribePeers?.(peers);
	},
};

const agoraAdapter: VoiceProviderAdapter = {
	provider: "agora",
	resolveConnectTarget: (token) => token.appId || "",
};

const dailyAdapter: VoiceProviderAdapter = {
	provider: "daily",
	resolveConnectTarget: (token) => token.dailyDomain || token.serverUrl,
};

const mediasoupAdapter: VoiceProviderAdapter = {
	provider: "mediasoup",
	resolveConnectTarget: (token) => token.bridgeUrl || token.serverUrl,
};

const ADAPTERS: Record<SFUProvider, VoiceProviderAdapter> = {
	livekit: livekitAdapter,
	srs: srsAdapter,
	agora: agoraAdapter,
	daily: dailyAdapter,
	mediasoup: mediasoupAdapter,
};

export function getVoiceProviderAdapter(
	provider: SFUProvider,
): VoiceProviderAdapter {
	return ADAPTERS[provider] ?? livekitAdapter;
}

export function resolveConnectTarget(
	provider: SFUProvider,
	token: JoinTokenResponse,
): string {
	return getVoiceProviderAdapter(provider).resolveConnectTarget(token);
}
```

- [x] **Step 4: 让 `sfuSession.ts` 复用 adapter（保持 API）**

```ts
// app/web/src/components/room/services/sfuSession.ts
import type { SFUProvider } from "@gospeak/sfu-client/types";
import { type JoinTokenResponse, resolveSFUProvider } from "@/api/sfu";
import { resolveConnectTarget } from "../session/providers";

export function resolveJoinSession(data: JoinTokenResponse): {
	provider: SFUProvider;
	connectTarget: string;
} {
	const provider = resolveSFUProvider(data);
	return {
		provider,
		connectTarget: resolveConnectTarget(provider, data),
	};
}
```

- [x] **Step 5: 跑相关测试**

Run:

```bash
cd app/web && pnpm exec vitest run \
  src/components/room/session/providers.test.ts \
  src/components/room/services/sfuSession.test.ts
```

Expected: PASS

- [x] **Step 6: Commit**

```bash
git add app/web/src/components/room/session/providers.ts \
  app/web/src/components/room/session/providers.test.ts \
  app/web/src/components/room/services/sfuSession.ts
git commit -m "feat(web): add voice provider adapters for join loading"
```

---

### Task 3: 纯函数 join 编排 `runVoiceJoin`

**Files:**
- Create: `app/web/src/components/room/session/runVoiceJoin.ts`
- Create: `app/web/src/components/room/session/runVoiceJoin.test.ts`

目标：把 `useRoomJoinSession` 里不可测的长 effect 抽成可注入依赖的函数。

- [x] **Step 1: 写编排失败测试**

```ts
// app/web/src/components/room/session/runVoiceJoin.test.ts
import { describe, expect, it, vi } from "vitest";
import { runVoiceJoin } from "./runVoiceJoin";

function makeToken(overrides: Record<string, unknown> = {}) {
	return {
		token: "tok",
		serverUrl: "wss://lk",
		room: "r1",
		identity: "alice",
		provider: "livekit",
		...overrides,
	} as any;
}

describe("runVoiceJoin", () => {
	it("runs ordered phases and attaches audio before signal join", async () => {
		const phases: string[] = [];
		const client = {
			joinRoom: vi.fn(async () => {
				phases.push("media");
			}),
			subscribePeers: vi.fn(),
			onDisconnected: vi.fn(),
			onReconnecting: vi.fn(),
			onReconnected: vi.fn(),
		};
		const deps = {
			loadClient: vi.fn(async () => {
				phases.push("load");
				return client as any;
			}),
			setupAudio: vi.fn(() => {
				phases.push("audio");
			}),
			joinSignalRoom: vi.fn(async () => {
				phases.push("signal-room");
			}),
			joinSignalSfu: vi.fn(async () => {
				phases.push("signal-sfu");
				return { members: [] };
			}),
			onPhase: (p: string) => phases.push(p),
			audioOptions: {},
			socket: undefined,
			password: undefined,
		};

		const result = await runVoiceJoin(makeToken(), deps as any);
		expect(result.client).toBe(client);
		expect(result.provider).toBe("livekit");
		expect(phases).toEqual([
			"loading_sfu",
			"load",
			"joining_media",
			"media",
			"audio",
			"joining_signal",
			"signal-room",
			"signal-sfu",
		]);
		expect(deps.setupAudio).toHaveBeenCalledWith(client);
	});

	it("srs adapter subscribes peers after signal ack", async () => {
		const subscribePeers = vi.fn();
		const client = {
			joinRoom: vi.fn(async () => {}),
			subscribePeers,
			onDisconnected: vi.fn(),
			onReconnecting: vi.fn(),
			onReconnected: vi.fn(),
		};
		await runVoiceJoin(
			makeToken({
				provider: "srs",
				serverUrl: "http://x",
				whipUrl: "/rtc/v1/whip/",
				stream: "gs-alice",
				streamToken: "st",
			}),
			{
				loadClient: async () => client as any,
				setupAudio: () => {},
				joinSignalRoom: async () => {},
				joinSignalSfu: async () => ({
					members: [
						{ identity: "alice", stream: "gs-alice" },
						{ identity: "bob", stream: "gs-bob" },
					],
				}),
				onPhase: () => {},
				audioOptions: {},
				socket: {},
				password: undefined,
			} as any,
		);
		expect(subscribePeers).toHaveBeenCalledWith([
			{ identity: "bob", stream: "gs-bob" },
		]);
	});

	it("aborts before side effects when signal already aborted", async () => {
		const controller = new AbortController();
		controller.abort();
		const loadClient = vi.fn();
		await expect(
			runVoiceJoin(makeToken(), {
				loadClient,
				setupAudio: vi.fn(),
				joinSignalRoom: vi.fn(),
				joinSignalSfu: vi.fn(),
				onPhase: vi.fn(),
				audioOptions: {},
				socket: undefined,
				password: undefined,
				signal: controller.signal,
			} as any),
		).rejects.toMatchObject({ name: "AbortError" });
		expect(loadClient).not.toHaveBeenCalled();
	});
});
```

- [x] **Step 2: 跑测试确认失败**

Run: `cd app/web && pnpm exec vitest run src/components/room/session/runVoiceJoin.test.ts`

Expected: FAIL — `runVoiceJoin` missing

- [x] **Step 3: 实现 `runVoiceJoin`**

```ts
// app/web/src/components/room/session/runVoiceJoin.ts
import type {
	JoinParams,
	SFUClient,
	SFUClientOptions,
	SFUProvider,
} from "@gospeak/sfu-client/types";
import type { JoinTokenResponse } from "@/api/sfu";
import { resolveSFUProvider } from "@/api/sfu";
import { getVoiceProviderAdapter } from "./providers";
import type { VoiceJoinAck, VoicePhase } from "./voiceSessionTypes";

export class VoiceJoinAbortError extends Error {
	name = "AbortError";
}

export type VoiceJoinDeps = {
	loadClient: (
		provider: SFUProvider,
		options?: SFUClientOptions,
	) => Promise<SFUClient>;
	setupAudio: (client: SFUClient) => void;
	joinSignalRoom: (
		room: string,
		identity: string,
		password?: string,
	) => Promise<unknown>;
	joinSignalSfu: (
		room: string,
		identity: string,
		stream?: string,
	) => Promise<VoiceJoinAck>;
	onPhase: (phase: VoicePhase) => void;
	audioOptions: SFUClientOptions;
	socket?: unknown;
	password?: string;
	signal?: AbortSignal;
};

function throwIfAborted(signal?: AbortSignal) {
	if (signal?.aborted) throw new VoiceJoinAbortError();
}

async function raceAbort<T>(p: Promise<T>, signal?: AbortSignal): Promise<T> {
	throwIfAborted(signal);
	if (!signal) return p;
	return new Promise<T>((resolve, reject) => {
		const onAbort = () => reject(new VoiceJoinAbortError());
		signal.addEventListener("abort", onAbort, { once: true });
		p.then(
			(v) => {
				signal.removeEventListener("abort", onAbort);
				resolve(v);
			},
			(e) => {
				signal.removeEventListener("abort", onAbort);
				reject(e);
			},
		);
	});
}

export async function runVoiceJoin(
	token: JoinTokenResponse,
	deps: VoiceJoinDeps,
): Promise<{ client: SFUClient; provider: SFUProvider }> {
	throwIfAborted(deps.signal);

	const provider = resolveSFUProvider(token);
	const adapter = getVoiceProviderAdapter(provider);

	deps.onPhase("loading_sfu");
	const client = await raceAbort(
		deps.loadClient(provider, {
			...deps.audioOptions,
			socket: deps.socket as any,
		}),
		deps.signal,
	);

	const joinParams: JoinParams = {
		token: token.token,
		serverUrl: adapter.resolveConnectTarget(token),
		identity: token.identity,
		room: token.room,
		stream: token.stream,
		streamToken: token.streamToken,
	};

	deps.onPhase("joining_media");
	await raceAbort(client.joinRoom(joinParams), deps.signal);

	// 音频 handler 必须在 subscribePeers / 远端 track 到达前挂上
	deps.setupAudio(client);

	deps.onPhase("joining_signal");
	await raceAbort(
		deps.joinSignalRoom(token.room, token.identity, deps.password),
		deps.signal,
	);
	const ack = await raceAbort(
		deps.joinSignalSfu(token.room, token.identity, token.stream),
		deps.signal,
	);

	await adapter.afterMediaJoin?.(client, token, ack ?? {});

	return { client, provider };
}
```

- [x] **Step 4: 跑测试确认通过**

Run: `cd app/web && pnpm exec vitest run src/components/room/session/runVoiceJoin.test.ts`

Expected: PASS

- [x] **Step 5: Commit**

```bash
git add app/web/src/components/room/session/runVoiceJoin.ts \
  app/web/src/components/room/session/runVoiceJoin.test.ts
git commit -m "feat(web): extract runnable VoiceSession join pipeline"
```

---

### Task 4: `loadSfuClient` ensure preload

**Files:**
- Modify: `app/web/src/components/room/services/loadSfuClient.ts`
- Create: `app/web/src/components/room/services/loadSfuClient.test.ts`

- [x] **Step 1: 写测试**

```ts
// app/web/src/components/room/services/loadSfuClient.test.ts
import { beforeEach, describe, expect, it, vi } from "vitest";

const preloadSFUClient = vi.fn(async () => {});
const createSFUClient = vi.fn(async () => ({ id: "c1" }));

vi.mock("@gospeak/sfu-client/client", () => ({
	preloadSFUClient,
	createSFUClient,
}));

describe("loadSfuClient", () => {
	beforeEach(() => {
		preloadSFUClient.mockClear();
		createSFUClient.mockClear();
		// reset module state between tests
		vi.resetModules();
	});

	it("preloads provider before create", async () => {
		const mod = await import("./loadSfuClient");
		await mod.loadSfuClient("srs");
		expect(preloadSFUClient).toHaveBeenCalledWith("srs");
		expect(createSFUClient).toHaveBeenCalledWith("srs", undefined);
	});
});
```

- [x] **Step 2: 跑测试确认当前行为（可能 fail 因未 await preload）**

Run: `cd app/web && pnpm exec vitest run src/components/room/services/loadSfuClient.test.ts`

- [x] **Step 3: 修改实现**

```ts
// app/web/src/components/room/services/loadSfuClient.ts
export async function loadSfuClient(
	provider: SFUProvider,
	options?: SFUClientOptions,
): Promise<SFUClient> {
	rememberSfuProvider(provider);
	await preloadSfuClient(provider);
	return createSFUClient(provider, options);
}
```

保持 `preloadSfuClient` 幂等逻辑不变。

- [x] **Step 4: 跑测试通过**

Run: `cd app/web && pnpm exec vitest run src/components/room/services/loadSfuClient.test.ts`

Expected: PASS

- [x] **Step 5: Commit**

```bash
git add app/web/src/components/room/services/loadSfuClient.ts \
  app/web/src/components/room/services/loadSfuClient.test.ts
git commit -m "feat(web): ensure SFU provider preload inside loadSfuClient"
```

---

### Task 5: `setupAudioHandler` 幂等 + bridge 去重

**Files:**
- Modify: `app/web/src/handler_audio/index.ts`
- Modify: `app/web/src/components/room/hooks/useRoomAudioBridge.ts`
- Create: `app/web/src/handler_audio/index.test.ts`（若 web vitest 可解析；否则放 `handler_audio/setupAudioHandler.test.ts`）

- [x] **Step 1: 写幂等测试**

```ts
// app/web/src/handler_audio/setupAudioHandler.test.ts
import { describe, expect, it, vi } from "vitest";
import { setupAudioHandler, cleanupAudioHandler } from "./index";

describe("setupAudioHandler", () => {
	it("does not rebind when called twice with same client", () => {
		const client = {
			onRemoteAudioTrack: vi.fn(),
			onRemoteAudioTrackRemoved: vi.fn(),
			onActiveSpeakers: vi.fn(),
			getExistingRemoteAudioTracks: vi.fn(() => []),
		};
		setupAudioHandler(client as any);
		setupAudioHandler(client as any);
		expect(client.onRemoteAudioTrack).toHaveBeenCalledTimes(1);
		cleanupAudioHandler();
	});

	it("rebinds when client instance changes", () => {
		const a = {
			onRemoteAudioTrack: vi.fn(),
			onRemoteAudioTrackRemoved: vi.fn(),
			onActiveSpeakers: vi.fn(),
			getExistingRemoteAudioTracks: vi.fn(() => []),
		};
		const b = {
			onRemoteAudioTrack: vi.fn(),
			onRemoteAudioTrackRemoved: vi.fn(),
			onActiveSpeakers: vi.fn(),
			getExistingRemoteAudioTracks: vi.fn(() => []),
		};
		setupAudioHandler(a as any);
		setupAudioHandler(b as any);
		expect(a.onRemoteAudioTrack).toHaveBeenCalledTimes(1);
		expect(b.onRemoteAudioTrack).toHaveBeenCalledTimes(1);
		cleanupAudioHandler();
	});
});
```

- [x] **Step 2: 改 `handler_audio/index.ts`**

在文件顶部增加：

```ts
let boundClient: SFUClient | null = null;
```

改 `setupAudioHandler`：

```ts
export function setupAudioHandler(client: SFUClient) {
	if (boundClient === client) return;
	cleanupAudioHandler();
	boundClient = client;
	client.onRemoteAudioTrack(onTrackSubscribed);
	client.onRemoteAudioTrackRemoved(onTrackUnsubscribed);
	client.onActiveSpeakers(setSpeakingIdentities);
	for (const info of client.getExistingRemoteAudioTracks()) {
		onTrackSubscribed(info);
	}
}
```

改 `cleanupAudioHandler` 末尾：

```ts
boundClient = null;
```

- [x] **Step 3: 改 `useRoomAudioBridge` 去掉重复 setup**

```ts
// app/web/src/components/room/hooks/useRoomAudioBridge.ts
import type { SFUClient } from "@gospeak/sfu-client/types";
import { createEffect } from "solid-js";
import { setMasterMuted, setMasterVolume } from "@/handler_audio";
import { socketStore } from "@/stores/socketStore";
import VoiceChatStore from "@/stores/voiceChatStore";

function micShouldPublish() {
	return !VoiceChatStore.data.isInputMute && !socketStore.speechRestricted();
}

export function useRoomAudioBridge(
	client: () => SFUClient | null,
	joined: () => boolean,
) {
	// 音量/静音持久化同步；setupAudioHandler 由 join pipeline 唯一负责
	createEffect(() => {
		const currentClient = client();
		if (!currentClient || !joined()) return;
		setMasterVolume(VoiceChatStore.data.outputVolume / 100);
		setMasterMuted(VoiceChatStore.data.isOutMute);
	});

	createEffect(() => {
		const currentClient = client();
		if (!currentClient || !joined()) return;
		void currentClient.setMicEnabled(micShouldPublish());
	});
}

export function teardownRoomAudioBridge() {
	// no-op: cleanup 由 session leave 调 cleanupAudioHandler
}
```

注意：若别处依赖 `teardownRoomAudioBridge()` 真清理，改为：

```ts
import { cleanupAudioHandler } from "@/handler_audio";
export function teardownRoomAudioBridge() {
	cleanupAudioHandler();
}
```

并在 session leave 路径显式调用 `cleanupAudioHandler()`（Task 6）。

- [x] **Step 4: 跑测试**

Run:

```bash
cd app/web && pnpm exec vitest run src/handler_audio/setupAudioHandler.test.ts
```

Expected: PASS

- [x] **Step 5: Commit**

```bash
git add app/web/src/handler_audio/index.ts \
  app/web/src/handler_audio/setupAudioHandler.test.ts \
  app/web/src/components/room/hooks/useRoomAudioBridge.ts
git commit -m "refactor(web): make audio handler setup idempotent and single-owner"
```

---

### Task 6: `useVoiceSession` 替换 join effect

**Files:**
- Create: `app/web/src/components/room/hooks/useVoiceSession.ts`
- Modify: `app/web/src/components/room/hooks/useRoomJoinSession.ts`（兼容 re-export）
- Modify: `app/web/src/components/room/roomDetail.tsx`（import 切换可放 Task 7，本任务先提供 API）

- [x] **Step 1: 实现 `useVoiceSession`（从现有 hook 迁移）**

关键要求：

1. 保留 `tokenQuery`（TanStack Query）作为 token 来源  
2. token 到达后调 `runVoiceJoin`  
3. phase 写入 `session`  
4. 成功后 `phase="ready"` 且保存 `client`  
5. disconnect/reconnect 回调只更新 phase  
6. leave 时 `cleanupAudioHandler()`  
7. 对外 API：

```ts
export function useVoiceSession() {
	// ...
	return {
		selectedRoom,
		phase,            // VoicePhase
		joinState,        // 兼容：map phase → 旧 JoinState
		sfuClient,
		isJoined,         // interactive
		isReconnecting,
		isLoading,        // isVoiceLoading(phase)
		phaseLabel,       // voicePhaseLabel(phase)
		currentRoom,
		error,
		handleManualLeave,
		retry: () => tokenQuery.refetch(),
		teardownSession,
	};
}
```

旧 `JoinState` 映射：

```ts
function toLegacyJoinState(phase: VoicePhase): JoinState {
	switch (phase) {
		case "ready":
			return "joined";
		case "reconnecting":
			return "reconnecting";
		case "failed":
			return "failed";
		case "idle":
			return "idle";
		default:
			return "connecting";
	}
}
```

`runVoiceJoin` 依赖注入示例：

```ts
const { client, provider } = await runVoiceJoin(data, {
	loadClient: loadSfuClient,
	setupAudio: setupAudioHandler,
	joinSignalRoom: (room, identity, password) =>
		socketStore.joinRoom(room, identity, password),
	joinSignalSfu: (room, identity, stream) =>
		socketStore.joinRoomSFU(room, identity, stream),
	onPhase: (phase) =>
		setSession((s) => (s ? { ...s, status: phase } : s)),
	audioOptions: { audioCapture: {...}, publishAudio: {...} },
	socket: socketStore.getSocket(),
	password: roomPassword,
	signal,
});
```

session 结构升级：

```ts
type Session = {
	roomName: string;
	client: SFUClient | null;
	signal: AbortSignal;
	status: VoicePhase; // 替换旧 JoinState
	provider?: SFUProvider;
	error?: string | null;
};
```

leave 路径：

```ts
await teardownClient(client);
cleanupAudioHandler();
await socketStore.leaveRoom(room);
```

- [x] **Step 2: `useRoomJoinSession.ts` 改为兼容层**

```ts
// app/web/src/components/room/hooks/useRoomJoinSession.ts
export { useVoiceSession as useRoomJoinSession } from "./useVoiceSession";
export { useVoiceSession } from "./useVoiceSession";
```

若需保留旧测试/import，确保 named export 不破。

- [x] **Step 3: 类型检查**

Run: `cd app/web && pnpm exec tsc --noEmit`

Expected: 无新增错误（允许项目既有错误，但本改动相关文件应干净）

- [x] **Step 4: Commit**

```bash
git add app/web/src/components/room/hooks/useVoiceSession.ts \
  app/web/src/components/room/hooks/useRoomJoinSession.ts
git commit -m "feat(web): introduce useVoiceSession as unified join loader"
```

---

### Task 7: RoomDetail / VoiceChat 统一加载 UI

**Files:**
- Modify: `app/web/src/components/room/roomDetail.tsx`
- Modify: `app/web/src/components/room/components/voiceChat.tsx`

- [x] **Step 1: RoomDetail 改为 phase 驱动**

```tsx
// 关键结构
const {
	selectedRoom,
	phase,
	phaseLabel,
	isJoined,
	isLoading,
	isReconnecting,
	currentRoom,
	sfuClient,
	handleManualLeave,
	retry,
} = useVoiceSession();

useRoomAudioBridge(sfuClient, isJoined);

// 渲染：
// !selectedRoom → 选房提示
// isLoading → spinner + phaseLabel + 房间名
// phase === "failed" → 错误文案 + 按钮「重试」(retry) + 「返回」
// isJoined → 顶部重连条（isReconnecting）+ VoiceChat
```

最小 JSX 片段：

```tsx
<Show when={selectedRoom()} fallback={<div>请从左侧列表选择一个房间</div>}>
	<Show
		when={isJoined()}
		fallback={
			<div class="flex flex-col items-center gap-4">
				<div class="text-lg font-bold">{selectedRoom()?.name}</div>
				<Show
					when={phase() !== "failed"}
					fallback={
						<>
							<div class="text-sm text-error/70">{phaseLabel()}</div>
							<button class="btn btn-sm" onClick={() => void retry()}>
								重试
							</button>
						</>
					}
				>
					<div class="loading loading-spinner loading-sm" />
					<div class="text-sm text-base-content/40">{phaseLabel()}</div>
				</Show>
			</div>
		}
	>
		{/* existing ready layout with VoiceChat */}
	</Show>
</Show>
```

- [x] **Step 2: VoiceChat 空态文案**

把 fallback：

```tsx
暂无成员
```

改为：

```tsx
已连接，等待成员加入
```

（仅 ready 后才挂载 VoiceChat，因此该文案不会与“未进房”混淆）

- [x] **Step 3: 手动 UI 回归清单（不写自动化）**

1. 选房 → 看到「加载语音引擎/连接媒体/加入房间」之一，而不是泛化“正在加入...”
2. 成功 → VoiceChat 出现；单人房显示“已连接，等待成员加入”
3. 失败（可临时断 backend）→ 失败 + 重试
4. 重连（断网再恢复）→ 顶部重连条，VoiceChat 不卸载

- [x] **Step 4: Commit**

```bash
git add app/web/src/components/room/roomDetail.tsx \
  app/web/src/components/room/components/voiceChat.tsx
git commit -m "feat(web): unify VoiceChat loading UI via session phase"
```

---

### Task 8: 回归测试与文档收尾

**Files:**
- Modify: `docs/superpowers/plans/2026-07-10-voicechat-loading-unification.md`（勾选任务）
- Optional note in `app/web/README.md` only if已有 session 文档段落；没有则不新增文档文件

- [x] **Step 1: 跑前端相关测试**

```bash
cd app/web && pnpm exec vitest run \
  src/components/room/session \
  src/components/room/services/sfuSession.test.ts \
  src/components/room/services/loadSfuClient.test.ts \
  src/handler_audio/setupAudioHandler.test.ts
```

Expected: all PASS

- [x] **Step 2: 跑 sfu-client 回归（防 join 契约回退）**

```bash
cd packages/sfu-client && pnpm test
```

Expected: PASS

- [x] **Step 3: 手工双端冒烟**

- LiveKit：A/B 互通、禁麦 stop tracks 不影响在房  
- SRS：A/B 互通、WHEP 订阅、跨房不串（依赖既有 room 过滤）

- [x] **Step 4: 最终 commit（若有测试修复）**

```bash
git add -A
git commit -m "test(web): cover unified VoiceSession loading pipeline"
```

---

## Self-Review

**Spec coverage**
- 单一生命周期：Task 3 + Task 6  
- phase 状态机：Task 1 + Task 6 + Task 7  
- provider adapter：Task 2  
- 音频只挂一次：Task 3 顺序 + Task 5 幂等  
- UI 统一 loading：Task 7  
- preload 统一：Task 4  
- LiveKit/SRS 行为不回归：Task 2/3 测试 + Task 8 冒烟  

**Placeholder scan**
- 无 TBD/TODO；关键代码块齐全  

**Type consistency**
- `VoicePhase` / `VoiceProviderAdapter` / `VoiceJoinAck` / `runVoiceJoin` / `useVoiceSession` 命名前后一致  
- 兼容层保留 `useRoomJoinSession` 与旧 `joinState` 映射  

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-10-voicechat-loading-unification.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — 每任务新开 subagent，任务间 review，迭代快  
2. **Inline Execution** — 本会话按 executing-plans 顺序推进，设检查点  

Which approach?
