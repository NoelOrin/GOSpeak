# socketStore 拆分 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `app/web/src/stores/socketStore.ts`（约 803 行 God Store）拆成可独立测试的模块，同时保持现有 `socketStore.*` 公共 API 兼容，避免一次性改爆房间/语音主链路。

**Architecture:** 采用“先抽纯模块 + 薄 facade”策略。`socketStore` 继续作为唯一对外入口，内部改为组合 `tabLock` / `types` / `roomState` / `mediasoupSignal` / `providerReload`。第一阶段不强制迁移调用方；第二阶段再按需让 session/dashboard 直接依赖更窄模块。所有可测逻辑优先抽成纯函数，TDD 落地。

**Tech Stack:** SolidJS `createRoot` + `createSignal`/`createMemo` 单例 store、Socket.IO client 适配器（`app/web/src/socket/client.ts`）、BroadcastChannel 多标签锁、Vitest、Biome、TypeScript。

---

## 背景与约束

### 当前问题

`app/web/src/stores/socketStore.ts` 同时承担：

1. 多标签页 socket 独占锁（BroadcastChannel）
2. Socket 连接生命周期
3. 房间/成员状态 reduce 与派生
4. join/leave/kick/list 信令 API
5. MediaSoup 传输/生产/消费信令
6. 禁言限制状态
7. activity/presence/kicked 订阅总线
8. SFU 热切换强制刷新
9. 类型定义 + `EVENTS` re-export
10. 直接写 `handler_audio/speakingStore`（store → audio 耦合）

### 兼容硬约束

以下调用方必须在第一阶段**零改或仅改 import 路径类型**：

- `components/room/hooks/useVoiceSession.ts`
- `components/room/session/providers.ts`
- `components/room/roomList.tsx`
- `components/room/roomDetail.tsx`
- `components/room/components/voiceChat.tsx`
- `components/room/components/memberSidebar.tsx`
- `components/room/components/passwordModal.tsx`
- `components/userBar.tsx`
- `components/dashboard/*`
- `components/modal/createRoomModal.tsx`

因此：

- 继续导出 `export const socketStore = ...`
- 继续导出 `MemberInfo` / `RoomInfo` / `ActivityEvent` / `RoomPresenceEvent` / `MuteEvent` / `UnmuteEvent`
- 继续 `export { EVENTS } from "@/socket/events"`（可标记 deprecated，但先别删）
- 方法名与返回值签名不变：`connect` / `joinRoom` / `leaveRoom` / `joinRoomSFU` / `getRouterCapabilities` / `produce` / `onActivity` 等

### 非目标（YAGNI）

- 不在本计划重写 Socket.IO 协议
- 不把 store 改成 Context
- 不强制 dashboard/room 立刻换新 import
- 不拆 `userStore` / `voiceChatStore`
- 不做多 tab 真正同时在线（产品仍是单 tab 持有 socket）

---

## 文件结构（锁定边界）

### 新增

| 文件 | 职责 |
|------|------|
| `app/web/src/socket/types.ts` | 信令领域类型：`MemberInfo`/`RoomInfo`/事件 payload |
| `app/web/src/socket/tabLock.ts` | BroadcastChannel 单 tab 锁：claim/release/foreign-claim |
| `app/web/src/socket/tabLock.test.ts` | tab 锁单元测试 |
| `app/web/src/socket/roomState.ts` | 纯函数：房间列表 merge、member shell upsert、ack members 覆盖 |
| `app/web/src/socket/roomState.test.ts` | 房间 reduce 测试 |
| `app/web/src/socket/providerReload.ts` | SFU provider 切换后的 toast + preload + reload |
| `app/web/src/socket/mediasoupSignal.ts` | MediaSoup 相关 emit/on 封装（依赖 socket adapter 接口） |
| `app/web/src/socket/mediasoupSignal.test.ts` | MediaSoup 封装测试 |

### 修改

| 文件 | 变化 |
|------|------|
| `app/web/src/stores/socketStore.ts` | 变为编排 facade：状态 + bindServerEvents + 对外 API |
| `app/web/src/handler_audio/speakingStore.ts` | 保持 setter；socketStore 继续调用，但计划末尾评估是否改为 listener 注入 |
| `app/web/docs/design/AGENTS-stores.md` | 更新 socketStore 模块边界说明 |
| 可选类型 re-export 兼容：`app/web/src/stores/socketStore.ts` 从 `socket/types` 再导出 |

### 保持不动

- `app/web/src/socket/client.ts`（已是不错的 transport 适配器）
- `app/web/src/socket/events.ts`
- 语音 session：`components/room/session/*`、`hooks/useVoiceSession.ts`（只做回归验证）

### 目标形态

```text
socket/
  client.ts            # transport adapter（已有）
  events.ts            # 事件名常量（已有）
  types.ts             # 领域类型
  tabLock.ts           # 多 tab 锁
  roomState.ts         # 纯状态变换
  providerReload.ts    # provider 热切换副作用
  mediasoupSignal.ts   # mediasoup 信令 helper

stores/
  socketStore.ts       # 组合以上模块，保留公共 API
```

`socketStore` 最终应主要包含：

- signals：`connected/rooms/currentRoom/selectedRoomInfo/...`
- `connect/disconnect`
- `bindServerEvents`（调用 `roomState` 纯函数）
- 薄封装的 room/mediasoup API
- `return { ...publicApi }`

目标体量：`socketStore.ts` 降到约 **250–350 行**。

---

## 公共 API 清单（拆完后必须仍存在）

```ts
socketStore.connected()
socketStore.rooms()
socketStore.currentRoom()
socketStore.members()
socketStore.selectedRoomInfo()
socketStore.activeSFUProvider()
socketStore.speechRestricted()
socketStore.speechRestrictionInfo()

socketStore.connect()
socketStore.disconnect()
socketStore.createRoom(name, password?)
socketStore.joinRoom(room, identity, password?)
socketStore.leaveRoom(room)
socketStore.joinRoomSFU(room, identity, stream?)
socketStore.clearCurrentRoom()
socketStore.getRouterCapabilities(room)
socketStore.createTransport(room, direction)
socketStore.connectTransport(room, transportId, dtlsParameters)
socketStore.produce(room, transportId, kind, rtpParameters, appData)
socketStore.consume(room, transportId, producerId, rtpCapabilities)
socketStore.onProducerReady(cb)
socketStore.onProducerClosed(cb)
socketStore.onActivity(cb)
socketStore.onPresence(cb)
socketStore.onRoomKicked(cb)
socketStore.getSocket()
socketStore.listRooms()
socketStore.kickMember(room, targetIdentity)
socketStore.emitMicState(room, identity, isMicMuted)
socketStore.emitSpeaking(room, identity, speaking)
socketStore.selectRoom(room)
socketStore.clearSelectedRoom()
socketStore.setCurrentSFUProvider(provider?)
```

