import { describe, expect, it, vi } from "vitest";
import { ListenRoomRegistry, parseRoomList } from "./listenRegistry";

describe("ListenRoomRegistry", () => {
	it("starts with initial rooms", () => {
		const reg = new ListenRoomRegistry(["b", "a", "a"]);
		expect(reg.list()).toEqual(["a", "b"]);
	});

	it("add/remove/clear notify listeners", () => {
		const reg = new ListenRoomRegistry();
		const changes: unknown[] = [];
		reg.onChange((c) => changes.push(c));

		expect(reg.add("lobby")).toBe(true);
		expect(reg.add("lobby")).toBe(false);
		expect(reg.remove("lobby")).toBe(true);
		expect(reg.clear()).toEqual([]);
		reg.add("x");
		expect(reg.clear()).toEqual(["x"]);
		expect(changes.length).toBeGreaterThan(0);
	});

	it("setAll computes added/removed", () => {
		const reg = new ListenRoomRegistry(["a", "b"]);
		const cb = vi.fn();
		reg.onChange(cb);
		const change = reg.setAll(["b", "c"]);
		expect(change.added).toEqual(["c"]);
		expect(change.removed).toEqual(["a"]);
		expect(reg.list()).toEqual(["b", "c"]);
		expect(cb).toHaveBeenCalled();
	});
});

describe("parseRoomList", () => {
	it("parses csv and arrays", () => {
		expect(parseRoomList("a, b, a")).toEqual(["a", "b"]);
		expect(parseRoomList(["x", " x ", ""])).toEqual(["x"]);
		expect(parseRoomList(null)).toEqual([]);
	});
});
