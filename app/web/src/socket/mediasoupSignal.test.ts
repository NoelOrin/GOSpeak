import { describe, expect, it, vi } from "vitest";
import { EVENTS } from "./events";
import { createMediasoupSignal } from "./mediasoupSignal";

describe("createMediasoupSignal", () => {
	it("emits router/transport/produce/consume over ack helper", async () => {
		const emitAck = vi.fn(
			async (event: string, payload?: Record<string, unknown>) => ({
				event,
				payload,
			}),
		);
		const onServerEvent = vi.fn(() => () => {});
		const api = createMediasoupSignal({ emitAck, onServerEvent });

		await api.getRouterCapabilities("r1");
		expect(emitAck).toHaveBeenCalledWith(EVENTS.SFU_GET_ROUTER_CAPABILITIES, {
			room: "r1",
		});

		await api.createTransport("r1", "send");
		expect(emitAck).toHaveBeenCalledWith(EVENTS.SFU_CREATE_TRANSPORT, {
			room: "r1",
			direction: "send",
		});

		await api.connectTransport("r1", "t1", { a: 1 });
		expect(emitAck).toHaveBeenCalledWith(EVENTS.SFU_CONNECT_TRANSPORT, {
			room: "r1",
			transportId: "t1",
			dtlsParameters: { a: 1 },
		});

		await api.produce("r1", "t1", "audio", { rtp: true }, { source: "mic" });
		expect(emitAck).toHaveBeenCalledWith(EVENTS.SFU_PRODUCE, {
			room: "r1",
			transportId: "t1",
			kind: "audio",
			rtpParameters: { rtp: true },
			appData: { source: "mic" },
		});

		await api.consume("r1", "t1", "p1", { caps: true });
		expect(emitAck).toHaveBeenCalledWith(EVENTS.SFU_CONSUME, {
			room: "r1",
			transportId: "t1",
			producerId: "p1",
			rtpCapabilities: { caps: true },
		});
	});

	it("fans out producer ready/closed listeners and supports unsubscribe", () => {
		const handlers = new Map<string, (data: unknown) => void>();
		const onServerEvent = vi.fn(
			(event: string, cb: (data: unknown) => void) => {
				handlers.set(event, cb);
				return () => handlers.delete(event);
			},
		);
		const api = createMediasoupSignal({
			emitAck: vi.fn(),
			onServerEvent: onServerEvent as any,
		});
		api.bindServerEvents();

		const ready = vi.fn();
		const closed = vi.fn();
		const offReady = api.onProducerReady(ready);
		const offClosed = api.onProducerClosed(closed);

		handlers.get(EVENTS.SFU_PRODUCER_READY)?.({ id: "p1" });
		handlers.get(EVENTS.SFU_PRODUCER_CLOSED)?.({ id: "p1" });
		expect(ready).toHaveBeenCalledWith({ id: "p1" });
		expect(closed).toHaveBeenCalledWith({ id: "p1" });

		offReady();
		offClosed();
		handlers.get(EVENTS.SFU_PRODUCER_READY)?.({ id: "p2" });
		handlers.get(EVENTS.SFU_PRODUCER_CLOSED)?.({ id: "p2" });
		expect(ready).toHaveBeenCalledTimes(1);
		expect(closed).toHaveBeenCalledTimes(1);
	});

	it("clearListeners drops all producer listeners", () => {
		const handlers = new Map<string, (data: unknown) => void>();
		const onServerEvent = vi.fn(
			(event: string, cb: (data: unknown) => void) => {
				handlers.set(event, cb);
				return () => handlers.delete(event);
			},
		);
		const api = createMediasoupSignal({
			emitAck: vi.fn(),
			onServerEvent: onServerEvent as any,
		});
		api.bindServerEvents();
		const ready = vi.fn();
		api.onProducerReady(ready);
		api.clearListeners();
		handlers.get(EVENTS.SFU_PRODUCER_READY)?.({ id: "p3" });
		expect(ready).not.toHaveBeenCalled();
	});
});