类型：

```ts
export type {
  MemberInfo,
  RoomInfo,
  MuteEvent,
  UnmuteEvent,
  ActivityEvent,
  RoomPresenceEvent,
} from "@/socket/types";
// 或从 socketStore 再导出，保证旧 import 不坏
```

---

### Task 1: 抽出信令类型到 `socket/types.ts`

**Files:**
- Create: `app/web/src/socket/types.ts`
- Modify: `app/web/src/stores/socketStore.ts`
- Test: 无独立测试（类型搬运）；用 `tsc` 验证

- [ ] **Step 1: 新建类型文件**

把当前 `socketStore.ts` 中这些 interface 原样迁出：

```ts
// app/web/src/socket/types.ts
export interface MemberInfo {
	id: string;
	identity: string;
	name: string;
	displayName: string;
	avatar: string;
	isMuted: boolean;
	isMicMuted: boolean;
	joinedAt: number;
	stream?: string;
}

export interface RoomInfo {
	id: number;
	uuid: string;
	name: string;
	hasPassword: boolean;
	description?: string;
	limit: number;
	audioOnly?: boolean;
	allowAudience?: boolean;
	members: MemberInfo[];
	count: number;
	createdAt: number;
	/** @internal 临时传递密码，不从服务器获取 */
	_password?: string;
}

export interface MuteEvent {
	user_id: number;
	permanent: boolean;
	expires_at: string | null;
	reason: string;
}

export interface UnmuteEvent {
	user_id: number;
}

export interface ActivityEvent {
	type: "member_joined" | "member_left" | "room_joined" | "room_left";
	room: string;
	identity?: string;
	timestamp: number;
}

export interface RoomPresenceEvent {
	type: "member_joined" | "member_left";
	room: string;
	identity: string;
	timestamp: number;
}
```

- [ ] **Step 2: 在 `socketStore.ts` 改为 re-export**

删除本地 interface 定义，改为：

```ts
export type {
	MemberInfo,
	RoomInfo,
	MuteEvent,
	UnmuteEvent,
	ActivityEvent,
	RoomPresenceEvent,
} from "@/socket/types";

import type {
	ActivityEvent,
	MemberInfo,
	MuteEvent,
	RoomInfo,
	RoomPresenceEvent,
	UnmuteEvent,
} from "@/socket/types";
```

注意：旧代码是 `export interface ...`，很多文件写的是：

```ts
import type { MemberInfo } from "@/stores/socketStore";
```

必须继续能从 `@/stores/socketStore` 导入。用 `export type { ... }` 即可。

- [ ] **Step 3: 类型检查**

Run:

```bash
cd app/web && pnpm exec tsc --noEmit --pretty false
```

Expected: `EXIT 0`，无 `MemberInfo` / `RoomInfo` 找不到的错误。

- [ ] **Step 4: Commit**

```bash
git add app/web/src/socket/types.ts app/web/src/stores/socketStore.ts
git commit -m "$(cat <<'EOF'
refactor(web): extract socket signal types

Move MemberInfo/RoomInfo and related event payloads out of socketStore so later room-state helpers can share one type source without importing the store.
EOF
)"
```

---

### Task 2: 为 tab 锁写失败测试

**Files:**
- Create: `app/web/src/socket/tabLock.test.ts`
- Create later: `app/web/src/socket/tabLock.ts`

- [ ] **Step 1: 写失败测试（先不实现模块）**

```ts
// app/web/src/socket/tabLock.test.ts
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

class MockBroadcastChannel {
	static channels = new Map<string, Set<MockBroadcastChannel>>();
	name: string;
	onmessage: ((ev: MessageEvent) => void) | null = null;
	private listeners = new Set<(ev: MessageEvent) => void>();

	constructor(name: string) {
		this.name = name;
		const set = MockBroadcastChannel.channels.get(name) ?? new Set();
		set.add(this);
		MockBroadcastChannel.channels.set(name, set);
	}

	postMessage(data: unknown) {
		const peers = MockBroadcastChannel.channels.get(this.name) ?? new Set();
		for (const peer of peers) {
			if (peer === this) continue;
			const ev = { data } as MessageEvent;
			peer.onmessage?.(ev);
			for (const listener of peer.listeners) listener(ev);
		}
	}

	addEventListener(_type: "message", cb: (ev: MessageEvent) => void) {
		this.listeners.add(cb);
	}

	removeEventListener(_type: "message", cb: (ev: MessageEvent) => void) {
		this.listeners.delete(cb);
	}

	close() {
		MockBroadcastChannel.channels.get(this.name)?.delete(this);
	}

	static reset() {
		MockBroadcastChannel.channels.clear();
	}
}

describe("socket tabLock", () => {
	beforeEach(() => {
		MockBroadcastChannel.reset();
		vi.stubGlobal("BroadcastChannel", MockBroadcastChannel as any);
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
		MockBroadcastChannel.reset();
	});

	it("claims ownership when no other tab answers probe", async () => {
		const { createTabLock } = await import("./tabLock");
		const lock = createTabLock({
			channelName: "test_socket_tab",
			tabId: "tab-a",
			probeTimeoutMs: 150,
		});

		const pending = lock.claim();
		await vi.advanceTimersByTimeAsync(150);
		await expect(pending).resolves.toBe(true);
		expect(lock.isOwner()).toBe(true);
	});

	it("fails claim when another tab already owns the socket", async () => {
		const { createTabLock } = await import("./tabLock");
		const owner = createTabLock({
			channelName: "test_socket_tab",
			tabId: "tab-owner",
			probeTimeoutMs: 150,
		});
		// owner 先成功 claim
		const ownerPending = owner.claim();
		await vi.advanceTimersByTimeAsync(150);
		await expect(ownerPending).resolves.toBe(true);

		const challenger = createTabLock({
			channelName: "test_socket_tab",
			tabId: "tab-b",
			probeTimeoutMs: 150,
		});
		const pending = challenger.claim();
		// owner 会自动响应 probe -> finish(false)
		await vi.advanceTimersByTimeAsync(150);
		await expect(pending).resolves.toBe(false);
		expect(challenger.isOwner()).toBe(false);
	});

	it("notifies current owner when a foreign claimed message arrives", async () => {
		const { createTabLock } = await import("./tabLock");
		const onForeignClaim = vi.fn();
		const owner = createTabLock({
			channelName: "test_socket_tab",
			tabId: "tab-owner",
			probeTimeoutMs: 150,
			onForeignClaim,
		});
		const pending = owner.claim();
		await vi.advanceTimersByTimeAsync(150);
		await pending;

		// 模拟异常抢占
		const rogue = new MockBroadcastChannel("test_socket_tab");
		rogue.postMessage({ type: "claimed", from: "tab-rogue" });
		expect(onForeignClaim).toHaveBeenCalledTimes(1);
	});

	it("release clears ownership and broadcasts release", async () => {
		const { createTabLock } = await import("./tabLock");
		const lock = createTabLock({
			channelName: "test_socket_tab",
			tabId: "tab-a",
			probeTimeoutMs: 150,
		});
		const pending = lock.claim();
		await vi.advanceTimersByTimeAsync(150);
		await pending;

		const seen: unknown[] = [];
		const watcher = new MockBroadcastChannel("test_socket_tab");
		watcher.onmessage = (ev) => seen.push(ev.data);

		lock.release();
		expect(lock.isOwner()).toBe(false);
		expect(seen).toContainEqual({ type: "release", from: "tab-a" });
	});
});
```

