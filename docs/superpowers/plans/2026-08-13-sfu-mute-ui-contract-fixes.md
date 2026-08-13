# SFU 禁言前端与能力契约修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复提交 74c835a 引入的前端问题：测试基建假绿灯、join 静音状态单向同步、socket 事件契约字段被忽略、SFU 能力表与后端漂移、agora 禁用不对称、音频状态残留与重复逻辑、WHIP 403 判定过宽、preload 行为不一致。

**Architecture:** 改动位于 `app/web` 与 `packages/sfu-client`。核心原则：(1) 先修 vitest 基建让 `providers.test.ts` 真正可跑，再写依赖它的测试；(2) join ack 静音同步改为按 `isMuted` 双向设置并覆盖 SRS/Cloudflare 两个适配器；(3) 前端静态能力表与后端 `CapabilitiesFor` 对齐（cloudflare `serverMute=true`、srs list=hard）；(4) 清理 `handler_audio` 的服务器静音状态残留。

**Tech Stack:** SolidJS、TypeScript、Vitest 4、pnpm workspace。

---

## Findings 覆盖表（Review → Task）

| Review finding | Task |
|---|---|
| 🟡 `providers.test.ts:74` suite import 期崩溃（node 无 localStorage）假绿灯 | T1 |
| 🟡 `providers.ts:44/45/46/47` join 只置位不重置、仅 SRS 适配器 | T2 |
| ❓ `providers.ts:70` cloudflare afterMediaJoin 未同步 isMuted | T2 |
| 🔵 `socketEvents.ts:219/222` 忽略 payload.muted 字段 | T3 |
| 🟡 `socketEvents.ts:220` 静音无兜底（断线错过 member:muted） | T2（join 快照覆盖，附验证） |
| 🟡 `sfuProfiles.ts:310` cloudflare serverMute=false 与后端矛盾 | T4 |
| 🟡 `sfuProfiles.ts:144/164/171/172` srs 文案/level 漂移 | T4 |
| 🟡 `provider.ts:4/5` agora 仅前端禁用（前端侧行为） | T5 |
| 🔵 `provider.ts:7` ENABLED_SFU_PROVIDERS 死导出 + 类型未收窄 | T5 |
| 🔵 `handler_audio/index.ts:135/138` 重复逻辑 + serverMuted 残留 | T6 |
| 🔵 `srs-stream-gate.ts:165/167` /403/ 子串过宽 | T7 |
| 🔵 `factory.ts:18` preloadSFUClient 静默 return 与 create 不一致 | T8 |
| ❓ `srs-stream-gate.ts:171` SRS on_publish 拒绝时 WHIP HTTP 状态 | T9（手工验证） |

---

### Task 1: vitest 全局 localStorage 基建（修复假绿灯）

**Files:**
- Create: `app/web/vitest.setup.ts`
- Create: `app/web/vitest.config.ts`
- Test: `app/web/src/components/room/session/providers.test.ts`（验证可运行）

背景：`app/web/src/stores/userStore.ts:23` 模块顶层访问 `localStorage`，node 环境（vitest 默认 environment）无该全局对象，`providers.test.ts` import 期即崩溃，文件 0 test。实现采用 `vitest.config.ts`（jsdom + setupFiles）接入，而非 package.json script：Vitest 4.1.9 已移除 `--setupFiles` CLI 参数，script 写法会直接失败。

- [ ] **Step 1: 创建 setup 文件**

新建 `app/web/vitest.setup.ts`：

```ts
// vitest.setup.ts — node 环境无 localStorage，userStore 等模块顶层读取会崩溃；
// 在测试加载前注入最小内存实现。
class MemoryStorage implements Storage {
	private data = new Map<string, string>();

	get length(): number {
		return this.data.size;
	}
	clear(): void {
		this.data.clear();
	}
	getItem(key: string): string | null {
		return this.data.get(key) ?? null;
	}
	key(index: number): string | null {
		return Array.from(this.data.keys())[index] ?? null;
	}
	removeItem(key: string): void {
		this.data.delete(key);
	}
	setItem(key: string, value: string): void {
		this.data.set(key, String(value));
	}
}

Object.defineProperty(globalThis, "localStorage", {
	value: new MemoryStorage(),
	writable: true,
});
```

