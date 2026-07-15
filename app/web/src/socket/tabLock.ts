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

	async function claim(): Promise<boolean> {
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