- [ ] **Step 2: 跑测试，确认失败**

Run:

```bash
cd app/web && pnpm exec vitest run src/socket/tabLock.test.ts
```

Expected: FAIL，报错类似 `Failed to resolve import "./tabLock"` 或 `createTabLock is not a function`。

- [ ] **Step 3: Commit 测试**

```bash
git add app/web/src/socket/tabLock.test.ts
git commit -m "$(cat <<'EOF'
test(web): add failing tabLock coverage

Lock down single-tab socket ownership behavior before extracting BroadcastChannel logic from socketStore.
EOF
)"
```

---

### Task 3: 实现 `tabLock` 并让测试通过

**Files:**
- Create: `app/web/src/socket/tabLock.ts`
- Modify: `app/web/src/stores/socketStore.ts`
- Test: `app/web/src/socket/tabLock.test.ts`

- [ ] **Step 1: 实现 `createTabLock`**

```ts
// app/web/src/socket/tabLock.ts
export type TabMessage =
	| { type: "probe"; from: string }
	| { type: "owner"; from: string }
	| { type: "claimed"; from: string }
	| { type: "release"; from: string };

export type TabLockOptions = {
	channelName: string;
	tabId: string;
	probeTimeoutMs?: number;
	onForeignClaim?: () => void;
};

export type TabLock = {
	claim: () => Promise<boolean>;
	release: () => void;
	isOwner: () => boolean;
	/** 确保 channel 已建立，能响应 probe */
	ensureListening: () => void;
	setOnForeignClaim: (cb: (() => void) | null) => void;
};

export function createTabLock(options: TabLockOptions): TabLock {
	const probeTimeoutMs = options.probeTimeoutMs ?? 150;
	let channel: BroadcastChannel | null = null;
	let isOwner = false;
	let onForeignClaim = options.onForeignClaim ?? null;

	function getChannel(): BroadcastChannel | null {
		if (
			typeof window === "undefined" ||
			typeof BroadcastChannel === "undefined"
		) {
			return null;
		}
		if (!channel) {
			channel = new BroadcastChannel(options.channelName);
			channel.onmessage = (ev: MessageEvent<TabMessage>) => {
				const msg = ev.data;
				if (!msg || msg.from === options.tabId) return;
				if (msg.type === "probe" && isOwner) {
					channel?.postMessage({
						type: "owner",
						from: options.tabId,
					} satisfies TabMessage);
					return;
				}
				if (msg.type === "claimed" && isOwner) {
					onForeignClaim?.();
				}
			};
		}
		return channel;
	}

	async function claim(): Promise<boolean> {
		const ch = getChannel();
		if (!ch) {
			isOwner = true;
			return true;
		}
		if (isOwner) return true;

		return await new Promise<boolean>((resolve) => {
			let settled = false;
			const finish = (ok: boolean) => {
				if (settled) return;
				settled = true;
				ch.removeEventListener("message", onMsg as EventListener);
				window.clearTimeout(timer);
				if (ok) {
					isOwner = true;
					ch.postMessage({
						type: "claimed",
						from: options.tabId,
					} satisfies TabMessage);
				}
				resolve(ok);
			};

			const onMsg = (ev: MessageEvent<TabMessage>) => {
				const msg = ev.data;
				if (!msg || msg.from === options.tabId) return;
				if (msg.type === "owner" || msg.type === "claimed") {
					finish(false);
				}
			};

			ch.addEventListener("message", onMsg as EventListener);
			ch.postMessage({ type: "probe", from: options.tabId } satisfies TabMessage);
			const timer = window.setTimeout(() => finish(true), probeTimeoutMs);
		});
	}

	function release(): void {
		if (!isOwner) return;
		isOwner = false;
		try {
			getChannel()?.postMessage({
				type: "release",
				from: options.tabId,
			} satisfies TabMessage);
		} catch {
			// ignore
		}
	}

	return {
		claim,
		release,
		isOwner: () => isOwner,
		ensureListening: () => {
			getChannel();
		},
		setOnForeignClaim: (cb) => {
			onForeignClaim = cb;
		},
	};
}
```

- [ ] **Step 2: 跑测试通过**

Run:

```bash
cd app/web && pnpm exec vitest run src/socket/tabLock.test.ts
```

Expected: 全部 PASS。

- [ ] **Step 3: 替换 `socketStore` 内联 tab 锁**

删除 `socketStore.ts` 顶部这些实现：

