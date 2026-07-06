import { Device } from "mediasoup-client";
import type { types as mediasoupTypes } from "mediasoup-client";
import type {
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUClient,
	SFUClientOptions,
} from "./types";

const MEDIASOUP_EVENTS = {
	GET_ROUTER_CAPABILITIES: "sfu:get-router-capabilities",
	CREATE_TRANSPORT: "sfu:create-transport",
	CONNECT_TRANSPORT: "sfu:connect-transport",
	PRODUCE: "sfu:produce",
	CONSUME: "sfu:consume",
	PRODUCER_READY: "sfu:producer-ready",
	PRODUCER_CLOSED: "sfu:producer-closed",
} as const;

class MediaSoupRemoteAudioTrack implements RemoteAudioTrackLike {
	private elements: HTMLAudioElement[] = [];
	private audioContext: AudioContext;
	private gainNode: GainNode;
	private analyser: AnalyserNode;
	private levelBuffer: Uint8Array;

	constructor(private consumer: mediasoupTypes.Consumer) {
		this.audioContext = new AudioContext();
		this.gainNode = this.audioContext.createGain();
		this.gainNode.gain.value = 1;
		this.analyser = this.audioContext.createAnalyser();
		this.analyser.fftSize = 512;
		this.levelBuffer = new Uint8Array(this.analyser.fftSize);
		this.analyser.connect(this.gainNode);
		this.gainNode.connect(this.audioContext.destination);
	}

	attach(): HTMLMediaElement {
		const audioElement = document.createElement("audio");
		audioElement.autoplay = true;
		const source = this.audioContext.createMediaStreamSource(
			new MediaStream([this.consumer.track]),
		);
		source.connect(this.analyser);
		this.elements.push(audioElement);
		return audioElement;
	}

	detach(): HTMLMediaElement[] {
		const detached = [...this.elements];
		this.elements = [];
		for (const element of detached) {
			element.pause();
			element.remove();
		}
		return detached;
	}

	setVolume(volume: number): void {
		this.gainNode.gain.value = Math.max(0, Math.min(1, volume));
	}

	getLevel(): number {
		this.analyser.getByteTimeDomainData(this.levelBuffer as Uint8Array<ArrayBuffer>);
		let sumSquares = 0;
		for (let i = 0; i < this.levelBuffer.length; i++) {
			const v = (this.levelBuffer[i] - 128) / 128;
			sumSquares += v * v;
		}
		return Math.sqrt(sumSquares / this.levelBuffer.length);
	}

	stop(): void {
		this.consumer.close();
		this.detach();
		this.audioContext.close();
	}
}

export class MediaSoupSFUClient implements SFUClient {
	private device: Device | null = null;
	private sendTransport: mediasoupTypes.Transport | null = null;
	private recvTransport: mediasoupTypes.Transport | null = null;
	private localTrack: MediaStreamTrack | null = null;
	private producer: mediasoupTypes.Producer | null = null;
	private remoteTracks = new Map<string, MediaSoupRemoteAudioTrack>();
	private roomId = "";
	private identity = "";
	private socket?: any;
	private onActiveSpeakersCb?: (ids: string[]) => void;
	private onRemoteAudioTrackCb?: (info: RemoteTrackInfo) => void;
	private onRemoteAudioTrackRemovedCb?: (identity: string) => void;
	private onDisconnectedCb?: () => void;
	private onReconnectingCb?: () => void;
	private onReconnectedCb?: () => void;
	private onSocketDisconnectBound?: () => void;
	private hasJoined = false;
	private isReconnecting = false;
	private onProducerReadyBound?: (info: any) => void;
	private onProducerClosedBound?: (info: any) => void;
	private onSendConnStateChangeBound?: (state: string) => void;
	private onRecvConnStateChangeBound?: (state: string) => void;
	private activeSpeakerTimer: ReturnType<typeof setInterval> | null = null;

	constructor(private options: SFUClientOptions = {}) {
		this.socket = options.socket;
	}

	private sfuEmit(event: string, payload: Record<string, unknown>): Promise<any> {
		return new Promise((resolve, reject) => {
			if (!this.socket) {
				reject(new Error("mediasoup socket not connected"));
				return;
			}
			this.socket.emit(event, JSON.stringify(payload), (resp: string) => {
				try {
					const parsed = JSON.parse(resp);
					if (parsed?.error) reject(new Error(parsed.error));
					else resolve(parsed);
				} catch {
					reject(new Error("invalid mediasoup response: " + resp));
				}
			});
		});
	}

