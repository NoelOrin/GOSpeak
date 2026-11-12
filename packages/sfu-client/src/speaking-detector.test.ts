import { describe, it, expect } from "vitest";
import { SpeakingDetector } from "./speaking-detector";

describe("SpeakingDetector", () => {
	it("requires holdOnMs of sustained speech before flipping to speaking", () => {
		const d = new SpeakingDetector(120, 300);
		// 首个异于当前态的样本只启动候选计时
		expect(d.update(0, true)).toBeNull();
		expect(d.update(50, true)).toBeNull();
		// 50ms 后回落：候选作废
		expect(d.update(60, false)).toBeNull();
		// 重新起麦，保持 120ms 后翻转
		expect(d.update(100, true)).toBeNull();
		expect(d.update(160, true)).toBeNull();
		expect(d.update(220, true)).toBe(true);
		// 已确认状态下重复 raw 不再上报
		expect(d.update(280, true)).toBeNull();
	});

	it("requires holdOffMs of sustained silence before flipping off", () => {
		const d = new SpeakingDetector(120, 300);
		expect(d.update(0, true)).toBeNull();
		expect(d.update(200, true)).toBe(true);
		// 短暂静音不翻转
		expect(d.update(300, false)).toBeNull();
		expect(d.update(400, false)).toBeNull();
		// 静音持续满 300ms（从 300 起算）后翻转为 false
		expect(d.update(600, false)).toBe(false);
		expect(d.update(700, false)).toBeNull();
	});

	it("forceFalse emits once and then no-ops", () => {
		const d = new SpeakingDetector(120, 300);
		expect(d.update(0, true)).toBeNull();
		expect(d.update(200, true)).toBe(true);
		expect(d.forceFalse()).toBe(false);
		expect(d.forceFalse()).toBeNull();
		expect(d.update(300, false)).toBeNull();
	});

	it("reset clears state so next speech needs fresh holdOn", () => {
		const d = new SpeakingDetector(120, 300);
		expect(d.update(0, true)).toBeNull();
		expect(d.update(200, true)).toBe(true);
		d.reset();
		expect(d.update(300, true)).toBeNull();
		expect(d.update(400, false)).toBeNull();
	});

	it("treats zero hold times as immediate flip on the second sample", () => {
		const d = new SpeakingDetector(0, 0);
		expect(d.update(100, true)).toBeNull(); // 首样本启动候选
		expect(d.update(101, true)).toBe(true); // elapsed>=0 立即翻转
		expect(d.update(200, false)).toBeNull();
		expect(d.update(201, false)).toBe(false);
	});
});