- `TAB_CHANNEL_NAME` / `TAB_PROBE_TIMEOUT_MS` / `TAB_ID`
- `TabMessage`
- `getTabChannel` / `claimSocketTabLock` / `releaseSocketTabLock`
- 模块级 `isSocketTabOwner` / `onForeignClaim` / `tabChannel`

改为：

```ts
import { createTabLock } from "@/socket/tabLock";

const tabLock = createTabLock({
	channelName: "gospeak_socket_tab",
	tabId:
		typeof crypto !== "undefined" && "randomUUID" in crypto
			? crypto.randomUUID()
			: `tab-${Date.now()}-${Math.random().toString(36).slice(2)}`,
	probeTimeoutMs: 150,
});
```

`connect()` 中：

```ts
void tabLock.claim().then((ok) => {
  // 原 claimSocketTabLock 成功/失败分支保持不变
});
```

`disconnect()` / `beforeunload`：

```ts
tabLock.release();
```

foreign claim：

```ts
tabLock.setOnForeignClaim(() => {
  if (!adapter.isConnected() && !connecting()) return;
  showToast("连接已切换到其他标签页", { type: "warning" });
  disconnect();
});
tabLock.ensureListening();
```

- [ ] **Step 4: 回归**

Run:

```bash
cd app/web && pnpm exec vitest run src/socket/tabLock.test.ts && pnpm exec tsc --noEmit --pretty false
```

Expected: tests PASS，`tsc` EXIT 0。

- [ ] **Step 5: Commit**

```bash
git add app/web/src/socket/tabLock.ts app/web/src/socket/tabLock.test.ts app/web/src/stores/socketStore.ts
git commit -m "$(cat <<'EOF'
refactor(web): extract socket tab lock

Isolate BroadcastChannel single-tab ownership from socketStore and cover claim/release races with unit tests.
EOF
)"
```

---

### Task 4: 为房间状态纯函数写失败测试

**Files:**
- Create: `app/web/src/socket/roomState.test.ts`
- Create later: `app/web/src/socket/roomState.ts`

这些函数直接对应 `bindServerEvents` 里最容易回归的 reduce 逻辑。

- [ ] **Step 1: 写失败测试**

```ts
// app/web/src/socket/roomState.test.ts
import { describe, expect, it } from "vitest";
import type { MemberInfo, RoomInfo } from "./types";
import {
	applyMemberJoinedShell,
	applyMemberLeft,
	applyMemberUpdated,
	mergeRoomUpdated,
	upsertRoomMembersFromAck,
} from "./roomState";

const member = (identity: string, extra: Partial<MemberInfo> = {}): MemberInfo => ({
	id: identity,
	identity,
	name: identity,
	displayName: identity,
	avatar: "",
	isMuted: false,
	isMicMuted: false,
	joinedAt: 1,
	...extra,
});

const room = (name: string, members: MemberInfo[] = []): RoomInfo => ({
	id: 1,
	uuid: "u1",
	name,
	hasPassword: false,
	description: "desc",
	limit: 10,
	audioOnly: true,
	allowAudience: false,
	members,
	count: members.length,
	createdAt: 100,
});

describe("roomState reducers", () => {
	it("mergeRoomUpdated preserves DB fields and overwrites live fields", () => {
		const prev = [room("lobby", [member("a")])];
		const incoming = {
			name: "lobby",
			hasPassword: true,
			members: [member("a"), member("b")],
			count: 2,
			createdAt: 200,
			// 这些零值不应覆盖本地 DB 字段
			id: 0,
			uuid: "",
			description: undefined,
			limit: 0,
		} as RoomInfo;

		const next = mergeRoomUpdated(prev, incoming);
		expect(next[0].description).toBe("desc");
		expect(next[0].limit).toBe(10);
		expect(next[0].audioOnly).toBe(true);
		expect(next[0].hasPassword).toBe(true);
		expect(next[0].members.map((m) => m.identity)).toEqual(["a", "b"]);
		expect(next[0].count).toBe(2);
	});

	it("mergeRoomUpdated inserts missing room shell", () => {
		const next = mergeRoomUpdated([], {
			name: "new-room",
			hasPassword: false,
			members: [member("x")],
			count: 1,
		} as RoomInfo);
		expect(next).toHaveLength(1);
		expect(next[0].name).toBe("new-room");
		expect(next[0].members).toHaveLength(1);
	});

	it("applyMemberJoinedShell appends shell member without wiping existing ones", () => {
		const prev = [room("lobby", [member("a", { displayName: "Alice" })])];
		const next = applyMemberJoinedShell(prev, {
			room: "lobby",
			identity: "b",
			id: "id-b",
			stream: "s1",
		});
		expect(next[0].members.map((m) => m.identity)).toEqual(["a", "b"]);
		expect(next[0].members[0].displayName).toBe("Alice");
		expect(next[0].members[1].stream).toBe("s1");
		expect(next[0].count).toBe(2);
	});

	it("applyMemberJoinedShell is idempotent for same identity", () => {
		const prev = [room("lobby", [member("a")])];
		const once = applyMemberJoinedShell(prev, {
			room: "lobby",
			identity: "a",
			id: "id-a",
		});
		expect(once[0].members).toHaveLength(1);
	});

	it("applyMemberLeft removes member and updates count", () => {
		const prev = [room("lobby", [member("a"), member("b")])];
		const next = applyMemberLeft(prev, { room: "lobby", identity: "a" });
		expect(next[0].members.map((m) => m.identity)).toEqual(["b"]);
		expect(next[0].count).toBe(1);
	});

	it("applyMemberUpdated patches mic flags by identity", () => {
		const prev = [room("lobby", [member("a"), member("b")])];
		const next = applyMemberUpdated(prev, {
			room: "lobby",
			identity: "b",
			isMicMuted: true,
		});
		expect(next[0].members.find((m) => m.identity === "b")?.isMicMuted).toBe(
			true,
		);
		expect(next[0].members.find((m) => m.identity === "a")?.isMicMuted).toBe(
			false,
		);
	});

	it("upsertRoomMembersFromAck replaces members for room and inserts if missing", () => {
		const prev = [room("lobby", [member("old")])];
		const next = upsertRoomMembersFromAck(prev, "lobby", [
			member("new1"),
			member("new2"),
		]);
		expect(next[0].members.map((m) => m.identity)).toEqual(["new1", "new2"]);
		expect(next[0].count).toBe(2);

		const created = upsertRoomMembersFromAck([], "r2", [member("x")]);
		expect(created[0].name).toBe("r2");
		expect(created[0].members).toHaveLength(1);
	});
});
```

