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

	it("emitFireAndForget reports whether the event was actually sent", async () => {
		const { createWSClient } = await import("./wsClient");
		const client = createWSClient();

		expect(client.emitFireAndForget("room:create", {})).toBe(false);

		client.connect("ws://example.test/ws");
		const ws = MockWebSocket.instances[0];
		ws.readyState = MockWebSocket.OPEN;
		ws.onopen?.();
		expect(client.emitFireAndForget("room:create", {})).toBe(true);

		client.disconnect();
	});

	it("keeps worker query when connecting through the cluster entry", async () => {
		const { createWSClient } = await import("./wsClient");
		const client = createWSClient();

		client.connect("https://voice.example.com/ws?worker=worker-1", "ticket");
		expect(MockWebSocket.instances[0].url).toBe(
			"wss://voice.example.com/ws?worker=worker-1",
		);

		client.disconnect();
	});

	it("appends /ws while preserving existing query params", async () => {
		const { createWSClient } = await import("./wsClient");
		const client = createWSClient();

		client.connect("https://voice.example.com?worker=worker-2", "ticket");
		expect(MockWebSocket.instances[0].url).toBe(
			"wss://voice.example.com/ws?worker=worker-2",
		);

		client.disconnect();
	});

	it("refreshes the ws ticket before automatic reconnect", async () => {
		const { createWSClient } = await import("./wsClient");
		const refreshTicket = vi.fn().mockResolvedValue({ token: "fresh-ticket" });
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

	it("re-resolves worker URL on reconnect", async () => {
		const { createWSClient } = await import("./wsClient");
		const refreshTicket = vi.fn().mockResolvedValue({
			url: "wss://new-worker/ws",
			token: "fresh-ticket",
		});
		const client = createWSClient({ refreshTicket });

		client.connect("wss://old-worker/ws", "stale-ticket");
		expect(MockWebSocket.instances[0].url).toBe("wss://old-worker/ws");
		MockWebSocket.instances[0].onclose?.({
			reason: "worker changed",
		} as CloseEvent);

		await vi.advanceTimersByTimeAsync(4000);

		expect(refreshTicket).toHaveBeenCalledTimes(1);
		expect(MockWebSocket.instances).toHaveLength(2);
		expect(MockWebSocket.instances[1].url).toBe("wss://new-worker/ws");
		expect(client.getCurrentUrl()).not.toContain("old-worker");

		client.disconnect();
	});

	it("exposes explicit connection state transitions", async () => {
		const { createWSClient } = await import("./wsClient");
		const client = createWSClient();
		const transitions: Array<[string, string]> = [];
		client.onStateChange((prev, next) => transitions.push([prev, next]));

		expect(client.getState()).toBe("new");

		client.connect("ws://example.test/ws");
		expect(client.getState()).toBe("connecting");

		const ws = MockWebSocket.instances[0];
		ws.onopen?.();
		expect(client.getState()).toBe("open");
		expect(transitions).toContainEqual(["new", "connecting"]);
		expect(transitions).toContainEqual(["connecting", "open"]);

		client.disconnect();
		expect(client.getState()).toBe("closed");
		expect(transitions).toContainEqual(["closing", "closed"]);
	});

	it("allows closed state to reconnect but rejects backward transitions", async () => {
		const { createWSClient } = await import("./wsClient");
		const client = createWSClient();
		const transitions: Array<[string, string]> = [];
		client.onStateChange((prev, next) => transitions.push([prev, next]));

		client.connect("ws://example.test/ws");
		const first = MockWebSocket.instances[0];
		first.onopen?.();
		first.onclose?.({ reason: "network lost" } as CloseEvent);
		expect(client.getState()).toBe("closed");

		client.connect("ws://example.test/ws");
		expect(client.getState()).toBe("connecting");
		const second = MockWebSocket.instances[1];
		second.onopen?.();
		expect(client.getState()).toBe("open");

		client.disconnect();
		expect(client.getState()).toBe("closed");
		client.disconnect();
		expect(client.getState()).toBe("closed");
		expect(transitions).not.toContainEqual(["closed", "closing"]);
	});
});
