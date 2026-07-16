import { describe, expect, it } from "vitest";
import { EventType } from "../core/types";
import { EventAdapter } from "./eventAdapter";

function makePayload(overrides: Record<string, unknown> = {}): string {
	return JSON.stringify({
		room: "test-room",
		content: "/kick alice",
		messageId: "alice-12345",
		from: {
			identity: "alice",
			displayName: "Alice",
			role: "admin",
		},
		timestamp: 1000000,
		...overrides,
	});
}

describe("EventAdapter", () => {
	const adapter = new EventAdapter();

	describe("adaptBotMessage", () => {
		it("emits OnMessageReceived and AdapterMessage for bot:command", () => {
			const payload = makePayload();
			const events = adapter.adaptBotMessage(payload, EventType.AdapterMessage);

			expect(events).toHaveLength(2);

			// First: OnMessageReceived
			expect(events[0].eventType).toBe(EventType.OnMessageReceived);
			if (events[0].eventType === EventType.OnMessageReceived) {
				expect(events[0].content).toBe("/kick alice");
				expect(events[0].isCommand).toBe(true);
				expect(events[0].rawCommand?.name).toBe("kick");
				expect(events[0].rawCommand?.args).toEqual(["alice"]);
				expect(events[0].sender.identity).toBe("alice");
				expect(events[0].sender.name).toBe("Alice");
				expect(events[0].room.name).toBe("test-room");
			}

			// Second: AdapterMessage
			expect(events[1].eventType).toBe(EventType.AdapterMessage);
		});

		it("handles non-command text", () => {
			const payload = makePayload({ content: "hello everyone" });
			const events = adapter.adaptBotMessage(payload, EventType.AdapterMessage);

			expect(events).toHaveLength(2);
			if (events[0].eventType === EventType.OnMessageReceived) {
				expect(events[0].isCommand).toBe(false);
				expect(events[0].rawCommand).toBeUndefined();
			}
		});

		it("handles bot:message with replyTo", () => {
			const payload = makePayload({
				content: "thanks!",
				replyTo: "bob",
			});
			const events = adapter.adaptBotMessage(payload, EventType.AdapterMessage);

			expect(events).toHaveLength(2);
			if (events[0].eventType === EventType.OnMessageReceived) {
				expect(events[0].content).toBe("thanks!");
			}
		});

		it("returns empty for invalid JSON", () => {
			const events = adapter.adaptBotMessage(
				"not json",
				EventType.AdapterMessage,
			);
			expect(events).toHaveLength(0);
		});

		it("returns empty for empty content", () => {
			const payload = makePayload({ content: "" });
			const events = adapter.adaptBotMessage(payload, EventType.AdapterMessage);
			expect(events).toHaveLength(0);
		});

		it("parses !commands too", () => {
			const payload = makePayload({ content: "!help moderation" });
			const events = adapter.adaptBotMessage(payload, EventType.AdapterMessage);

			expect(events).toHaveLength(2);
			if (events[0].eventType === EventType.OnMessageReceived) {
				expect(events[0].isCommand).toBe(true);
				expect(events[0].rawCommand?.name).toBe("help");
				expect(events[0].rawCommand?.args).toEqual(["moderation"]);
			}
		});

		it("uses identity as fallback name when displayName is empty", () => {
			const payload = makePayload({
				from: { identity: "bot1", displayName: "", role: "bot" },
			});
			const events = adapter.adaptBotMessage(payload, EventType.AdapterMessage);

			expect(events).toHaveLength(2);
			if (events[0].eventType === EventType.OnMessageReceived) {
				expect(events[0].sender.name).toBe("bot1");
			}
		});
	});
});