- [ ] **Step 2: 跑测试，确认失败**

Run:

```bash
cd app/web && pnpm exec vitest run src/socket/roomState.test.ts
```

Expected: FAIL（模块不存在或导不出函数）。

- [ ] **Step 3: Commit 测试**

```bash
git add app/web/src/socket/roomState.test.ts
git commit -m "$(cat <<'EOF'
test(web): add failing roomState reducer coverage

Capture room merge/member shell semantics before extracting them from socketStore event handlers.
EOF
)"
```

---

### Task 5: 实现 `roomState` 并替换 event handler 内联 reduce

**Files:**
- Create: `app/web/src/socket/roomState.ts`
- Modify: `app/web/src/stores/socketStore.ts`（`bindServerEvents` / `joinRoomSFU`）
- Test: `app/web/src/socket/roomState.test.ts`

- [ ] **Step 1: 实现纯函数**

```ts
// app/web/src/socket/roomState.ts
import type { MemberInfo, RoomInfo } from "./types";

export function mergeRoomUpdated(
	prev: RoomInfo[],
	room: RoomInfo,
): RoomInfo[] {
	if (!room?.name) return prev;
	const idx = prev.findIndex((r) => r.name === room.name);
	if (idx < 0) {
		return [
			...prev,
			{
				id: room.id ?? 0,
				uuid: room.uuid ?? "",
				name: room.name,
				hasPassword: room.hasPassword,
				description: room.description,
				limit: room.limit ?? 0,
				audioOnly: room.audioOnly,
				allowAudience: room.allowAudience,
				members: room.members ?? [],
				count: room.count ?? room.members?.length ?? 0,
				createdAt: room.createdAt ?? Date.now(),
			},
		];
	}
	return prev.map((r) =>
		r.name === room.name
			? {
					...r,
					name: room.name,
					hasPassword: room.hasPassword,
					members: room.members ?? [],
					count: room.count ?? room.members?.length ?? 0,
					createdAt: room.createdAt ?? r.createdAt,
				}
			: r,
	);
}

export function applyMemberJoinedShell(
	prev: RoomInfo[],
	data: { room: string; identity: string; id: string; stream?: string },
): RoomInfo[] {
	return prev.map((r) => {
		if (r.name !== data.room) return r;
		if (r.members.some((m) => m.identity === data.identity)) return r;
		const shell: MemberInfo = {
			id: data.id,
			identity: data.identity,
			name: data.identity,
			displayName: data.identity,
			avatar: "",
			isMuted: false,
			isMicMuted: false,
			joinedAt: Date.now(),
			stream: data.stream,
		};
		const members = [...r.members, shell];
		return { ...r, members, count: members.length };
	});
}

export function applyMemberLeft(
	prev: RoomInfo[],
	data: { room: string; identity: string },
): RoomInfo[] {
	return prev.map((r) => {
		if (r.name !== data.room) return r;
		const members = r.members.filter((m) => m.identity !== data.identity);
		return { ...r, members, count: members.length };
	});
}

export function applyMemberUpdated(
	prev: RoomInfo[],
	data: { room: string; identity: string; isMicMuted?: boolean; isMuted?: boolean },
): RoomInfo[] {
	return prev.map((r) => {
		if (r.name !== data.room) return r;
		return {
			...r,
			members: r.members.map((m) =>
				m.identity === data.identity
					? {
							...m,
							isMicMuted: data.isMicMuted ?? m.isMicMuted,
							isMuted: data.isMuted ?? m.isMuted,
						}
					: m,
			),
		};
	});
}

export function upsertRoomMembersFromAck(
	prev: RoomInfo[],
	roomName: string,
	ackMembers: MemberInfo[],
): RoomInfo[] {
	const exists = prev.some((r) => r.name === roomName);
	if (!exists) {
		return [
			...prev,
			{
				id: 0,
				uuid: "",
				name: roomName,
				hasPassword: false,
				limit: 0,
				members: ackMembers,
				count: ackMembers.length,
				createdAt: Date.now(),
			},
		];
	}
	return prev.map((r) =>
		r.name === roomName
			? { ...r, members: ackMembers, count: ackMembers.length }
			: r,
	);
}

export function addCreatedRoom(prev: RoomInfo[], room: RoomInfo): RoomInfo[] {
	if (prev.some((r) => r.name === room.name)) return prev;
	return [...prev, room];
}
```

> 实现时以**当前 `socketStore.ts` 真实分支**为准；上面是按现有语义整理的目标实现。若现有代码对 `member:joined` 的字段更丰富，把实际字段一并搬进 pure function，不要“简化掉”。

- [ ] **Step 2: 测试通过**

Run:

```bash
cd app/web && pnpm exec vitest run src/socket/roomState.test.ts
```

Expected: PASS。

- [ ] **Step 3: `bindServerEvents` 改为调用 pure functions**

示例：

```ts
adapter.onServerEvent(EVENTS.ROOM_UPDATED, (room: RoomInfo) => {
  if (!room?.name) return;
  setRooms((prev) => mergeRoomUpdated(prev, room));
});

adapter.onServerEvent(EVENTS.MEMBER_JOINED, (data) => {
  setRooms((prev) => applyMemberJoinedShell(prev, data));
  emitPresence({ type: "member_joined", room: data.room, identity: data.identity, timestamp: Date.now() });
  emitActivity({ type: "member_joined", room: data.room, identity: data.identity, timestamp: Date.now() });
});

// joinRoomSFU ack:
setRooms((prev) => upsertRoomMembersFromAck(prev, data.room, data.members));
```

**禁止**在这一步改事件语义、toast 文案、或 `leaveRoom` 清状态策略。

- [ ] **Step 4: 全量类型与相关测试**

Run:

```bash
cd app/web && pnpm exec vitest run src/socket/roomState.test.ts src/socket/tabLock.test.ts src/components/room/session && pnpm exec tsc --noEmit --pretty false
```

Expected: PASS + `tsc` 0。

- [ ] **Step 5: Commit**

```bash
git add app/web/src/socket/roomState.ts app/web/src/socket/roomState.test.ts app/web/src/stores/socketStore.ts
git commit -m "$(cat <<'EOF'
refactor(web): extract pure room state reducers

Move room/member list merge logic out of socketStore event handlers so membership races can be unit-tested without a live socket.
EOF
)"
```

