import type {
	AudioFrameEvent,
	PcmStream,
	PcmStreamFilter,
	PcmStreamReader,
	PcmStreamSink,
} from "./types";

type FrameListener = (frame: AudioFrameEvent) => void;

/**
 * 进程内 PCM 帧总线：旁听 adapter 写入，ASR/插件/外部模块读取。
 * 同时实现 `PcmStream`（读）与 `PcmStreamSink`（写）。
 */
export class PcmStreamHub implements PcmStream, PcmStreamSink {
	private listeners = new Set<FrameListener>();
	private frameCount = 0;
	private lastFrameAt = 0;

	publish(frame: AudioFrameEvent): void {
		this.frameCount += 1;
		this.lastFrameAt = frame.timestamp || Date.now();
		for (const listener of this.listeners) {
			try {
				listener(frame);
			} catch {
				// 单个订阅者异常不影响其他订阅者
			}
		}
	}

	subscribe(listener: FrameListener): () => void {
		this.listeners.add(listener);
		return () => {
			this.listeners.delete(listener);
		};
	}

	subscribeFiltered(
		filter: PcmStreamFilter,
		listener: FrameListener,
	): () => void {
		return this.subscribe((frame) => {
			if (filter.room && frame.room !== filter.room) return;
			if (filter.identity && frame.identity !== filter.identity) return;
			listener(frame);
		});
	}

	open(filter: PcmStreamFilter = {}): PcmStreamReader {
		return new DefaultPcmStreamReader(this, filter);
	}

	get stats(): { listeners: number; frameCount: number; lastFrameAt: number } {
		return {
			listeners: this.listeners.size,
			frameCount: this.frameCount,
			lastFrameAt: this.lastFrameAt,
		};
	}

	clear(): void {
		this.listeners.clear();
		this.frameCount = 0;
		this.lastFrameAt = 0;
	}
}

class DefaultPcmStreamReader implements PcmStreamReader {
	private closedFlag = false;
	private unsub: (() => void) | null = null;
	private waiters: Array<(frame: AudioFrameEvent | null) => void> = [];
	private queue: AudioFrameEvent[] = [];
	private frameListeners = new Set<FrameListener>();

	constructor(hub: PcmStreamHub, filter: PcmStreamFilter) {
		this.unsub = hub.subscribeFiltered(filter, (frame) => {
			if (this.closedFlag) return;
			if (this.waiters.length > 0) {
				const resolve = this.waiters.shift();
				resolve?.(frame);
			} else {
				// 有界缓冲，避免慢消费者占满内存
				if (this.queue.length >= 64) this.queue.shift();
				this.queue.push(frame);
			}
			for (const listener of this.frameListeners) {
				try {
					listener(frame);
				} catch {
					// ignore
				}
			}
		});
	}

	get closed(): boolean {
		return this.closedFlag;
	}

	onFrame(listener: FrameListener): () => void {
		if (this.closedFlag) return () => {};
		this.frameListeners.add(listener);
		return () => {
			this.frameListeners.delete(listener);
		};
	}

	close(): void {
		if (this.closedFlag) return;
		this.closedFlag = true;
		this.unsub?.();
		this.unsub = null;
		this.frameListeners.clear();
		this.queue = [];
		for (const resolve of this.waiters) resolve(null);
		this.waiters = [];
	}

	async *[Symbol.asyncIterator](): AsyncIterator<AudioFrameEvent> {
		try {
			while (!this.closedFlag) {
				if (this.queue.length > 0) {
					const frame = this.queue.shift();
					if (frame) yield frame;
					continue;
				}
				const frame = await new Promise<AudioFrameEvent | null>((resolve) => {
					this.waiters.push(resolve);
				});
				if (!frame) break;
				yield frame;
			}
		} finally {
			this.close();
		}
	}
}

/** 将 Int16Array PCM 转为 Buffer（s16le） */
export function pcm16ToBuffer(pcm16: Int16Array): Buffer {
	return Buffer.from(pcm16.buffer, pcm16.byteOffset, pcm16.byteLength);
}