	async joinRoom(
		token: string,
		_url: string,
		identity: string,
		room?: string,
	): Promise<void> {
		if (!this.socket) throw new Error("mediasoup client requires a socket.io socket");

		const [tokenRoom, tokenIdentity] = token.split(":", 2);
		this.roomId = room || tokenRoom;
		this.identity = tokenIdentity || identity;

		this.device = new Device();
		const { rtpCapabilities } = await this.sfuEmit(MEDIASOUP_EVENTS.GET_ROUTER_CAPABILITIES, { room: this.roomId });
		await this.device.load({ routerRtpCapabilities: rtpCapabilities as never });

		this.sendTransport = await this.createSendTransport();
		this.recvTransport = await this.createRecvTransport();
		this.localTrack = await this.createLocalAudioTrack();
		this.producer = await this.sendTransport.produce({
			track: this.localTrack,
			appData: { identity: this.identity },
		});

		this.onProducerReadyBound = (info: any) => {
			if (info.kind !== "audio") return;
			const id = info.appData?.identity;
			if (!id || id === this.identity) return;
			void this.consumeProducer(info.producerId, id);
		};

		this.onProducerClosedBound = (info: any) => {
			const id = info.identity;
			if (!id) return;
			const track = this.remoteTracks.get(id);
			if (!track) return;
			track.stop();
			this.remoteTracks.delete(id);
			this.onRemoteAudioTrackRemovedCb?.(id);
		};

		this.socket.on(MEDIASOUP_EVENTS.PRODUCER_READY, this.onProducerReadyBound);
		this.socket.on(MEDIASOUP_EVENTS.PRODUCER_CLOSED, this.onProducerClosedBound);
		this.hasJoined = true;
		this.onSocketDisconnectBound = () => {
			if (this.hasJoined) {
				this.hasJoined = false;
				this.isReconnecting = false;
				this.onDisconnectedCb?.();
			}
		};
		this.socket.on("disconnect", this.onSocketDisconnectBound);

		// transport 连接状态：disconnected→reconnecting（等 ICE 自恢复），failed/closed→disconnected，恢复→reconnected
		// TODO: 后端未实现 restartIce 端点，目前不主动重启 ICE，依赖 WebRtcTransport 默认超时行为
		const handleConnState = (state: string) => {
			if (!this.hasJoined) return;
			if (state === "failed" || state === "closed") {
				this.hasJoined = false;
				this.isReconnecting = false;
				this.onDisconnectedCb?.();
				return;
			}
			if (state === "disconnected" && !this.isReconnecting) {
				this.isReconnecting = true;
				this.onReconnectingCb?.();
			} else if (state === "connected" && this.isReconnecting) {
				const sendUp = this.sendTransport?.connectionState === "connected";
				const recvUp = this.recvTransport?.connectionState === "connected";
				if (sendUp && recvUp) {
					this.isReconnecting = false;
					this.onReconnectedCb?.();
				}
			}
		};
		this.onSendConnStateChangeBound = handleConnState;
		this.onRecvConnStateChangeBound = handleConnState;
		this.sendTransport?.on("connectionstatechange", this.onSendConnStateChangeBound);
		this.recvTransport?.on("connectionstatechange", this.onRecvConnStateChangeBound);
		this.activeSpeakerTimer = setInterval(() => {
			if (this.remoteTracks.size === 0) return;
			let loudest: { identity: string; level: number } | null = null;
			for (const [identity, track] of this.remoteTracks) {
				const level = track.getLevel();
				if (!loudest || level > loudest.level) {
					loudest = { identity, level };
				}
			}
			if (loudest && loudest.level > 0.01) {
				this.onActiveSpeakersCb?.([loudest.identity]);
			} else {
				this.onActiveSpeakersCb?.([]);
			}
		}, 500);
	}

	async leaveRoom(): Promise<void> {
		this.hasJoined = false;
		if (this.activeSpeakerTimer) {
			clearInterval(this.activeSpeakerTimer);
			this.activeSpeakerTimer = null;
		}
		this.isReconnecting = false;
		if (this.socket) {
			if (this.onProducerReadyBound) {
				this.socket.off(MEDIASOUP_EVENTS.PRODUCER_READY, this.onProducerReadyBound);
			}
			if (this.onProducerClosedBound) {
				this.socket.off(MEDIASOUP_EVENTS.PRODUCER_CLOSED, this.onProducerClosedBound);
			}
			if (this.onSocketDisconnectBound) {
				this.socket.off("disconnect", this.onSocketDisconnectBound);
			}
		}
		this.onProducerReadyBound = undefined;
		this.onProducerClosedBound = undefined;
		this.onSocketDisconnectBound = undefined;
		this.onReconnectingCb = undefined;
		this.onReconnectedCb = undefined;
		if (this.sendTransport && this.onSendConnStateChangeBound) {
			this.sendTransport.off("connectionstatechange", this.onSendConnStateChangeBound);
		}
		if (this.recvTransport && this.onRecvConnStateChangeBound) {
			this.recvTransport.off("connectionstatechange", this.onRecvConnStateChangeBound);
		}
		this.onSendConnStateChangeBound = undefined;
		this.onRecvConnStateChangeBound = undefined;
		this.producer?.close();
		this.producer = null;
		this.localTrack?.stop();
		this.localTrack = null;
		this.remoteTracks.forEach((track) => track.stop());
		this.remoteTracks.clear();
		this.sendTransport?.close();
		this.recvTransport?.close();
		this.sendTransport = null;
		this.recvTransport = null;
	}