---

### Task 6: 抽出 SFU provider 热切换副作用

**Files:**
- Create: `app/web/src/socket/providerReload.ts`
- Create: `app/web/src/socket/providerReload.test.ts`
- Modify: `app/web/src/stores/socketStore.ts`

- [ ] **Step 1: 写失败测试**

```ts
// app/web/src/socket/providerReload.test.ts
import { afterEach, describe, expect, it, vi } from "vitest";

describe("handleProviderChanged", () => {
	afterEach(() => {
		vi.useRealTimers();
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
	});

	it("toasts, preloads provider client, then reloads after 500ms", async () => {
		vi.useFakeTimers();
		const showToast = vi.fn();
		const preloadSfuClient = vi.fn().mockResolvedValue(undefined);
		const reload = vi.fn();
		vi.stubGlobal("location", { reload });

		const { createProviderReloadHandler } = await import("./providerReload");
		const handle = createProviderReloadHandler({
			showToast,
			preloadSfuClient,
			reload: () => reload(),
			delayMs: 500,
		});

		handle("livekit");
		expect(showToast).toHaveBeenCalled();
		expect(preloadSfuClient).toHaveBeenCalledWith("livekit");
		expect(reload).not.toHaveBeenCalled();

		await vi.advanceTimersByTimeAsync(500);
		expect(reload).toHaveBeenCalledTimes(1);
	});

	it("still reloads when preload fails", async () => {
		vi.useFakeTimers();
		const preloadSfuClient = vi.fn().mockRejectedValue(new Error("boom"));
		const reload = vi.fn();
		const { createProviderReloadHandler } = await import("./providerReload");
		const handle = createProviderReloadHandler({
			showToast: vi.fn(),
			preloadSfuClient,
			reload: () => reload(),
			delayMs: 500,
		});
		handle("srs");
		await vi.advanceTimersByTimeAsync(500);
		expect(reload).toHaveBeenCalledTimes(1);
	});
});
```

- [ ] **Step 2: 跑红**

```bash
cd app/web && pnpm exec vitest run src/socket/providerReload.test.ts
```

Expected: FAIL。

- [ ] **Step 3: 实现**

```ts
// app/web/src/socket/providerReload.ts
import type { SFUProvider } from "@gospeak/sfu-client/types";

export type ProviderReloadDeps = {
	showToast: (msg: string, opts?: { type?: string }) => void;
	preloadSfuClient: (provider: SFUProvider) => Promise<unknown>;
	reload?: () => void;
	delayMs?: number;
};

export function createProviderReloadHandler(deps: ProviderReloadDeps) {
	const delayMs = deps.delayMs ?? 500;
	const reload = deps.reload ?? (() => window.location.reload());

	return (provider?: string) => {
		console.log("[Socket] sfu:provider-changed", provider);
		deps.showToast("语音后端已切换，即将刷新页面", { type: "warning" });
		if (provider) {
			void deps.preloadSfuClient(provider as SFUProvider).catch(() => {});
		}
		window.setTimeout(() => {
			reload();
		}, delayMs);
	};
}
```

- [ ] **Step 4: `socketStore` 使用它**

```ts
import { preloadSfuClient } from "@/components/room/services/loadSfuClient";
import { createProviderReloadHandler } from "@/socket/providerReload";
import { showToast } from "solid-notifications";

const handleProviderChanged = createProviderReloadHandler({
	showToast,
	preloadSfuClient,
});
```

删除原 `handleProviderChanged` 函数体。

- [ ] **Step 5: 测试 + tsc**

```bash
cd app/web && pnpm exec vitest run src/socket/providerReload.test.ts && pnpm exec tsc --noEmit --pretty false
```

Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add app/web/src/socket/providerReload.ts app/web/src/socket/providerReload.test.ts app/web/src/stores/socketStore.ts
git commit -m "$(cat <<'EOF'
refactor(web): extract sfu provider reload side effects

Pull toast/preload/reload orchestration out of socketStore so hot-switch behavior can be tested without socket wiring.
EOF
)"
```

---

### Task 7: 抽出 MediaSoup 信令 helper

**Files:**
- Create: `app/web/src/socket/mediasoupSignal.ts`
- Create: `app/web/src/socket/mediasoupSignal.test.ts`
- Modify: `app/web/src/stores/socketStore.ts`

目标：把 `getRouterCapabilities/createTransport/connectTransport/produce/consume/onProducerReady/onProducerClosed` 从 store 核心挪走，但仍由 `socketStore` 暴露同名方法。

- [ ] **Step 1: 定义最小 adapter 接口并写测试**

```ts
// app/web/src/socket/mediasoupSignal.test.ts
import { describe, expect, it, vi } from "vitest";
import { createMediasoupSignal } from "./mediasoupSignal";
import { EVENTS } from "./events";

