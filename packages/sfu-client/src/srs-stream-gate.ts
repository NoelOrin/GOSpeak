import type { SRSSFUClient } from "./srs-client";

// 同 stream 全局闸门（不改 useVoiceSession）：
// orchestrator abort 会并发“旧 leave + 新 join”，若双 WHIP 同时 POST 同 stream → 5020/500。
// 规则：
// 1) 同一 stream 同一时刻最多一个 holder
// 2) leave 立即 abort in-flight WHIP
// 3) 新 join 必须等旧 holder 释放后再 POST
type StreamGate = {
	holder: SRSSFUClient | null;
	free: Promise<void>;
	resolveFree: (() => void) | null;
	tail: Promise<unknown>;
	heldAt: number;
};

const streamGates = new Map<string, StreamGate>();
// 防止 holder 异常未释放时永久卡在 joining_media。
// 僵死 holder 阈值：仅用于清理泄漏，不能用于抢占仍在 join 的会话。
const STREAM_HOLD_STALE_MS = 30_000;
// 仅配合 abort 检查，不再超时抢 holder。
const ACQUIRE_TIMEOUT_MS = 30_000;
export const WHIP_BUSY_RETRY = 4;
export const WHIP_BUSY_RETRY_MS = 250;

function getStreamGate(streamKey: string): StreamGate {
	const key = streamKey || "__empty__";
	let gate = streamGates.get(key);
	if (!gate) {
		gate = {
			holder: null,
			free: Promise.resolve(),
			resolveFree: null,
			tail: Promise.resolve(),
			heldAt: 0,
		};
		streamGates.set(key, gate);
	}
	return gate;
}

export function enqueueStreamOp<T>(streamKey: string, op: () => Promise<T>): Promise<T> {
	const gate = getStreamGate(streamKey);
	const run = gate.tail.catch(() => undefined).then(op);
	gate.tail = run.then(
		() => undefined,
		() => undefined,
	);
	return run;
}

function forceReleaseStream(streamKey: string, reason: string): void {
	const gate = getStreamGate(streamKey);
	if (!gate.holder && !gate.resolveFree) return;
	console.warn("[SRS] force release stream gate", streamKey, reason);
	gate.holder = null;
	gate.heldAt = 0;
	const resolve = gate.resolveFree;
	gate.resolveFree = null;
	resolve?.();
	gate.free = Promise.resolve();
}

function isDeadHolder(holder: SRSSFUClient): boolean {
	// 仅判定“明确已死但仍占闸门”的 holder。
	// leaving 过程中若仍有 publish 资源，必须等 DELETE 完成，不能抢闸门。
	const anyHolder = holder as unknown as {
		leaving?: boolean;
		hasJoined?: boolean;
		activeStreamKey?: string;
		pendingStreamKey?: string;
		publishPc?: unknown;
		publishResourceUrl?: string;
	};
	const hasMedia = !!(anyHolder.publishPc || anyHolder.publishResourceUrl);
	if (hasMedia) return false;
	if (anyHolder.hasJoined && !anyHolder.leaving) return false;
	// 无媒体且无 stream key：泄漏的空 holder
	if (!anyHolder.activeStreamKey && !anyHolder.pendingStreamKey) {
		return true;
	}
	// leave 已拆媒体但仍短暂占 key：可视为可回收
	if (anyHolder.leaving && !hasMedia) {
		return true;
	}
	return false;
}

function requestHolderLeave(holder: SRSSFUClient): void {
	// 新 join 到来时，尽量让旧 holder 主动 leave，而不是抢闸门双 WHIP。
	void holder.leaveRoom().catch(() => {});
}

export async function acquireStream(
	streamKey: string,
	self: SRSSFUClient,
	isAborted: () => boolean,
): Promise<void> {
	const gate = getStreamGate(streamKey);
	const started = Date.now();
	let askedLeave = false;
	for (;;) {
		if (isAborted()) throw new Error("SRS join aborted");
		if (gate.holder && gate.holder !== self) {
			if (isDeadHolder(gate.holder)) {
				forceReleaseStream(streamKey, "dead-holder");
			} else if (
				gate.heldAt > 0 &&
				Date.now() - gate.heldAt > STREAM_HOLD_STALE_MS
			) {
				// 只对真正僵死的 holder 强制释放；仍先请求 leave。
				if (!askedLeave) {
					requestHolderLeave(gate.holder);
					askedLeave = true;
				}
				forceReleaseStream(streamKey, "stale-holder");
			} else if (!askedLeave) {
				// 同 stream 二次 join：先让旧会话 leave，再等闸门释放。
				requestHolderLeave(gate.holder);
				askedLeave = true;
			}
		}
		if (!gate.holder || gate.holder === self) {
			gate.holder = self;
			gate.heldAt = Date.now();
			if (!gate.resolveFree) {
				gate.free = new Promise<void>((resolve) => {
					gate.resolveFree = resolve;
				});
			}
			return;
		}
		// 禁止超时抢活 holder：那是双 WHIP 根因（第一次成功、第二次 busy）。
		// 只在 abort 或旧 holder leave 后继续。
		if (Date.now() - started > ACQUIRE_TIMEOUT_MS && isAborted()) {
			throw new Error("SRS join aborted");
		}
		await Promise.race([
			gate.free,
			new Promise<void>((resolve) => setTimeout(resolve, 50)),
		]);
	}
}

export function releaseStream(streamKey: string, self: SRSSFUClient): void {
	const gate = getStreamGate(streamKey);
	if (gate.holder !== self) return;
	gate.holder = null;
	gate.heldAt = 0;
	const resolve = gate.resolveFree;
	gate.resolveFree = null;
	resolve?.();
	// free promise 重置为已完成，后续 wait 不挂
	if (!gate.resolveFree) {
		gate.free = Promise.resolve();
	}
}


export function sleep(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

export function isWhipBusyError(err: unknown): boolean {
	const msg = err instanceof Error ? err.message : String(err ?? "");
	// 403 = SRS on_publish 回调拒绝（禁推/鉴权失败），不是 busy，绝不能按 busy 无限重试。
	if (/403|publish denied|forbidden/i.test(msg)) {
		return false;
	}
	// SRS stream busy / duplicate publish 常见：5020、500、400、Failed to fetch 竞态
	return /5020|stream busy|busy|already|409|500|400|Failed to fetch|WHIP request failed/i.test(
		msg,
	);
}

export function appendStream(
	url: string,
	stream: string | undefined,
	token: string | undefined,
	withToken: boolean,
): string {
	if (!stream) return url;
	const sep = url.includes("?") ? "&" : "?";
	let q = `app=live&stream=${encodeURIComponent(stream)}`;
	if (withToken && token) q += `&token=${encodeURIComponent(token)}`;
	return url + sep + q;
}
