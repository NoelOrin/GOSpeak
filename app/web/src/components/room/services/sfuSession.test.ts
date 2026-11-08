import { describe, expect, it, vi } from "vitest";
import { resolveJoinSession } from "./sfuSession";

vi.mock("idb-keyval", () => ({
	get: vi.fn(async () => undefined),
	set: vi.fn(async () => {}),
	del: vi.fn(async () => {}),
}));

describe("resolveJoinSession", () => {
	it("resolves livekit provider and serverUrl as connect target", () => {
		const session = resolveJoinSession({
			token: "t",
			serverUrl: "wss://lk.example.com",
			room: "room-a",
			identity: "alice",
			provider: "livekit",
		});
		expect(session.provider).toBe("livekit");
		expect(session.connectTarget).toBe("wss://lk.example.com");
	});

	it("always resolves livekit regardless of the response provider", () => {
		const session = resolveJoinSession({
			token: "t",
			serverUrl: "wss://lk.example.com",
			room: "room-a",
			identity: "alice",
		});
		expect(session.provider).toBe("livekit");
	});
});