describe("createMediasoupSignal", () => {
	it("emits router/transport/produce/consume over ack helper", async () => {
		const emitAck = vi.fn(async (event: string, payload?: Record<string, unknown>) => ({
			event,
			payload,
		}));
		const onServerEvent = vi.fn(() => () => {});
		const api = createMediasoupSignal({ emitAck, onServerEvent });

		await api.getRouterCapabilities("r1");
		expect(emitAck).toHaveBeenCalledWith(EVENTS.SFU_GET_ROUTER_CAPABILITIES, {
			room: "r1",
		});

		await api.createTransport("r1", "send");
		expect(emitAck).toHaveBeenCalledWith(EVENTS.SFU_CREATE_TRANSPORT, {
			room: "r1",
			direction: "send",
		});

		await api.connectTransport("r1", "t1", { a: 1 });
		expect(emitAck).toHaveBeenCalledWith(EVENTS.SFU_CONNECT_TRANSPORT, {
			room: "r1",
			transportId: "t1",
			dtlsParameters: { a: 1 },
		});

		await api.produce("r1", "t1", "audio", { rtp: true }, { source: "mic" });
		expect(emitAck).toHaveBeenCalledWith(EVENTS.SFU_PRODUCE, {
			room: "r1",
			transportId: "t1",
			kind: "audio",
			rtpParameters: { rtp: true },
			appData: { source: "mic" },
		});

		await api.consume("r1", "t1", "p1", { caps: true });
		expect(emitAck).toHaveBeenCalledWith(EVENTS.SFU_CONSUME, {
			room: "r1",
			transportId: "t1",
			producerId: "p1",
			rtpCapabilities: { caps: true },
		});
	});

	it("fans out producer ready/closed listeners and supports unsubscribe", () => {
		const handlers = new Map<string, Function>();
		const onServerEvent = vi.fn((event: string, cb: Function) => {
			handlers.set(event, cb);
			return () => handlers.delete(event);
		});
		const api = createMediasoupSignal({
			emitAck: vi.fn(),
			onServerEvent: onServerEvent as any,
		});
		api.bindServerEvents();

		const ready = vi.fn();
		const closed = vi.fn();
		const offReady = api.onProducerReady(ready);
		const offClosed = api.onProducerClosed(closed);

		handlers.get(EVENTS.SFU_PRODUCER_READY)?.({ id: "p1" });
		handlers.get(EVENTS.SFU_PRODUCER_CLOSED)?.({ id: "p1" });
		expect(ready).toHaveBeenCalledWith({ id: "p1" });
		expect(closed).toHaveBeenCalledWith({ id: "p1" });

		offReady();
		offClosed();
		handlers.get(EVENTS.SFU_PRODUCER_READY)?.({ id: "p2" });
		handlers.get(EVENTS.SFU_PRODUCER_CLOSED)?.({ id: "p2" });
		expect(ready).toHaveBeenCalledTimes(1);
		expect(closed).toHaveBeenCalledTimes(1);
	});
});
```

- [ ] **Step 2: 跑红**

```bash
cd app/web && pnpm exec vitest run src/socket/mediasoupSignal.test.ts
```

Expected: FAIL。

- [ ] **Step 3: 实现 helper**

```ts
// app/web/src/socket/mediasoupSignal.ts
import { EVENTS } from "./events";

export type MediasoupSignalDeps = {
	emitAck: (event: string, payload?: Record<string, unknown>) => Promise<any>;
	onServerEvent: (event: string, cb: (...args: any[]) => void) => () => void;
};

export function createMediasoupSignal(deps: MediasoupSignalDeps) {
	const producerReadyListeners = new Set<(info: any) => void>();
	const producerClosedListeners = new Set<(info: any) => void>();

	function bindServerEvents() {
		deps.onServerEvent(EVENTS.SFU_PRODUCER_READY, (info: any) => {
			for (const listener of producerReadyListeners) listener(info);
		});
		deps.onServerEvent(EVENTS.SFU_PRODUCER_CLOSED, (info: any) => {
			for (const listener of producerClosedListeners) listener(info);
		});
	}

	function clearListeners() {
		producerReadyListeners.clear();
		producerClosedListeners.clear();
	}

	return {
		bindServerEvents,
		clearListeners,
		getRouterCapabilities(room: string) {
			return deps.emitAck(EVENTS.SFU_GET_ROUTER_CAPABILITIES, { room });
		},
		createTransport(room: string, direction: "send" | "recv") {
			return deps.emitAck(EVENTS.SFU_CREATE_TRANSPORT, { room, direction });
		},
		connectTransport(
			room: string,
			transportId: string,
			dtlsParameters: unknown,
		) {
			return deps.emitAck(EVENTS.SFU_CONNECT_TRANSPORT, {
				room,
				transportId,
				dtlsParameters,
			});
		},
		produce(
			room: string,
			transportId: string,
			kind: string,
			rtpParameters: unknown,
			appData: unknown,
		) {
			return deps.emitAck(EVENTS.SFU_PRODUCE, {
				room,
				transportId,
				kind,
				rtpParameters,
				appData,
			});
		},
		consume(
			room: string,
			transportId: string,
			producerId: string,
			rtpCapabilities: unknown,
		) {
			return deps.emitAck(EVENTS.SFU_CONSUME, {
				room,
				transportId,
				producerId,
				rtpCapabilities,
			});
		},
		onProducerReady(cb: (info: any) => void) {
			producerReadyListeners.add(cb);
			return () => {
				producerReadyListeners.delete(cb);
			};
		},
		onProducerClosed(cb: (info: any) => void) {
			producerClosedListeners.add(cb);
			return () => {
				producerClosedListeners.delete(cb);
			};
		},
	};
}
```

- [ ] **Step 4: 接入 `socketStore`**

在 `createRoot` 内：

```ts
const mediasoupSignal = createMediasoupSignal({
  emitAck: (event, payload) => signalEmit(event, payload),
  onServerEvent: (event, cb) => adapter.onServerEvent(event, cb),
});
```

- `bindServerEvents()` 末尾调用 `mediasoupSignal.bindServerEvents()`，删除原 producer ready/closed 手写监听
- `disconnect()` 调 `mediasoupSignal.clearListeners()`
- return 对象里 mediasoup 方法改为转发

- [ ] **Step 5: 测试**

```bash
cd app/web && pnpm exec vitest run src/socket/mediasoupSignal.test.ts src/socket/roomState.test.ts src/socket/tabLock.test.ts && pnpm exec tsc --noEmit --pretty false
```

Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add app/web/src/socket/mediasoupSignal.ts app/web/src/socket/mediasoupSignal.test.ts app/web/src/stores/socketStore.ts
git commit -m "$(cat <<'EOF'
refactor(web): extract mediasoup signal helpers

Keep socketStore public media methods stable while moving transport/produce/consume fan-out into a dedicated helper.
EOF
)"
```

---

### Task 8: 收敛 `socketStore` 结构与 speaking 耦合注释

**Files:**
- Modify: `app/web/src/stores/socketStore.ts`
- Modify: `app/web/docs/design/AGENTS-stores.md`
- Optional Modify: `docs/frontend-coupling-remaining.md`（若仍写 React/useEffect 旧方案，改成现状）

- [ ] **Step 1: 整理 `socketStore.ts` 分区顺序**

建议最终顺序（用注释分区，不改行为）：

```ts
// 1. imports + re-exports
// 2. tabLock / providerReload / mediasoupSignal / adapter 创建
// 3. signals
// 4. listener sets (activity/presence/kicked)
// 5. lifecycle callbacks (onConnected/onDisconnected/onConnectError)
// 6. connect / bindServerEvents / disconnect
// 7. room APIs (create/join/leave/list/kick/select)
// 8. mediasoup API forwarding
// 9. mic/speaking emits
// 10. return public API
```

