import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

class MockWebSocket {
	static instances: MockWebSocket[] = [];
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	url: string;
	protocols: string[];
	readyState = MockWebSocket.CONNECTING;
	onopen: (() => void) | null = null;
	onmessage: ((ev: MessageEvent) => void) | null = null;
	onclose: ((ev: CloseEvent) => void) | null = null;
	onerror: (() => void) | null = null;

	constructor(url: string, protocols?: string[]) {
		this.url = url;
		this.protocols = protocols ?? [];
		MockWebSocket.instances.push(this);
	}

	send() {}

	close() {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.({ reason: "closed" } as CloseEvent);
	}
}

describe("wsClient", () => {
	beforeEach(() => {
		MockWebSocket.instances = [];
		vi.stubGlobal("WebSocket", MockWebSocket);
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
		MockWebSocket.instances = [];
	});

	it("refreshes the ws ticket before automatic reconnect", async () => {
		const { createWSClient } = await import("./wsClient");
		const refreshTicket = vi.fn().mockResolvedValue("fresh-ticket");
		const client = createWSClient({ refreshTicket });

		client.connect("ws://example.test/ws", "stale-ticket");
		expect(MockWebSocket.instances).toHaveLength(1);
		const first = MockWebSocket.instances[0];
		first.onclose?.({ reason: "network lost" } as CloseEvent);

		await vi.advanceTimersByTimeAsync(4000);

		expect(refreshTicket).toHaveBeenCalledTimes(1);
		expect(MockWebSocket.instances).toHaveLength(2);
		expect(MockWebSocket.instances[1].protocols).toContain("fresh-ticket");

		client.disconnect();
	});
});
