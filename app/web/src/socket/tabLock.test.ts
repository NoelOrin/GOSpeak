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

class MockLockManager {
	private held = false;
	private resolveRelease: (() => void) | null = null;

	request(
		_name: string,
		_options: { ifAvailable?: boolean },
		callback: (lock: { name: string; mode: string } | null) => unknown,
	): Promise<unknown> {
		if (this.held) {
			return Promise.resolve(callback(null));
		}
		this.held = true;
		const released = new Promise<void>((r) => {
			this.resolveRelease = r;
		});
		const granted = Promise.resolve(
			callback({ name: _name, mode: "exclusive" }),
		);
		// request 在回调返回的 promise settle（即锁释放）后才 settle
		return Promise.all([granted, released]).then(([v]) => v);
	}

	releaseHeld() {
		this.held = false;
		this.resolveRelease?.();
		this.resolveRelease = null;
	}
}

describe("socket tabLock (navigator.locks)", () => {
	let lockManager: MockLockManager;

	beforeEach(() => {
		lockManager = new MockLockManager();
		vi.stubGlobal("navigator", { ...navigator, locks: lockManager });
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it("claims immediately when the lock is free (no probe timeout)", async () => {
		const { createTabLock } = await import("./tabLock");
		const lock = createTabLock({
			channelName: "locks_socket_tab",
			tabId: "tab-a",
			probeTimeoutMs: 150,
		});

		await expect(lock.claim()).resolves.toBe(true);
		expect(lock.isOwner()).toBe(true);
	});

	it("fails claim when another tab holds the lock", async () => {
		const { createTabLock } = await import("./tabLock");
		const owner = createTabLock({
			channelName: "locks_socket_tab",
			tabId: "tab-owner",
		});
		await owner.claim();

		const challenger = createTabLock({
			channelName: "locks_socket_tab",
			tabId: "tab-b",
		});
		await expect(challenger.claim()).resolves.toBe(false);
		expect(challenger.isOwner()).toBe(false);
	});

	it("release frees the lock so another tab can claim", async () => {
		const { createTabLock } = await import("./tabLock");
		const owner = createTabLock({
			channelName: "locks_socket_tab",
			tabId: "tab-owner",
		});
		await owner.claim();

		const challenger = createTabLock({
			channelName: "locks_socket_tab",
			tabId: "tab-b",
		});
		await expect(challenger.claim()).resolves.toBe(false);

		owner.release();
		// 模拟浏览器侧锁真正释放后回调 promise 才 settle
		lockManager.releaseHeld();

		await expect(challenger.claim()).resolves.toBe(true);
		expect(owner.isOwner()).toBe(false);
	});
});
