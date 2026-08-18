import { describe, expect, it } from "vitest";
import { EventType } from "../core/types";
import { GOSpeakSocketClient } from "./socketClient";

describe("GOSpeakSocketClient speaking contract", () => {
	describe("parseActiveSpeakers", () => {
		it("maps room:active-speakers payload to OnActiveSpeakers", () => {
			const ev = GOSpeakSocketClient.parseActiveSpeakers({
				room: "alpha",
				domain_uuid: "dom-1",
				identities: ["u1", "u2"],
			});
			expect(ev).not.toBeNull();
			if (!ev) return;
			expect(ev.eventType).toBe(EventType.OnActiveSpeakers);
			expect(ev.room.name).toBe("alpha");
			expect(ev.identities).toEqual(["u1", "u2"]);
		});

		it("tolerates missing identities and falls back to empty list", () => {
			const ev = GOSpeakSocketClient.parseActiveSpeakers({ room: "r" });
			expect(ev?.identities).toEqual([]);
			expect(ev?.room.name).toBe("r");
		});

		it("returns null when payload is missing", () => {
			expect(GOSpeakSocketClient.parseActiveSpeakers(null)).toBeNull();
			expect(GOSpeakSocketClient.parseActiveSpeakers("")).toBeNull();
		});
	});

	describe("buildSpeakingMessage", () => {
		it("builds a member:speaking frame with identity and state", () => {
			const msg = GOSpeakSocketClient.buildSpeakingMessage("r1", "bot", true);
			expect(msg.event).toBe("member:speaking");
			expect(msg.data).toEqual({ room: "r1", identity: "bot", speaking: true });
		});

		it("reflects the speaking flag", () => {
			const msg = GOSpeakSocketClient.buildSpeakingMessage("r1", "bot", false);
			expect(msg.data).toEqual({
				room: "r1",
				identity: "bot",
				speaking: false,
			});
		});
	});

	describe("getSpeakers", () => {
		it("returns an empty list before any active-speakers broadcast", () => {
			const client = new GOSpeakSocketClient({
				url: "ws://localhost:8998/ws",
				logger: console,
			});
			expect(client.getSpeakers("any-room")).toEqual([]);
		});
	});
});
