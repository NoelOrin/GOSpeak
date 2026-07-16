import { afterEach, describe, expect, it, vi } from "vitest";
import { Scheduler } from "./scheduler";

describe("Scheduler", () => {
	let scheduler: Scheduler;

	afterEach(() => {
		scheduler?.clearAll();
		vi.useRealTimers();
	});

	it("runs every() tasks on interval", () => {
		vi.useFakeTimers();
		scheduler = new Scheduler();
		const fn = vi.fn();
		scheduler.every("tick", 1000, fn);

		vi.advanceTimersByTime(1000);
		expect(fn).toHaveBeenCalledTimes(1);
		vi.advanceTimersByTime(2000);
		expect(fn).toHaveBeenCalledTimes(3);
	});

	it("runs once() only once", () => {
		vi.useFakeTimers();
		scheduler = new Scheduler();
		const fn = vi.fn();
		scheduler.once("later", 500, fn);

		vi.advanceTimersByTime(500);
		expect(fn).toHaveBeenCalledTimes(1);
		vi.advanceTimersByTime(500);
		expect(fn).toHaveBeenCalledTimes(1);
		expect(scheduler.has("later")).toBe(false);
	});

	it("clear() stops a task", () => {
		vi.useFakeTimers();
		scheduler = new Scheduler();
		const fn = vi.fn();
		scheduler.every("tick", 1000, fn);
		scheduler.clear("tick");
		vi.advanceTimersByTime(3000);
		expect(fn).not.toHaveBeenCalled();
	});

	it("clearAll() stops all tasks", () => {
		vi.useFakeTimers();
		scheduler = new Scheduler();
		const a = vi.fn();
		const b = vi.fn();
		scheduler.every("a", 100, a);
		scheduler.once("b", 100, b);
		scheduler.clearAll();
		vi.advanceTimersByTime(500);
		expect(a).not.toHaveBeenCalled();
		expect(b).not.toHaveBeenCalled();
		expect(scheduler.size).toBe(0);
	});
});
