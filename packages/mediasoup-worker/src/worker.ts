import * as mediasoup from "mediasoup";

type Worker = Awaited<ReturnType<typeof mediasoup.createWorker>>;
type Router = Awaited<ReturnType<Worker["createRouter"]>>;
type WebRtcTransport = Awaited<ReturnType<Router["createWebRtcTransport"]>>;
type Producer = Awaited<ReturnType<WebRtcTransport["produce"]>>;
type Consumer = Awaited<ReturnType<WebRtcTransport["consume"]>>;

type MediasoupFactory = Pick<typeof mediasoup, "createWorker">;

export interface ProducerInfo {
	id: string;
	kind: string;
	appData: unknown;
}

export type RoomStatus = "open" | "closed";
export type ParticipantStatus = "joined" | "closed";

interface ParticipantState {
	sendTransportId?: string;
	recvTransportId?: string;
	producerIds: Set<string>;
	status: ParticipantStatus;
}

interface RoomState {
	status: RoomStatus;
	router: Router;
	transports: Map<string, WebRtcTransport>;
	producers: Map<string, Producer>;
	consumers: Map<string, Consumer>;
	participants: Map<string, ParticipantState>;
}

class MediasoupWorker {
	private readonly mediasoupFactory: MediasoupFactory;

	constructor(mediasoupFactory: MediasoupFactory = mediasoup) {
		this.mediasoupFactory = mediasoupFactory;
	}

	private worker: Worker | null = null;
	private rooms: Map<string, RoomState> = new Map();
	private routerPromises = new Map<string, Promise<Router>>();

	async init(): Promise<void> {
		this.worker = await this.mediasoupFactory.createWorker({
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
		const inflight = this.routerPromises.get(roomId);
		if (inflight) return inflight;

		const promise = this.worker
			.createRouter({
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
			})
			.then((router) => {
				const state: RoomState = {
					status: "open",
					router,
					transports: new Map(),
					producers: new Map(),
					consumers: new Map(),
					participants: new Map(),
				};
				this.rooms.set(roomId, state);
				router.observer.on("close", () => this.rooms.delete(roomId));
				return router;
			})
			.finally(() => {
				this.routerPromises.delete(roomId);
			});
		this.routerPromises.set(roomId, promise);
		return promise;
	}

	getRouter(roomId: string): Router | undefined {
		return this.rooms.get(roomId)?.router;
	}

	getRoom(roomId: string): RoomState | undefined {
		return this.rooms.get(roomId);
	}

	private closeRoomResources(room: RoomState): void {
		room.status = "closed";
		for (const transport of room.transports.values()) {
			if (!transport.closed) transport.close();
		}
		for (const producer of room.producers.values()) {
			if (!producer.closed) producer.close();
		}
		for (const consumer of room.consumers.values()) {
			if (!consumer.closed) consumer.close();
		}
		for (const participant of room.participants.values()) {
			participant.status = "closed";
		}
		room.transports.clear();
		room.producers.clear();
		room.consumers.clear();
		room.participants.clear();
	}

	listRooms(): string[] {
		return Array.from(this.rooms.keys());
	}

	async createTransport(roomId: string, identity?: string, direction?: "send" | "recv"): Promise<WebRtcTransport> {
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
		if (identity) {
			let participant = room.participants.get(identity);
			if (!participant) {
				participant = { producerIds: new Set(), status: "joined" };
				room.participants.set(identity, participant);
			}
			if (direction === "send") participant.sendTransportId = transport.id;
			else if (direction === "recv") participant.recvTransportId = transport.id;
		}
		return transport;
	}

	getTransport(roomId: string, transportId: string): WebRtcTransport | undefined {
		return this.rooms.get(roomId)?.transports.get(transportId);
	}

	addProducer(roomId: string, producer: Producer): void {
		const room = this.rooms.get(roomId);
		if (!room) return;
		room.producers.set(producer.id, producer);
		const identity = (producer.appData as { identity?: string })?.identity;
		if (identity) {
			let participant = room.participants.get(identity);
			if (!participant) {
				participant = { producerIds: new Set(), status: "joined" };
				room.participants.set(identity, participant);
			}
			participant.producerIds.add(producer.id);
		}
		producer.observer.on("close", () => {
			room.producers.delete(producer.id);
			if (identity) {
				room.participants.get(identity)?.producerIds.delete(producer.id);
			}
		});
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

	getParticipant(roomId: string, identity: string): ParticipantState | undefined {
		return this.rooms.get(roomId)?.participants.get(identity);
	}

	listParticipants(roomId: string): Array<{ identity: string; producerCount: number; hasSendTransport: boolean; hasRecvTransport: boolean; status: ParticipantStatus }> {
		const room = this.rooms.get(roomId);
		if (!room) return [];
		return Array.from(room.participants.entries()).map(([identity, p]) => ({
			identity,
			producerCount: p.producerIds.size,
			hasSendTransport: p.sendTransportId !== undefined,
			hasRecvTransport: p.recvTransportId !== undefined,
			status: p.status,
		}));
	}

	closeParticipant(roomId: string, identity: string): string[] {
		const room = this.rooms.get(roomId);
		if (!room) return [];
		const participant = room.participants.get(identity);
		if (!participant) return [];
		if (participant.status === "closed") return [];
		participant.status = "closed";
		const closedProducerIds: string[] = [];
		for (const pid of participant.producerIds) {
			const producer = room.producers.get(pid);
			if (producer && !producer.closed) {
				producer.close();
				closedProducerIds.push(pid);
			}
		}
		if (participant.sendTransportId) {
			const t = room.transports.get(participant.sendTransportId);
			if (t && !t.closed) t.close();
		}
		if (participant.recvTransportId) {
			const t = room.transports.get(participant.recvTransportId);
			if (t && !t.closed) t.close();
		}
		room.participants.delete(identity);
		return closedProducerIds;
	}

	pauseProducer(roomId: string, producerId: string): void {
		const producer = this.rooms.get(roomId)?.producers.get(producerId);
		if (producer && !producer.closed) producer.pause();
	}

	resumeProducer(roomId: string, producerId: string): void {
		const producer = this.rooms.get(roomId)?.producers.get(producerId);
		if (producer && !producer.closed) producer.resume();
	}

	pauseParticipant(roomId: string, identity: string): void {
		const room = this.rooms.get(roomId);
		if (!room) return;
		const participant = room.participants.get(identity);
		if (!participant) return;
		for (const pid of participant.producerIds) {
			const producer = room.producers.get(pid);
			if (producer && !producer.closed) producer.pause();
		}
	}

	resumeParticipant(roomId: string, identity: string): void {
		const room = this.rooms.get(roomId);
		if (!room) return;
		const participant = room.participants.get(identity);
		if (!participant) return;
		for (const pid of participant.producerIds) {
			const producer = room.producers.get(pid);
			if (producer && !producer.closed) producer.resume();
		}
	}

	async closeRouter(roomId: string): Promise<void> {
		const room = this.rooms.get(roomId);
		if (!room) return;
		this.closeRoomResources(room);
		this.rooms.delete(roomId);
		if (!room.router.closed) room.router.close();
	}

	async close(): Promise<void> {
		for (const roomId of Array.from(this.rooms.keys())) {
			await this.closeRouter(roomId);
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
