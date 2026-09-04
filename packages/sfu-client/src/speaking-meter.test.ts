import { beforeEach, describe, expect, it, vi } from "vitest";
import { LocalSpeakingMeter } from "./speaking-meter";

type FakeAnalyser = {
	frequencyBinCount: number;
	getByteFrequencyData: ReturnType<typeof vi.fn>;
	connect: ReturnType<typeof vi.fn>;
	disconnect: ReturnType<typeof vi.fn>;
};

type FakeNode = {
	port: { onmessage: (() => void) | null; postMessage: ReturnType<typeof vi.fn>; close: ReturnType<typeof vi.fn> };
	connect: ReturnType<typeof vi.fn>;
	disconnect: ReturnType<typeof vi.fn>;
};

let level = 0;

class FakeAudioContext {
	destination = {} as AudioDestinationNode;
	closed = false;
	analyser: FakeAnalyser | null = null;
	audioWorklet: { addModule: ReturnType<typeof vi.fn> } = {
		addModule: vi.fn(async () => {}),
	};
	createMediaStreamSource = vi.fn(() => ({ connect: vi.fn() }));
	createAnalyser = vi.fn(() => {
		this.analyser = {
			frequencyBinCount: 128,
			getByteFrequencyData: vi.fn((arr: Uint8Array) => {
				arr.fill(level);
			}),
			connect: vi.fn(),
			disconnect: vi.fn(),
		};
		return this.analyser;
	});
	createGain = vi.fn(() => ({ gain: { value: 1 }, connect: vi.fn() }));
	close = vi.fn(() => {
		this.closed = true;
		return Promise.resolve();
	});
}

let fakeCtx: FakeAudioContext | null = null;
let fakeNode: FakeNode | null = null;
let now = 0;

beforeEach(() => {
	level = 0;
	fakeCtx = null;
	fakeNode = null;
	now = 0;
	vi.restoreAllMocks();
	vi.spyOn(performance, "now").mockImplementation(() => now);
	(globalThis as any).AudioContext = class {
		constructor() {
			fakeCtx = new FakeAudioContext();
			return fakeCtx;
		}
	};
	(globalThis as any).AudioWorkletNode = class {
		port = { onmessage: null, postMessage: vi.fn(), close: vi.fn() };
		connect = vi.fn();
		disconnect = vi.fn();
		constructor() {
			fakeNode = this;
		}
	};
});

function stream(): MediaStream {
	return { id: "s" } as unknown as MediaStream;
}

/** 等 meter 异步 init（addModule 后建图）完成。 */
async function waitReady(): Promise<void> {
	await vi.waitFor(() => {
		expect(fakeCtx).not.toBeNull();
		expect(fakeNode).not.toBeNull();
	});
}

/** 模拟工作台一次采样事件（reportBlocks 音频块 ≈ 43ms），并推进时钟。 */
async function tick(): Promise<void> {
	now += 43;
	// init 是异步的；若尚未就绪则先等一回合微任务
	await Promise.resolve();
	fakeNode?.port.onmessage?.();
}

/** 连续大声直至滞回翻转 true。 */
async function speakUp(onChange: ReturnType<typeof vi.fn>): Promise<void> {
	level = 200;
	for (let i = 0; i < 3; i++) await tick(); // ~129ms < 120ms+首样本，未翻转
	expect(onChange).not.toHaveBeenCalled();
	await tick(); // ~172ms >= 120ms，翻转
	expect(onChange).toHaveBeenLastCalledWith(true);
}

