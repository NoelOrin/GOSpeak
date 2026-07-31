import { describe, expect, it, vi } from "vitest";
import { resolveJoinSession } from "./sfuSession";

vi.mock("idb-keyval", () => ({
	get: vi.fn(async () => undefined),
	set: vi.fn(async () => {}),
	del: vi.fn(async () => {}),
}));

describe("resolveJoinSession", () => {
	it("uses whipUrl for srs and ignores serverUrl host:port", () => {
		const session = resolveJoinSession({
			token: "t",
			serverUrl: "http://srs.example.com:1985",
			room: "room-a",
			identity: "alice",
			provider: "srs",
			whipUrl: "/rtc/v1/whip/",
		});
		expect(session.provider).toBe("srs");
		expect(session.connectTarget).toBe("/rtc/v1/whip/");
	});

	it("does not fall back to serverUrl for srs", () => {
		const session = resolveJoinSession({
			token: "t",
			serverUrl: "http://srs.example.com:1985",
			room: "room-a",
			identity: "alice",
			provider: "srs",
		});
		expect(session.connectTarget).toBe("");
	});
});