	async setMicEnabled(enabled: boolean): Promise<void> {
		if (!this.producer) return;
		if (enabled) {
			if (this.localTrack) this.localTrack.enabled = true;
			await this.producer.resume();
			return;
		}
		if (this.localTrack) this.localTrack.enabled = false;
		await this.producer.pause();
	}

	onActiveSpeakers(cb: (ids: string[]) => void): void {
		this.onActiveSpeakersCb = cb;
	}

	onRemoteAudioTrack(cb: (info: RemoteTrackInfo) => void): void {
		this.onRemoteAudioTrackCb = cb;
	}

	onRemoteAudioTrackRemoved(cb: (identity: string) => void): void {
		this.onRemoteAudioTrackRemovedCb = cb;
	}

	getExistingRemoteAudioTracks(): RemoteTrackInfo[] {
		return Array.from(this.remoteTracks.entries()).map(([identity, track]) => ({
			identity,
			track,
		}));
	}

	onDisconnected(cb: () => void): void {
		this.onDisconnectedCb = cb;
	}

	onReconnecting(cb: () => void): void {
		this.onReconnectingCb = cb;
	}

	onReconnected(cb: () => void): void {
		this.onReconnectedCb = cb;
	}

	async destroy(): Promise<void> {
		try {
			await this.leaveRoom();
		} catch (err) {
			console.warn("[MediaSoup] destroy error:", err);
		}
	}

	private async createSendTransport(): Promise<mediasoupTypes.Transport> {
		if (!this.device || !this.socket) throw new Error("mediasoup device or socket not initialized");
		const data = await this.sfuEmit(MEDIASOUP_EVENTS.CREATE_TRANSPORT, { room: this.roomId, direction: "send", identity: this.identity });
		const transport = this.device.createSendTransport(data as never);
		transport.on("connect", ({ dtlsParameters }, callback, errback) => {
			if (!this.socket) return;
			this.sfuEmit(MEDIASOUP_EVENTS.CONNECT_TRANSPORT, {
				room: this.roomId,
				transportId: data.id as string,
				dtlsParameters,
			}).then(callback).catch(errback);
		});
		transport.on("produce", ({ kind, rtpParameters, appData }, callback, errback) => {
			if (!this.socket) return;
			this.sfuEmit(MEDIASOUP_EVENTS.PRODUCE, {
				room: this.roomId,
				transportId: data.id as string,
				kind,
				rtpParameters,
				appData,
			})
				.then(({ id }) => callback({ id }))
				.catch(errback);
		});
		return transport;
	}

	private async createRecvTransport(): Promise<mediasoupTypes.Transport> {
		if (!this.device || !this.socket) throw new Error("mediasoup device or socket not initialized");
		const data = await this.sfuEmit(MEDIASOUP_EVENTS.CREATE_TRANSPORT, { room: this.roomId, direction: "recv", identity: this.identity });
		const transport = this.device.createRecvTransport(data as never);
		transport.on("connect", ({ dtlsParameters }, callback, errback) => {
			if (!this.socket) return;
			this.sfuEmit(MEDIASOUP_EVENTS.CONNECT_TRANSPORT, {
				room: this.roomId,
				transportId: data.id as string,
				dtlsParameters,
			}).then(callback).catch(errback);
		});
		return transport;
	}

	private async createLocalAudioTrack(): Promise<MediaStreamTrack> {
		const stream = await navigator.mediaDevices.getUserMedia({
			audio: {
				echoCancellation: this.options.audioCapture?.echoCancellation ?? true,
				noiseSuppression: this.options.audioCapture?.noiseSuppression ?? true,
				autoGainControl: this.options.audioCapture?.autoGainControl ?? true,
				sampleRate: this.options.audioCapture?.sampleRate,
				sampleSize: this.options.audioCapture?.sampleSize,
				channelCount: this.options.audioCapture?.channelCount,
			},
		});
		return stream.getAudioTracks()[0];
	}

	private async consumeProducer(producerId: string, identity: string): Promise<void> {
		if (!this.device || !this.recvTransport || !this.socket) return;
		if (this.remoteTracks.has(identity)) return;
		try {
			const consumerData = await this.sfuEmit(MEDIASOUP_EVENTS.CONSUME, {
				room: this.roomId,
				transportId: this.recvTransport.id,
				producerId,
				rtpCapabilities: this.device.rtpCapabilities,
			});
			const consumer = await this.recvTransport.consume(consumerData as never);
			const track = new MediaSoupRemoteAudioTrack(consumer);
			this.remoteTracks.set(identity, track);
			this.onRemoteAudioTrackCb?.({ identity, track });
		} catch (err) {
			console.warn("[MediaSoup] consumeProducer failed:", err);
			this.onRemoteAudioTrackRemovedCb?.(identity);
		}
	}
}