describe("LocalSpeakingMeter", () => {
	it("is event-driven via AudioWorklet ticks: fires only on state flip with hysteresis", async () => {
		const onChange = vi.fn();
		const meter = new LocalSpeakingMeter({
			threshold: 10,
			holdOnMs: 120,
			holdOffMs: 300,
			onSpeakingChange: onChange,
		});
		meter.start(stream());
		await waitReady();

		// addModule 只加载一次工作台模块
		expect(fakeCtx?.audioWorklet.addModule).toHaveBeenCalledTimes(1);

		await speakUp(onChange);
		expect(onChange).toHaveBeenCalledTimes(1);

		// 持续大声不回退
		for (let i = 0; i < 3; i++) await tick();
		expect(onChange).toHaveBeenCalledTimes(1);

		// 转静音：holdOff 300ms 才翻转（静音候选自首个静音样本起算）
		level = 0;
		for (let i = 0; i < 7; i++) await tick(); // 候选持续 258ms，未翻转
		expect(onChange).toHaveBeenCalledTimes(1);
		await tick(); // 301ms >= 300ms，翻转为 false
		expect(onChange).toHaveBeenLastCalledWith(false);
		expect(onChange).toHaveBeenCalledTimes(2);
	});

	it("start with the same stream is a no-op (single AudioContext)", async () => {
		const meter = new LocalSpeakingMeter({});
		const s = stream();
		meter.start(s);
		await waitReady();
		const first = fakeCtx;
		meter.start(s);
		await Promise.resolve();
		expect(fakeCtx).toBe(first);
		expect(first?.audioWorklet.addModule).toHaveBeenCalledTimes(1);
	});

	it("start with a different stream rebinds (old context closed)", async () => {
		const meter = new LocalSpeakingMeter({});
		meter.start(stream());
		await waitReady();
		const oldCtx = fakeCtx;
		expect(oldCtx?.closed).toBe(false);
		meter.start(stream());
		await vi.waitFor(() => {
			expect(oldCtx?.closed).toBe(true);
		});
		expect(fakeCtx?.audioWorklet.addModule).toHaveBeenCalledTimes(1);
	});

	it("stop emits false once, closes context, and later forceFalse no-ops", async () => {
		const onChange = vi.fn();
		const meter = new LocalSpeakingMeter({ onSpeakingChange: onChange });
		meter.start(stream());
		await waitReady();
		await speakUp(onChange);

		meter.stop();
		expect(onChange).toHaveBeenLastCalledWith(false);
		expect(fakeCtx?.closed).toBe(true);
		expect(meter.isActive).toBe(false);

		meter.forceFalse();
		expect(onChange).toHaveBeenCalledTimes(2);
	});

	it("forceFalse emits false once without tearing down", async () => {
		const onChange = vi.fn();
		const meter = new LocalSpeakingMeter({ onSpeakingChange: onChange });
		meter.start(stream());
		await waitReady();
		await speakUp(onChange);

		meter.forceFalse();
		expect(onChange).toHaveBeenLastCalledWith(false);
		expect(fakeCtx?.closed).toBe(false);
		expect(meter.isActive).toBe(true);

		meter.forceFalse();
		expect(onChange).toHaveBeenCalledTimes(2);
	});

	it("disables detection and closes context when AudioWorklet is unavailable", async () => {
		const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
		const onChange = vi.fn();
		const missingCtx = new FakeAudioContext();
		(missingCtx as unknown as { audioWorklet?: unknown }).audioWorklet = undefined;
		(globalThis as any).AudioContext = class {
			constructor() {
				return missingCtx;
			}
		};
		fakeCtx = missingCtx;

		const meter = new LocalSpeakingMeter({ onSpeakingChange: onChange });
		meter.start(stream());
		await Promise.resolve();
		expect(meter.isActive).toBe(false);
		expect(missingCtx.closed).toBe(true);
		expect(onChange).not.toHaveBeenCalled();
		expect(warn).toHaveBeenCalled();
		warn.mockRestore();
	});

	it("disables detection when addModule rejects", async () => {
		const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
		const onChange = vi.fn();
		const failingCtx = new FakeAudioContext();
		failingCtx.audioWorklet.addModule.mockRejectedValueOnce(
			new Error("module failed"),
		);
		(globalThis as any).AudioContext = class {
			constructor() {
				return failingCtx;
			}
		};
		fakeCtx = failingCtx;

		const meter = new LocalSpeakingMeter({ onSpeakingChange: onChange });
		meter.start(stream());
		await Promise.resolve();
		expect(meter.isActive).toBe(false);
		expect(failingCtx.closed).toBe(true);
		expect(onChange).not.toHaveBeenCalled();
		expect(warn).toHaveBeenCalled();
		warn.mockRestore();
	});
});