- [ ] **Step 2: 接入 vitest（config 方案，Vitest 4.1.9 不再支持 CLI --setupFiles）**

新建 `app/web/vitest.config.ts`，复用 vite 配置并显式使用 jsdom + setup 文件：

```ts
import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

export default mergeConfig(
	viteConfig,
	defineConfig({
		test: {
			environment: "jsdom",
			setupFiles: ["./vitest.setup.ts"],
		},
	}),
);
```

- [ ] **Step 3: 验证原崩溃套件可运行**

Run: `cd app/web && pnpm vitest run src/components/room/session/providers.test.ts`
Expected: 既有用例全部 PASS（此前 0 test 崩溃）。

- [ ] **Step 4: 回归全量前端测试**

Run: `cd app/web && pnpm test`
Expected: 全部 PASS（含 socketEvents/handler_audio 新增用例）。

- [ ] **Step 5: Commit**

```bash
git add app/web/vitest.setup.ts app/web/vitest.config.ts
git commit -m "test(web): add localStorage setup so suite runs under node"
```

---

### Task 2: join 静音状态双向同步（SRS + Cloudflare）

**Files:**
- Modify: `app/web/src/components/room/session/providers.ts`
- Test: `app/web/src/components/room/session/providers.test.ts`

背景：`srsAdapter.afterMediaJoin` 只对 `isMuted` 为 true 的成员调用 `setServerMutedByIdentity(id, true)`，从不按 ack 清除旧值；断线重连错过 `member:unmuted` 后残留静音无法恢复。`cloudflareAdapter` 完全没做同步（❓:70）。

- [ ] **Step 1: 写失败测试**

更新 `app/web/src/components/room/session/providers.test.ts`：

- 在 `srs afterMediaJoin applies server mute for muted members` 用例中先 `vi.mocked(setServerMutedByIdentity).mockClear()`，并追加断言：

```ts
		expect(setServerMutedByIdentity).toHaveBeenCalledWith("bob", true);
		expect(setServerMutedByIdentity).toHaveBeenCalledWith("carol", false);
		expect(setServerMutedByIdentity).not.toHaveBeenCalledWith("alice", true);
		expect(setServerMutedByIdentity).not.toHaveBeenCalledWith("alice", false);
```

- 新增用例：

```ts
	it("cloudflare afterMediaJoin syncs server mute state both ways", async () => {
		const { setServerMutedByIdentity } = await import("@/handler_audio");
		vi.mocked(setServerMutedByIdentity).mockClear();
		const adapter = getVoiceProviderAdapter("cloudflare");
		await adapter.afterMediaJoin?.(
			{ subscribePeers: vi.fn() } as any,
			{
				token: "t",
				room: "r1",
				identity: "alice",
				stream: "sess-alice",
			} as any,
			{
				members: [
					{ identity: "alice", stream: "sess-alice", isMuted: true } as any,
					{ identity: "bob", stream: "sess-bob", isMuted: true } as any,
					{ identity: "carol", stream: "sess-carol", isMuted: false } as any,
				],
			},
		);
		expect(setServerMutedByIdentity).toHaveBeenCalledWith("bob", true);
		expect(setServerMutedByIdentity).toHaveBeenCalledWith("carol", false);
		expect(setServerMutedByIdentity).not.toHaveBeenCalledWith("alice", true);
	});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/web && pnpm vitest run src/components/room/session/providers.test.ts`
Expected: FAIL——carol 未收到 `(false)` 调用、cloudflare 未同步。

- [ ] **Step 3: 最小实现**

在 `app/web/src/components/room/session/providers.ts` 增加共享 helper，并在两个适配器使用：