确认 `socketStore.ts` 行数大致到 250–350。如果仍 > 450，检查是否还有大段 reduce/mediasoup/tab 逻辑没抽干净。

- [ ] **Step 2: speaking 耦合处理（最小改动）**

当前：

```ts
import { setSpeakingIdentities } from "@/handler_audio/speakingStore";
// ROOM_ACTIVE_SPEAKERS -> setSpeakingIdentities(...)
```

本计划**先保留**这条调用，但在 `bindServerEvents` 旁加注释：

```ts
// NOTE: store -> audio 写入是已知耦合；房间 UI 直接读 speakingStore。
// 若后续要彻底解耦，改为 onActiveSpeakers 订阅，由 useRoomAudioBridge/voiceChat 写入。
```

不要在本任务同时改 `voiceChat.tsx` 与 `speakingStore`，避免扩大回归面。

- [ ] **Step 3: 更新 stores 文档**

在 `app/web/docs/design/AGENTS-stores.md` 的 `socketStore.ts` 一节替换为：

```md
### socketStore.ts
实时信令编排 facade（不是所有逻辑的容器）。

内部组合：
- `socket/client.ts`：transport
- `socket/tabLock.ts`：单标签页独占
- `socket/roomState.ts`：房间/成员纯状态变换
- `socket/mediasoupSignal.ts`：MediaSoup 信令
- `socket/providerReload.ts`：SFU 热切换刷新
- `socket/types.ts`：领域类型

对外仍导出 `socketStore` 单例与类型，调用方默认继续从 `@/stores/socketStore` 导入。
```

- [ ] **Step 4: 校验**

```bash
cd app/web && pnpm test && pnpm exec tsc --noEmit --pretty false && pnpm exec biome check src/socket src/stores/socketStore.ts
```

Expected:

- vitest 全绿（至少新增 socket 测试 + 既有 room/session 测试）
- tsc 0
- biome 无 error（可 `--write` 仅限本次改动文件）

- [ ] **Step 5: Commit**

```bash
git add app/web/src/stores/socketStore.ts app/web/docs/design/AGENTS-stores.md docs/frontend-coupling-remaining.md
git commit -m "$(cat <<'EOF'
docs(web): document socketStore module boundaries

Record the facade-plus-helpers shape after decomposition so future changes keep transport, room state, and mediasoup signaling separated.
EOF
)"
```

---

### Task 9: 手工回归清单（必须执行，不可跳过）

**Files:** 无代码改动，只验证行为。

- [ ] **Step 1: 启动前端与后端**

```bash
# terminal A
cd /Users/noelorin/GOSpeak/app/server && go run . server

# terminal B
cd /Users/noelorin/GOSpeak/app/web && pnpm dev
```

- [ ] **Step 2: 单标签页主路径**

1. 登录
2. 打开房间列表，确认 `connect` 成功、房间可刷新
3. 创建房间 / 加入房间
4. 观察成员列表、进房离房提示音（若开启）
5. 麦克风静音切换，确认 `member:updated` 反映到 UI
6. 离开房间，列表 count 正确

Expected: 与拆分前一致，无 “已在其他标签页连接” 误报。

- [ ] **Step 3: 多标签页锁**

1. 同一账号再开一个浏览器标签页进入应用
2. 第二个标签页触发连接

Expected: toast “已在其他标签页连接，请关闭其他标签页后重试”，且不会出现双 socket 稳定在线。

- [ ] **Step 4: 被踢 / 禁言（有权限时）**

1. 管理端或另一管理员踢人
2. 被踢端应触发 `onRoomKicked` 并退出语音会话
3. 禁言后 `userBar` / 推流桥接应进入仅收听（`speechRestricted === true`）

- [ ] **Step 5: 若环境有 MediaSoup**

1. 切换到 mediasoup provider（若可用）
2. 进房，确认不会因 `getRouterCapabilities/produce/consume` 转发回归而失败

- [ ] **Step 6: 最终提交前再跑一次自动检查**

```bash
cd app/web && pnpm test && pnpm exec tsc --noEmit --pretty false
```

Expected: PASS。

若手工回归发现行为差异：优先修 pure function / helper，**不要**在 UI 层打补丁掩盖 store 语义变化。

---

## 后续可选（不在本计划提交范围）

1. **去掉 store → speakingStore 直写**  
   改为 `socketStore.onActiveSpeakers(cb)`，由 `useRoomAudioBridge` 或 `voiceChat` 写入。
2. **dashboard 只依赖 activity 订阅模块**  
   不必知道 join/leave/mediasoup。
3. **逐步让 `useVoiceSession` 直接用 `mediasoupSignal` 类型**  
   但保持 runtime 仍走 `socketStore` 直到 MediaSoup 路径稳定。

---

## Self-Review

### Spec coverage

| 目标 | 对应任务 |
|------|----------|
| 拆掉 God Store 且不炸调用方 | Task 1–8 全程 facade 兼容 |
| tab 锁可测 | Task 2–3 |
| 房间成员 reduce 可测 | Task 4–5 |
| provider 热切换副作用可测 | Task 6 |
| MediaSoup 信令可测 | Task 7 |
| 文档同步 | Task 8 |
| 真实语音链路回归 | Task 9 |
| 不扩 scope 到 Context/协议重写 | 非目标已声明 |

### Placeholder scan

- 无 TBD/TODO 步骤
- 每个代码任务都有完整代码块或精确替换说明
- 测试命令与期望结果已写明
- 每次任务都有 commit message

### Type/API consistency

- 公共方法名与现网 `socketStore` return 一致
- 类型名 `MemberInfo`/`RoomInfo`/`ActivityEvent`/`RoomPresenceEvent` 保持不变
- `createTabLock` / `mergeRoomUpdated` / `createMediasoupSignal` / `createProviderReloadHandler` 在后续任务中的命名一致
- `EVENTS` 仍从 `@/socket/events` 来，store 继续 re-export

---

## 执行建议

推荐按 Task 1 → 9 顺序执行，**每个 Task 单独 commit**。  
若中途 `useVoiceSession` 或进房失败，先回看最近一个 helper 的语义差异，不要继续往后拆。
