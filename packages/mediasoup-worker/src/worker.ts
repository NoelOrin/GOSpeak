import * as mediasoup from "mediasoup";

type Worker = Awaited<ReturnType<typeof mediasoup.createWorker>>;
type Router = Awaited<ReturnType<Worker["createRouter"]>>;
type WebRtcTransport = Awaited<ReturnType<Router["createWebRtcTransport"]>>;
type Producer = Awaited<ReturnType<WebRtcTransport["produce"]>>;
type Consumer = Awaited<ReturnType<WebRtcTransport["consume"]>>;

export interface ProducerInfo {
	id: string;
	kind: string;
	appData: unknown;
}

interface RoomState {
	router: Router;
	transports: Map<string, WebRtcTransport>;
	producers: Map<string, Producer>;
	consumers: Map<string, Consumer>;
}

class MediasoupWorker {
	private worker: Worker | null = null;
	private rooms: Map<string, RoomState> = new Map();

	async init(): Promise<void> {
		this.worker = await mediasoup.createWorker({
			logLevel: "warn",
			logTags: ["info", "ice", "dtls", "rtp", "srtp", "rtcp"],
			rtcMinPort: Number(process.env.RTC_MIN_PORT || 40000),
			rtcMaxPort: Number(process.env.RTC_MAX_PORT || 49999),
		});
		console.log(`[mediasoup] worker pid=${this.worker.pid}`);
		this.worker.observer.on("close", () => this.rooms.clear());
	}

	async createRouter(roomId: string): Promise<Router> {
		const existing = this.rooms.get(roomId);
		if (existing) return existing.router;
		if (!this.worker) throw new Error("mediasoup worker is not initialized");

		const router = await this.worker.createRouter({
			mediaCodecs: [
				{ kind: "audio", mimeType: "audio/opus", clockRate: 48000, channels: 2 },
				{ kind: "video", mimeType: "video/VP8", clockRate: 90000 },
				{ kind: "video", mimeType: "video/VP9", clockRate: 90000 },
				{
					kind: "video",
					mimeType: "video/H264",
					clockRate: 90000,
					parameters: { "packetization-mode": 1 },
				},
			],
		});
		const state: RoomState = {
			router,
			transports: new Map(),
			producers: new Map(),
			consumers: new Map(),
		};
		this.rooms.set(roomId, state);
		router.observer.on("close", () => this.rooms.delete(roomId));
		return router;
	}

	getRouter(roomId: string): Router | undefined {
		return this.rooms.get(roomId)?.router;
	}

	getRoom(roomId: string): RoomState | undefined {
		return this.rooms.get(roomId);
	}

	listRooms(): string[] {
		return Array.from(this.rooms.keys());
	}

	async createTransport(roomId: string): Promise<WebRtcTransport> {
		const room = this.rooms.get(roomId);
		if (!room) throw new Error("room not found");
		const announcedIp = process.env.ANNOUNCED_IP || undefined;
		const transport = await room.router.createWebRtcTransport({
			listenIps: [
				{
					ip: process.env.LISTEN_IP || "0.0.0.0",
					announcedIp,
				},
			],
			enableUdp: true,
			enableTcp: true,
			preferUdp: true,
			initialAvailableOutgoingBitrate: 1_000_000,
			maxSendMessageSize: 262_144,
		});
		room.transports.set(transport.id, transport);
		transport.observer.on("close", () => room.transports.delete(transport.id));
		return transport;
	}

	getTransport(roomId: string, transportId: string): WebRtcTransport | undefined {
		return this.rooms.get(roomId)?.transports.get(transportId);
	}

	addProducer(roomId: string, producer: Producer): void {
		const room = this.rooms.get(roomId);
		if (!room) return;
		room.producers.set(producer.id, producer);
		producer.observer.on("close", () => room.producers.delete(producer.id));
	}

	getProducer(roomId: string, producerId: string): Producer | undefined {
		return this.rooms.get(roomId)?.producers.get(producerId);
	}

	addConsumer(roomId: string, consumer: Consumer): void {
		const room = this.rooms.get(roomId);
		if (!room) return;
		room.consumers.set(consumer.id, consumer);
		consumer.observer.on("close", () => room.consumers.delete(consumer.id));
	}

	listProducers(roomId: string): ProducerInfo[] {
		const room = this.rooms.get(roomId);
		if (!room) return [];
		return Array.from(room.producers.values()).map((producer) => ({
			id: producer.id,
			kind: producer.kind,
			appData: producer.appData,
		}));
	}

	async closeRouter(roomId: string): Promise<void> {
		const room = this.rooms.get(roomId);
		if (room && !room.router.closed) room.router.close();
		this.rooms.delete(roomId);
	}

	async close(): Promise<void> {
		for (const [id, room] of this.rooms) {
			if (!room.router.closed) room.router.close();
			this.rooms.delete(id);
		}
		if (this.worker && !this.worker.closed) this.worker.close();
	}

	async getStats(): Promise<{ roomCount: number; workerAlive: boolean }> {
		return {
			roomCount: this.rooms.size,
			workerAlive: this.worker !== null && !this.worker.closed,
		};
	}
}

export default MediasoupWorker;
