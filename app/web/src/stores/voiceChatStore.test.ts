import { describe, expect, it, vi } from "vitest";

const { resolveLoad, getMock } = vi.hoisted(() => {
	let resolveLoad: (value: unknown) => void = () => {};
	return {
		resolveLoad: (value: unknown) => resolveLoad(value),
		getMock: vi.fn(
			() =>
				new Promise((resolve) => {
					resolveLoad = resolve;
				}),
		),
	};
});

vi.mock("idb-keyval", () => ({
	get: getMock,
	set: vi.fn(async () => {}),
	del: vi.fn(async () => {}),
}));

import VoiceChatStore from "./voiceChatStore";

describe("voiceChatStore persisted load", () => {
	it("does not overwrite user changes made before persisted state loads", async () => {
		VoiceChatStore.setIsInputMute(true);

		resolveLoad({ isInputMute: false, isOutMute: false, isVideoMute: false });
		await new Promise((resolve) => setTimeout(resolve, 0));

		expect(VoiceChatStore.data.isInputMute).toBe(true);
	});
});