```ts
// 服务器禁言状态同步：以 join ack 快照为权威双向设置。
// 只置位不重置会让断线重连错过 member:unmuted 的成员残留陈旧静音。
function syncServerMuteState(
	ack: { members?: Array<{ identity: string; isMuted?: boolean }> },
	selfIdentity: string,
): void {
	for (const m of ack.members ?? []) {
		if (m.identity === selfIdentity) continue;
		setServerMutedByIdentity(m.identity, Boolean(m.isMuted));
	}
}
```

`srsAdapter.afterMediaJoin` 末尾的循环替换为：

```ts
		syncServerMuteState(ack, token.identity);
```

`cloudflareAdapter.afterMediaJoin` 在 `bindSignalActiveSpeakers(client, token);` 前加入：

```ts
		syncServerMuteState(ack, token.identity);
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/web && pnpm vitest run src/components/room/session/providers.test.ts`
Expected: 全部 PASS。

- [ ] **Step 5: 验证断线兜底（对应 🟡:220）**

Run: `cd app/web && pnpm test`
Expected: 全部 PASS。说明：断线重连会重新执行 `afterMediaJoin`，ack 快照即"重拉 mute 状态"，因此 🟡:220 的"错过 member:muted"场景由本任务的双向同步覆盖。

- [ ] **Step 6: Commit**

```bash
git add app/web/src/components/room/session/providers.ts app/web/src/components/room/session/providers.test.ts
git commit -m "fix(web): sync server mute state from join ack for srs and cloudflare"
```

---

### Task 3: socket 事件使用 payload.muted 字段

**Files:**
- Modify: `app/web/src/socket/socketEvents.ts`
- Test: `app/web/src/socket/socketEvents.test.ts`

背景：`MEMBER_MUTED` handler 忽略 `data.muted` 恒置 true，异常时序收到 `{muted:false}` 会错误静音；`MEMBER_UNMUTED` 同理。

- [ ] **Step 1: 写失败测试**

在 `app/web/src/socket/socketEvents.test.ts` 新增：

```ts
	it("member:muted honors payload muted flag", async () => {
		const { setServerMutedByIdentity } = await import("@/handler_audio");
		vi.mocked(setServerMutedByIdentity).mockClear();
		const { bindServerEvents } = await import("@/socket/socketEvents");
		const adapter = createFakeAdapter();
		bindServerEvents(adapter as any, fakeDeps());
		adapter.emit(EVENTS.MEMBER_MUTED, { identity: "bob", muted: false });
		expect(setServerMutedByIdentity).toHaveBeenCalledWith("bob", false);
	});
```

`createFakeAdapter`/`fakeDeps`/`bindServerEvents` 均已在 `app/web/src/socket/socketEvents.test.ts` 定义（既有 `member:muted / member:unmuted` 用例同模式）。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/web && pnpm vitest run src/socket/socketEvents.test.ts`
Expected: FAIL，当前 `toHaveBeenCalledWith("bob", false)` 未发生（被恒置 true）。

- [ ] **Step 3: 最小实现**

修改 `app/web/src/socket/socketEvents.ts` 的两个 handler：

```ts
	adapter.onServerEvent(
		EVENTS.MEMBER_MUTED as string,
		(data: { identity?: string; muted?: boolean }) => {
			if (data.identity) {
				setServerMutedByIdentity(data.identity, data.muted !== false);
			}
		},
	);
	adapter.onServerEvent(
		EVENTS.MEMBER_UNMUTED as string,
		(data: { identity?: string; muted?: boolean }) => {
			if (data.identity) {
				setServerMutedByIdentity(data.identity, Boolean(data.muted));
			}
		},
	);
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/web && pnpm vitest run src/socket/socketEvents.test.ts`
Expected: 全部 PASS（既有 `muted:true`/空 payload 用例不回归）。

- [ ] **Step 5: Commit**

```bash
git add app/web/src/socket/socketEvents.ts app/web/src/socket/socketEvents.test.ts
git commit -m "fix(web): honor muted flag in member mute socket events"
```

---

### Task 4: SFU 能力表与后端对齐

**Files:**
- Modify: `app/web/src/api/sfuProfiles.ts`
- Create: `app/web/src/api/sfuProfiles.test.ts`

背景：静态能力表与后端 `CapabilitiesFor` 漂移：cloudflare `serverMute:false`（后端已 true/degraded）；srs serverMute/listMembers/listRooms 文案与 level 仍旧（后端已改为禁推黑名单 + SRS API 直查 + hard）。

- [ ] **Step 1: 写失败测试**

新建 `app/web/src/api/sfuProfiles.test.ts`：

```ts
import { describe, expect, it } from "vitest";
import {
	getSFUProviderCapabilities,
	SFU_ENFORCEMENT_PROFILES,
} from "./sfuProfiles";

