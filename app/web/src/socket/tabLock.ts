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
	/** 确保监听已建立，能响应他页的 probe（Web Locks 模式下为 no-op） */
	ensureListening: () => void;
	setOnForeignClaim: (cb: (() => void) | null) => void;
};

function hasWebLocks(): boolean {
	return typeof navigator !== "undefined" && !!navigator.locks;
}

let warnedNoLocks = false;

export function createTabLock(options: TabLockOptions): TabLock {
	const probeTimeoutMs = options.probeTimeoutMs ?? 150;
	let isOwner = false;
	let onForeignClaim = options.onForeignClaim ?? null;

	// ── Web Locks 模式：navigator.locks（ifAvailable）是浏览器原生的跨标签页互斥，
	//    没有竞速窗口，也不需要 probe/owner 消息协议。锁随文档销毁自动释放。
	let locksRelease: (() => void) | null = null;

	async function claimWithLocks(): Promise<boolean> {
		if (isOwner) return true;
		return new Promise<boolean>((resolve) => {
			let settled = false;
			let resolveRelease: (() => void) | null = null;
			void navigator.locks
				.request(options.channelName, { ifAvailable: true }, (lock) => {
					if (!lock) {
						settled = true;
						resolve(false);
						return;
					}
					isOwner = true;
					settled = true;
					resolve(true);
					// 挂住回调 promise 直到 release()：持锁期间 request 不会 settle
					return new Promise<void>((r) => {
						resolveRelease = r;
					});
				})
				.catch(() => {
					if (!settled) {
						settled = true;
						resolve(false);
					}
				});
			locksRelease = () => resolveRelease?.();
		});
	}

	function releaseWithLocks(): void {
		if (!isOwner) return;
		isOwner = false;
		locksRelease?.();
		locksRelease = null;
	}

	// ── BroadcastChannel 降级模式：150ms probe 竞速（旧浏览器无 Web Locks 时）──
	let channel: BroadcastChannel | null = null;

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
					// 已有持有者时回应，阻止其他标签页连接
					channel?.postMessage({
						type: "owner",
						from: options.tabId,
					} satisfies TabMessage);
					return;
				}
				if (msg.type === "claimed" && isOwner) {
					// 理论上不应发生；若他页抢占成功，本页让出
					onForeignClaim?.();
				}
			};
		}
		return channel;
	}

	async function claimWithChannel(): Promise<boolean> {
		const ch = getChannel();
		if (!ch) {
			// 不支持 BroadcastChannel 时放行
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
			// timer 先于 probe：owner 可能同步回应，finish 需能 clearTimeout
			const timer = window.setTimeout(() => finish(true), probeTimeoutMs);
			ch.postMessage({
				type: "probe",
				from: options.tabId,
			} satisfies TabMessage);
		});
	}

	function releaseWithChannel(): void {
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

	if (!hasWebLocks() && !warnedNoLocks) {
		warnedNoLocks = true;
		console.warn(
			"[tabLock] navigator.locks unavailable; falling back to BroadcastChannel probe lock",
		);
	}

	const useLocks = hasWebLocks();

	return {
		claim: () => (useLocks ? claimWithLocks() : claimWithChannel()),
		release: () => (useLocks ? releaseWithLocks() : releaseWithChannel()),
		isOwner: () => isOwner,
		ensureListening: () => {
			if (!useLocks) getChannel();
		},
		setOnForeignClaim: (cb) => {
			onForeignClaim = cb;
		},
	};
}
