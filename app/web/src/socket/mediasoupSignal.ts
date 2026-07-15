import { EVENTS } from "./events";

export type MediasoupSignalDeps = {
	emitAck: (event: string, payload?: Record<string, unknown>) => Promise<any>;
	onServerEvent: (event: string, cb: (data: any) => void) => () => void;
};

export function createMediasoupSignal(deps: MediasoupSignalDeps) {
	const producerReadyListeners = new Set<(info: any) => void>();
	const producerClosedListeners = new Set<(info: any) => void>();

	function bindServerEvents() {
		deps.onServerEvent(EVENTS.SFU_PRODUCER_READY, (info: any) => {
			for (const listener of producerReadyListeners) listener(info);
		});
		deps.onServerEvent(EVENTS.SFU_PRODUCER_CLOSED, (info: any) => {
			for (const listener of producerClosedListeners) listener(info);
		});
	}

	function clearListeners() {
		producerReadyListeners.clear();
		producerClosedListeners.clear();
	}

	return {
		bindServerEvents,
		clearListeners,
		getRouterCapabilities(room: string) {
			return deps.emitAck(EVENTS.SFU_GET_ROUTER_CAPABILITIES, { room });
		},
		createTransport(room: string, direction: "send" | "recv") {
			return deps.emitAck(EVENTS.SFU_CREATE_TRANSPORT, { room, direction });
		},
		connectTransport(
			room: string,
			transportId: string,
			dtlsParameters: unknown,
		) {
			return deps.emitAck(EVENTS.SFU_CONNECT_TRANSPORT, {
				room,
				transportId,
				dtlsParameters,
			});
		},
		produce(
			room: string,
			transportId: string,
			kind: string,
			rtpParameters: unknown,
			appData: unknown,
		) {
			return deps.emitAck(EVENTS.SFU_PRODUCE, {
				room,
				transportId,
				kind,
				rtpParameters,
				appData,
			});
		},
		consume(
			room: string,
			transportId: string,
			producerId: string,
			rtpCapabilities: unknown,
		) {
			return deps.emitAck(EVENTS.SFU_CONSUME, {
				room,
				transportId,
				producerId,
				rtpCapabilities,
			});
		},
		onProducerReady(cb: (info: any) => void) {
			producerReadyListeners.add(cb);
			return () => {
				producerReadyListeners.delete(cb);
			};
		},
		onProducerClosed(cb: (info: any) => void) {
			producerClosedListeners.add(cb);
			return () => {
				producerClosedListeners.delete(cb);
			};
		},
	};
}