describe("SFU capability tables match backend CapabilitiesFor", () => {
	it("cloudflare serverMute is enabled (backend: ServerMute=true, degraded)", () => {
		expect(getSFUProviderCapabilities("cloudflare").serverMute).toBe(true);
	});

	it("srs list capabilities are hard (backend: SRS API direct, ListLevel=hard)", () => {
		const caps = getSFUProviderCapabilities("srs");
		expect(caps.listRooms).toBe(true);
		expect(caps.listMembers).toBe(true);
		const details = SFU_ENFORCEMENT_PROFILES.srs.details;
		const listRooms = details.find((d) => d.key === "listRooms");
		const listMembers = details.find((d) => d.key === "listMembers");
		expect(listRooms?.level).toBe("hard");
		expect(listMembers?.level).toBe("hard");
	});

	it("srs serverMute impl describes publish-block semantics, not kick", () => {
		const detail = SFU_ENFORCEMENT_PROFILES.srs.details.find(
			(d) => d.key === "serverMute",
		);
		expect(detail?.impl).toContain("禁推");
	});
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/web && pnpm vitest run src/api/sfuProfiles.test.ts`
Expected: FAIL（`serverMute` 为 false、level 为 degraded、impl 文案含"踢"）。

- [ ] **Step 3: 最小实现**

`app/web/src/api/sfuProfiles.ts`：

- `SFU_PROVIDER_CAPABILITIES.cloudflare.serverMute`：`false` → `true`
- `SFU_ENFORCEMENT_PROFILES.srs.details` 中 `serverMute.impl` 改为：

```ts
impl: "写禁推黑名单 + 订阅端静音，断流后禁止重推",
```

- `listMembers`/`listRooms` 的 `level` 改为 `"hard"`，`impl` 分别改为：

```ts
impl: "SRS /api/v1/streams 直查 + stream→room 反查聚合 identity",
```

```ts
impl: "SRS /api/v1/streams 直查 + stream→room 反查聚合 room",
```

（保持与后端 `CapabilitiesFor("srs")` 的 `ListLevel: hard` 及实现一致；`fallback` 字段可保留。）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/web && pnpm vitest run src/api/sfuProfiles.test.ts`
Expected: 全部 PASS。

- [ ] **Step 5: 类型检查**

Run: `cd app/web && pnpm exec tsc --noEmit`
Expected: 无错误。

- [ ] **Step 6: Commit**

```bash
git add app/web/src/api/sfuProfiles.ts app/web/src/api/sfuProfiles.test.ts
git commit -m "fix(web): align SFU capability tables with backend"
```

---

### Task 5: agora 禁用对称与死导出清理

**Files:**
- Modify: `packages/sfu-client/src/provider.ts`
- Modify: `app/web/src/pages/(app)/manage/sfu/index.tsx`
- Test: `packages/sfu-client/src/factory.test.ts`（既有禁用用例保持）

背景：`ENABLED_SFU_PROVIDERS` 全仓无消费方（死导出）且 filter 后类型未收窄；`handleSave` 对当前 provider 为 agora 时拦截保存，提示文案缺少迁移引导。

- [ ] **Step 1: 确认死导出**

Run: `cd /Users/noelorin/GOSpeak && rg -n "ENABLED_SFU_PROVIDERS" packages app/web --glob '!node_modules'`
Expected: 仅 `packages/sfu-client/src/provider.ts` 一处定义，无消费方。

- [ ] **Step 2: 删除死导出并收窄类型**

`packages/sfu-client/src/provider.ts` 删除 `ENABLED_SFU_PROVIDERS` 常量（保留 `DISABLED_SFU_PROVIDERS`/`isSFUProviderEnabled`/`assertSFUProviderEnabled`）。

若后续需要启用列表，用带谓词的收窄写法：

```ts
export const ENABLED_SFU_PROVIDERS = ALL_SFU_PROVIDERS.filter(
	(provider): provider is Exclude<(typeof ALL_SFU_PROVIDERS)[number], "agora"> =>
		!DISABLED_SFU_PROVIDERS.includes(provider),
);
```

（本计划直接删除；恢复时使用上述谓词保证类型收窄。）

- [ ] **Step 3: 更新管理页迁移引导**

`app/web/src/pages/(app)/manage/sfu/index.tsx` 的 `handleSave` 拦截分支文案改为带迁移引导：

```tsx
		if (!isSFUProviderEnabled(current.provider)) {
			showToast(
				"该语音后端已在前端暂时停用，请先切换到已启用的 provider 再保存配置",
				{ type: "error" },
			);
			return;
		}
```

- [ ] **Step 4: 验证**

Run: `cd packages/sfu-client && pnpm exec tsc --noEmit && pnpm exec vitest run src/factory.test.ts`
Expected: 类型检查通过；`factory.test.ts` 禁用用例（agora disabled、preload、create rejects）PASS。

Run: `cd app/web && pnpm exec tsc --noEmit`
Expected: 无错误。

- [ ] **Step 5: Commit**

```bash
git add packages/sfu-client/src/provider.ts "app/web/src/pages/(app)/manage/sfu/index.tsx"
git commit -m "chore(sfu): drop unused enabled-provider export, guide migration on save"
```

---

### Task 6: handler_audio 去重与 serverMuted 状态清理

**Files:**
- Modify: `app/web/src/handler_audio/index.ts`
- Test: `app/web/src/handler_audio/index.test.ts`

背景：`setServerMutedByIdentity` 与 `setMutedByIdentity` 逻辑同构；`serverMutedIdentities` 在 `onTrackUnsubscribed` 时不清除，Set 随历史 identity 无限增长。

- [ ] **Step 1: 写失败测试**

在 `app/web/src/handler_audio/index.test.ts` 新增：

```ts
	it("clears server mute state when remote track unsubscribed", async () => {
		mod.setServerMutedByIdentity("alice", true);
		expect(mod.getServerMutedIdentities().has("alice")).toBe(true);

		const client = makeClient(track);
		setupAudioHandler(client as any);
		const removeCb = vi.mocked(client.onRemoteAudioTrackRemoved).mock
			.calls[0][0] as (identity: string) => void;
		removeCb("alice");

		expect(mod.getServerMutedIdentities().has("alice")).toBe(false);
	});
```

`makeClient` 的 `onRemoteAudioTrackRemoved` 已是 `vi.fn()`，`setupAudioHandler` 注册回调后可触发。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/web && pnpm vitest run src/handler_audio/index.test.ts`
Expected: 编译失败（`getServerMutedIdentities` 未导出）。

- [ ] **Step 3: 最小实现**

`app/web/src/handler_audio/index.ts`：

```ts
function applyMuteState(identity: string, store: Set<string>, muted: boolean): void {
	if (muted) {
		store.add(identity);
	} else {
		store.delete(identity);
	}
	const track = tracks.get(identity);
	if (track) {
		track.setVolume(effectiveVolume(identity));
	}
}

export function setMutedByIdentity(identity: string, muted: boolean) {
	applyMuteState(identity, mutedIdentities, muted);
}

/** 服务器禁言驱动的订阅端静音（member:muted/member:unmuted），独立于本地个人静音。 */
export function setServerMutedByIdentity(identity: string, muted: boolean) {
	applyMuteState(identity, serverMutedIdentities, muted);
}

/** 测试/调试用：当前被服务器静音的 identity 集合。 */
export function getServerMutedIdentities(): ReadonlySet<string> {
	return serverMutedIdentities;
}
```

`onTrackUnsubscribed` 中清理服务器静音状态：

```ts
function onTrackUnsubscribed(identity: string) {
	const track = tracks.get(identity);
	track?.detach();
	tracks.delete(identity);
	// 服务器静音是事件驱动状态：轨道移除后残留会导致 Set 无限增长，
	// 且成员重连后 join ack 会重新同步该状态。
	serverMutedIdentities.delete(identity);
	const el = audioElements.get(identity);
	if (el?.parentNode) {
		el.parentNode.removeChild(el);
	}
	audioElements.delete(identity);
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/web && pnpm vitest run src/handler_audio/index.test.ts`
Expected: 全部 PASS（既有 volume 用例不回归）。

- [ ] **Step 5: Commit**

```bash
git add app/web/src/handler_audio/index.ts app/web/src/handler_audio/index.test.ts
git commit -m "fix(web): dedupe mute state application, clear server mute on unsub"
```

---

### Task 7: WHIP 403 判定加数字边界

**Files:**
- Modify: `packages/sfu-client/src/srs-stream-gate.ts`
- Test: `packages/sfu-client/src/srs-client.test.ts`

背景：`/403|publish denied|forbidden/i` 中 `403` 为无边界子串，误伤 `4032`/`1403` 等 SRS 业务码，把本应 busy 重试的临时错误判为"非 busy"直接失败。

- [ ] **Step 1: 写失败测试**

在 `packages/sfu-client/src/srs-client.test.ts` 的 `isWhipBusyError` describe 中新增：

```ts
	it("treats codes containing 403 (e.g. 4032) as busy, not publish denied", () => {
		expect(
			isWhipBusyError(new Error("SRS WHIP request failed: 4032")),
		).toBe(true);
	});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/sfu-client && pnpm exec vitest run src/srs-client.test.ts`
Expected: FAIL（`4032` 因包含 `403` 被判 false）。

- [ ] **Step 3: 最小实现**

`packages/sfu-client/src/srs-stream-gate.ts`：

```ts
	// 403 = SRS on_publish 回调拒绝（禁推/鉴权失败），不是 busy，绝不能按 busy 无限重试；
	// 加数字边界避免误伤 4032/1403 等业务码（那些应视为 busy 重试）。
	if (/(?:^|\D)403(?:\D|$)|publish denied|forbidden/i.test(msg)) {
		return false;
	}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/sfu-client && pnpm exec vitest run src/srs-client.test.ts`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/sfu-client/src/srs-stream-gate.ts packages/sfu-client/src/srs-client.test.ts
git commit -m "fix(sfu-client): bound 403 match so SRS codes like 4032 stay busy-retryable"
```

---

### Task 8: preloadSFUClient 与 create 行为一致

**Files:**
- Modify: `packages/sfu-client/src/factory.ts`
- Test: `packages/sfu-client/src/factory.test.ts`

背景：`preloadSFUClient` 对禁用 provider 静默 `return`，与 `createSFUClient` 抛错不一致，agora loader/分支成为行为死角。

- [ ] **Step 1: 更新测试预期**

`packages/sfu-client/src/factory.test.ts` 修改既有用例：

```ts
	it("preload rejects disabled provider like create", async () => {
		await expect(preloadSFUClient("agora")).rejects.toThrow(
			/temporarily disabled/,
		);
	});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/sfu-client && pnpm exec vitest run src/factory.test.ts`
Expected: FAIL（当前 resolves）。

- [ ] **Step 3: 最小实现**

`packages/sfu-client/src/factory.ts`：

```ts
export async function preloadSFUClient(provider: SFUProvider): Promise<void> {
	assertSFUProviderEnabled(provider);
	await (providerLoaders[provider] ?? providerLoaders.livekit)();
}
```

`assertSFUProviderEnabled` 已在 import 中（当前只 import 了 `isSFUProviderEnabled`），需把 `isSFUProviderEnabled` 换成 `assertSFUProviderEnabled`：

```ts
import {
	assertSFUProviderEnabled,
	type SFUProvider,
} from "./provider";
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/sfu-client && pnpm exec vitest run src/factory.test.ts`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/sfu-client/src/factory.ts packages/sfu-client/src/factory.test.ts
git commit -m "fix(sfu-client): preload rejects disabled providers consistently"
```

---

### Task 9: 手工验证清单（❓ 收口）

**Files:** 无代码改动；仅验证。

> 执行备注（2026-08-13）：用户指示跳过本任务的手工验证。以下清单保留供部署环境补做；代码侧 `❓:171` 的 WHIP 状态码分支已按 403 语义处理，若真实 SRS 返回其他 4xx/5xx，需按 Step 1 补“非 busy”分支。

- [ ] **Step 1: 验证 SRS on_publish 拒绝的 WHIP HTTP 状态码（❓:171）**

部署 SRS（`docker compose -f deploy/docker-compose.yml up srs` 或现有实例），对被禁言 stream 发起 WHIP：

```bash
curl -i -X POST "http://<srs-host>:<srs-http-port>/rtc/v1/whip/?app=live&stream=gs-blocked&token=<stream-token>" -H "Content-Type: application/sdp" --data-binary @offer.sdp
```

Expected: 记录返回状态码（403 或 4xx/5xx）。若为 403：`isWhipBusyError` 返回 false，客户端不无限重试（正确）；若为其他状态码：`isWhipBusyError` 会判定 busy 并重试——需在 `srs-stream-gate.ts` 补充该状态码到"非 busy"分支。

- [ ] **Step 2: 验证离线 unmute 后重连自愈**

1. 用户 A 在线进房 → 管理员禁言 A（观察 `member:muted` 广播到该房间）→ A 断流/离线。
2. 管理员解禁 A。
3. A 重新 join 房间并推流。
Expected: join 通过；`on_publish` 不再被黑名单拒绝（join 自愈清理生效）。

- [ ] **Step 3: 验证跨实例解禁延迟**

双实例部署，实例 X 执行 mute、实例 Y 执行 unmute，随后在 X 上重推。
Expected: `on_publish` 立即放行（`GetFresh` 跳过 L1，不再有 30s 窗口）。

- [ ] **Step 4: 验证到期禁言广播**

创建 10s 临时禁言，等待到期。
Expected: 到期后收到 `member:unmuted`/`user:unmuted`，订阅端音量恢复（定时扫描任务生效）。

---

## 验收清单

- [ ] `cd app/web && pnpm test` 全部通过（含 providers/socketEvents/handler_audio/sfuProfiles 用例）
- [ ] `cd app/web && pnpm exec tsc --noEmit` 无错误
- [ ] `cd packages/sfu-client && pnpm exec tsc --noEmit && pnpm exec vitest run` 全部通过
- [ ] 手工验证清单（Task 9）4 项完成并记录结果

## 残余风险

- `member:muted` 仍为单发事件：如果订阅端在事件发出后才建立订阅且 join ack 快照缺失，需依赖下一次重连同步；已由 Task 2 的双向同步覆盖在线成员。
- SRS WHIP 拒绝状态码（❓:171）若为 4xx/5xx 而非 403，需按验证结果追加"非 busy"分支（Task 9 Step 1）。
