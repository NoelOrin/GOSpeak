import { beforeEach, describe, expect, it } from "vitest";
import { speakingStore, useSpeakingStore } from "./speakingStore";

describe("speakingStore", () => {
	beforeEach(() => {
		speakingStore.set([]);
	});

	it("set 覆盖发言 identity 列表", () => {
		speakingStore.set(["u1", "u2"]);
		expect(useSpeakingStore.getState().speakingIdentities).toEqual([
			"u1",
			"u2",
		]);
	});

	it("再次 set 整体替换（active speakers 快照语义）", () => {
		speakingStore.set(["u1"]);
		speakingStore.set(["u2"]);
		expect(useSpeakingStore.getState().speakingIdentities).toEqual(["u2"]);
	});

	it("get 读取非组件侧状态", () => {
		speakingStore.set(["alice"]);
		expect(speakingStore.get()).toEqual(["alice"]);
	});
});
