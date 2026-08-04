import { describe, expect, it, vi } from "vitest";

// Mock modules with side effects to avoid indexedDB / socket issues in test
vi.mock("@/stores/socketStore", () => ({
	socketStore: {
		getSocket: () => null,
		currentDomainUUID: () => "fallback-domain",
	},
}));
vi.mock("@/api/message", () => ({
	listMessages: vi.fn(),
}));

import {
	mergeMessages,
	mergePrivateMessages,
	remapConversationList,
	remapPrivateMessages,
	remapRecordKey,
} from "./chatStore";

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

describe("mergePrivateMessages", () => {
	it("replaces pending message by client_nonce", () => {
		const pending = {
			uuid: "nonce-1",
			client_nonce: "nonce-1",
			conversation_id: "pm_alice",
			content: "hello",
			created_at: "2026-01-01T00:00:01Z",
		} as any;
		const server = {
			uuid: "msg-1",
			client_nonce: "nonce-1",
			conversation_id: "conv-1",
			content: "hello",
			created_at: "2026-01-01T00:00:01Z",
		} as any;
		const out = mergePrivateMessages([pending], [server]);
		expect(out).toHaveLength(1);
		expect(out[0].uuid).toBe("msg-1");
		expect(out[0].conversation_id).toBe("conv-1");
	});
});

describe("remapPrivateConversationKeys", () => {
	it("moves simple records to the real conversation id", () => {
		expect(remapRecordKey({ pm_alice: true }, "pm_alice", "conv-1")).toEqual({
			"conv-1": true,
		});
	});

	it("moves private messages and rewrites conversation_id", () => {
		const messages = {
			pm_alice: [
				{
					uuid: "msg-1",
					conversation_id: "pm_alice",
					created_at: "2026-01-01T00:00:01Z",
				} as any,
			],
		};
		const out = remapPrivateMessages(messages, "pm_alice", "conv-1");
		expect(out.pm_alice).toBeUndefined();
		expect(out["conv-1"][0].conversation_id).toBe("conv-1");
	});

	it("replaces a temporary conversation in the list", () => {
		const temp = {
			conversation_id: "pm_alice",
			other_identity: "alice",
			last_content: "",
			last_message_at: 0,
		} as any;
		const real = {
			conversation_id: "conv-1",
			other_identity: "alice",
			last_content: "server message",
			last_message_at: 123,
		} as any;
		const out = remapConversationList([temp, real], "pm_alice", "conv-1");
		expect(out).toHaveLength(1);
		expect(out[0].conversation_id).toBe("conv-1");
		expect(out[0].last_content).toBe("server message");
	});
});
