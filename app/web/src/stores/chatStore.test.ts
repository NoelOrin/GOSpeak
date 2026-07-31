import { describe, expect, it, vi } from "vitest";

// Mock modules with side effects to avoid indexedDB / socket issues in test
vi.mock("@/stores/socketStore", () => ({
	socketStore: { getSocket: () => null },
}));
vi.mock("@/api/message", () => ({
	listMessages: vi.fn(),
}));

import { mergeMessages } from "./chatStore";

vi.mock("idb-keyval", () => ({
	get: vi.fn(async () => undefined),
	set: vi.fn(async () => {}),
	del: vi.fn(async () => {}),
}));

describe("mergeMessages", () => {
	it("dedupes by uuid and sorts by created_at", () => {
		const a = {
			uuid: "1",
			created_at: "2026-01-01T00:00:01Z",
			content: "a",
		} as any;
		const b = {
			uuid: "2",
			created_at: "2026-01-01T00:00:02Z",
			content: "b",
		} as any;
		const b2 = {
			uuid: "2",
			created_at: "2026-01-01T00:00:02Z",
			content: "b-edit",
		} as any;
		const out = mergeMessages([a, b], [b2]);
		expect(out.map((m) => m.uuid)).toEqual(["1", "2"]);
		expect(out[1].content).toBe("b-edit");
	});
});
