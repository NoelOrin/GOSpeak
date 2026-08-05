import type {
	JoinParams,
	PeerStream,
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUClient,
	SFUClientOptions,
	SignalSocket,
} from "./types";

type SessionDescription = { type: RTCSdpType | string; sdp: string };

type CFTokenPayload = {
	appId?: string;
	sessionId?: string;
	stunUrl?: string;
	room?: string;
	identity?: string;
};

type TracksResponse = {
	sessionDescription?: SessionDescription;
	tracks?: Array<{
		trackName?: string;
		mid?: string;
		location?: string;
		sessionId?: string;
		errorCode?: string;
		errorDescription?: string;
	}>;
	requiresImmediateRenegotiation?: boolean;
	errorCode?: string;
	errorDescription?: string;
};

type APIResult<T> = {
	code: number;
	msg: string;
	data?: T;
};

const LOCAL_TRACK_NAME = "audio";

class CloudflareRemoteAudioTrack implements RemoteAudioTrackLike {
	private elements: HTMLAudioElement[] = [];

	constructor(private readonly stream: MediaStream) {}

	attach(): HTMLMediaElement {
		const element = document.createElement("audio");
		element.autoplay = true;
		element.srcObject = this.stream;
		this.elements.push(element);
		return element;
	}

	detach(): HTMLMediaElement[] {
		const detached = [...this.elements];
		this.elements = [];
		for (const element of detached) {
			element.pause();
			element.srcObject = null;
			element.remove();
		}
		return detached;
	}

	setVolume(volume: number): void {
		for (const element of this.elements) {
			element.volume = Math.max(0, Math.min(1, volume));
		}
	}
}

function readAccessToken(): string {
	try {
		return localStorage.getItem("accessToken") || "";
	} catch {
		return "";
	}
}

function parseToken(raw: string): CFTokenPayload {
	try {
		const parsed = JSON.parse(raw) as CFTokenPayload;
		if (parsed && typeof parsed === "object") return parsed;
	} catch {
		// ignore
	}
	return {};
}

function stunServers(stunUrl?: string): RTCIceServer[] {
	const host = (stunUrl || "stun.cloudflare.com:3478").replace(/^stun:/i, "");
	return [{ urls: `stun:${host}` }];
}

export class CloudflareSFUClient implements SFUClient {
	private pc: RTCPeerConnection | null = null;
	private localTrack: MediaStreamTrack | null = null;
	private remoteTracks = new Map<string, CloudflareRemoteAudioTrack>();
	private peerSessions = new Map<string, string>();
	/** mid → identity for remote track mapping from tracks/new responses */
	private midToIdentity = new Map<string, string>();
	private onActiveSpeakersCb?: (ids: string[]) => void;
	private onLocalSpeakingChangeCb?: (speaking: boolean) => void;
	private onRemoteAudioTrackCb?: (info: RemoteTrackInfo) => void;
	private onRemoteAudioTrackRemovedCb?: (identity: string) => void;
	private onDisconnectedCb?: () => void;
	private onReconnectingCb?: () => void;
	private onReconnectedCb?: () => void;
	private hasJoined = false;
	private leaving = false;
	private joining = false;
	private isReconnecting = false;
	private identity = "";
	private room = "";
	private sessionId = "";
	private appId = "";
	private stunUrl = "";
	private socket?: SignalSocket | null;
	private onPcStateChangeBound?: () => void;
	private socketMemberJoinedCleanup?: () => void;
	private socketMemberLeftCleanup?: () => void;
	private activeSpeakerTimer: ReturnType<typeof setInterval> | null = null;
	private analyzerContext: AudioContext | null = null;
	private analyzer?: AnalyserNode | null = null;
	private analyzedTrack: MediaStreamTrack | null = null;
	private micEnabled = true;

	constructor(private readonly options: SFUClientOptions = {}) {
		this.socket = options.socket;
	}

	async joinRoom(params: JoinParams): Promise<void> {
		if (this.hasJoined && !this.leaving && this.pc) return;
		if (this.joining) return;
		this.joining = true;

		this.leaving = false;
		const payload = parseToken(params.token);
		this.sessionId = payload.sessionId || params.stream || "";
		this.appId = payload.appId || "";
		this.stunUrl = payload.stunUrl || "";
		this.identity = payload.identity || params.identity || "";
		this.room = payload.room || params.room || "";
		this.micEnabled = true;

		if (!this.sessionId) {
			throw new Error("cloudflare sessionId missing in join token");
		}

		this.pc = new RTCPeerConnection({ iceServers: stunServers(this.stunUrl) });
		this.pc.ontrack = (event) => {
			const identity = this.identityFromTrackEvent(event);
			const stream = event.streams?.[0] || new MediaStream([event.track]);
			if (!identity) return;
			const remote = new CloudflareRemoteAudioTrack(stream);
			this.remoteTracks.set(identity, remote);
			this.onRemoteAudioTrackCb?.({ identity, track: remote });
			event.track.addEventListener("ended", () => {
				remote.detach();
				this.remoteTracks.delete(identity);
				this.onRemoteAudioTrackRemovedCb?.(identity);
			});
		};

		this.onPcStateChangeBound = () => {
			if (!this.hasJoined || !this.pc) return;
			const state = this.pc.iceConnectionState;
			if (state === "failed") {
				this.hasJoined = false;
				this.isReconnecting = false;
				this.onDisconnectedCb?.();
				return;
			}
			if (state === "disconnected" && !this.isReconnecting) {
				this.isReconnecting = true;
				this.onReconnectingCb?.();
			} else if (
				(state === "connected" || state === "completed") &&
				this.isReconnecting
			) {
				this.isReconnecting = false;
				this.onReconnectedCb?.();
			}
		};
		this.pc.addEventListener("iceconnectionstatechange", this.onPcStateChangeBound);

		try {
			this.localTrack = await this.createLocalAudioTrack();
			const sender = this.pc.addTrack(this.localTrack);
			const transceiver = this.pc
				.getTransceivers()
				.find((t) => t.sender === sender);
			const mid = transceiver?.mid || "0";

			const offer = await this.pc.createOffer();
			await this.pc.setLocalDescription(offer);
			await this.waitIceGatheringComplete(this.pc);

			const localDesc = this.pc.localDescription;
			if (!localDesc?.sdp) throw new Error("cloudflare local SDP missing");

			const tracksResp = await this.proxyJSON<TracksResponse>(
				`/api/v1/signal/cloudflare/sessions/${encodeURIComponent(this.sessionId)}/tracks/new`,
				"POST",
				{
					sessionDescription: {
						type: localDesc.type,
						sdp: localDesc.sdp,
					},
					tracks: [
						{
							location: "local",
							mid,
							trackName: LOCAL_TRACK_NAME,
						},
					],
				},
			);

			if (tracksResp.errorCode) {
				throw new Error(
					tracksResp.errorDescription || tracksResp.errorCode || "add tracks failed",
				);
			}
			const answer = tracksResp.sessionDescription;
			if (!answer?.sdp) {
				throw new Error("cloudflare answer SDP missing");
			}
			await this.pc.setRemoteDescription({
				type: (answer.type as RTCSdpType) || "answer",
				sdp: answer.sdp,
			});

			this.hasJoined = true;
			this.bindSocketMembers();
			this.startActiveSpeakerLoop();
		} catch (err) {
			await this.cleanupMedia();
			throw err;
		} finally {
			this.joining = false;
		}
	}

	async leaveRoom(): Promise<void> {
		if (this.leaving) return;
		this.leaving = true;
		this.hasJoined = false;
		this.unbindSocketMembers();
		this.stopActiveSpeakerLoop();

		const sessionId = this.sessionId;
		try {
			if (sessionId) {
				await this.proxyJSON(
					`/api/v1/signal/cloudflare/sessions/${encodeURIComponent(sessionId)}/tracks/close`,
					"PUT",
					{
						tracks: [{ mid: "0" }],
						force: true,
					},
				).catch(() => undefined);
				await this.proxyJSON(
					`/api/v1/signal/cloudflare/sessions/${encodeURIComponent(sessionId)}`,
					"DELETE",
				).catch(() => undefined);
			}
		} finally {
			await this.cleanupMedia();
			this.leaving = false;
		}
	}

	async setMicEnabled(enabled: boolean): Promise<void> {
		this.micEnabled = enabled;
		if (this.localTrack) {
			this.localTrack.enabled = enabled;
			if (!enabled) this.onLocalSpeakingChangeCb?.(false);
			return;
		}
		if (enabled && this.hasJoined && this.pc) {
			// New local track requires full tracks/new renegotiation so remote sees audio.
			await this.publishLocalTrack();
		}
	}

	onActiveSpeakers(cb: (identities: string[]) => void): void {
		this.onActiveSpeakersCb = cb;
	}

	onLocalSpeakingChange(cb: (speaking: boolean) => void): void {
		this.onLocalSpeakingChangeCb = cb;
	}

	onRemoteAudioTrack(cb: (info: RemoteTrackInfo) => void): void {
		this.onRemoteAudioTrackCb = cb;
	}

	onRemoteAudioTrackRemoved(cb: (identity: string) => void): void {
		this.onRemoteAudioTrackRemovedCb = cb;
	}

	getExistingRemoteAudioTracks(): RemoteTrackInfo[] {
		return [...this.remoteTracks.entries()].map(([identity, track]) => ({
			identity,
			track,
		}));
	}

	subscribePeers(peers: PeerStream[]): void {
		for (const peer of peers) {
			void this.subscribePeer(peer.identity, peer.stream);
		}
	}

	unsubscribePeer(identity: string): void {
		const track = this.remoteTracks.get(identity);
		if (track) {
			track.detach();
			this.remoteTracks.delete(identity);
			this.onRemoteAudioTrackRemovedCb?.(identity);
		}
		this.peerSessions.delete(identity);
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

	isConnected(): boolean {
		return this.hasJoined && !!this.pc;
	}

	async destroy(): Promise<void> {
		await this.leaveRoom();
	}

	private bindSocketMembers(): void {
		if (!this.socket) return;
		this.socketMemberJoinedCleanup = this.socket.onServerEvent(
			"member:joined",
			(data: any) => {
				if (!this.isSameRoomEvent(data)) return;
				if (data?.identity && data?.stream) {
					void this.subscribePeer(data.identity, data.stream);
				}
			},
		);
		this.socketMemberLeftCleanup = this.socket.onServerEvent(
			"member:left",
			(data: any) => {
				if (!this.isSameRoomEvent(data)) return;
				if (data?.identity) this.unsubscribePeer(data.identity);
			},
		);
	}

	private unbindSocketMembers(): void {
		this.socketMemberJoinedCleanup?.();
		this.socketMemberLeftCleanup?.();
		this.socketMemberJoinedCleanup = undefined;
		this.socketMemberLeftCleanup = undefined;
	}

	private isSameRoomEvent(data: any): boolean {
		if (!this.room) return true;
		if (!data?.room) return true;
		const domain = typeof data.domain_uuid === "string" ? data.domain_uuid : "";
		const composite = domain ? `${domain}:${data.room}` : data.room;
		return data.room === this.room || composite === this.room;
	}

	private async subscribePeer(identity: string, remoteSessionId: string): Promise<void> {
		if (!this.hasJoined || this.leaving || !this.pc) return;
		if (!identity || !remoteSessionId) return;
		if (identity === this.identity || remoteSessionId === this.sessionId) return;
		if (this.peerSessions.get(identity) === remoteSessionId && this.remoteTracks.has(identity)) {
			return;
		}
		this.peerSessions.set(identity, remoteSessionId);

		const resp = await this.proxyJSON<TracksResponse>(
			`/api/v1/signal/cloudflare/sessions/${encodeURIComponent(this.sessionId)}/tracks/new`,
			"POST",
			{
				tracks: [
					{
						location: "remote",
						sessionId: remoteSessionId,
						trackName: LOCAL_TRACK_NAME,
					},
				],
			},
		);

		if (resp.errorCode) {
			console.warn("[cloudflare] subscribe peer failed", identity, resp.errorDescription || resp.errorCode);
			return;
		}
		const failed = resp.tracks?.find((t) => t.errorCode);
		if (failed) {
			console.warn("[cloudflare] subscribe peer track error", identity, failed.errorDescription || failed.errorCode);
			return;
		}

		for (const t of resp.tracks ?? []) {
			if (t.mid) this.midToIdentity.set(t.mid, identity);
		}

		if (resp.sessionDescription?.sdp) {
			const desc = resp.sessionDescription;
			if (desc.type === "offer" || resp.requiresImmediateRenegotiation) {
				await this.pc.setRemoteDescription({
					type: (desc.type as RTCSdpType) || "offer",
					sdp: desc.sdp,
				});
				const answer = await this.pc.createAnswer();
				await this.pc.setLocalDescription(answer);
				await this.waitIceGatheringComplete(this.pc);
				const local = this.pc.localDescription;
				if (local?.sdp) {
					await this.proxyJSON(
						`/api/v1/signal/cloudflare/sessions/${encodeURIComponent(this.sessionId)}/renegotiate`,
						"PUT",
						{
							sessionDescription: {
								type: local.type,
								sdp: local.sdp,
							},
						},
					);
				}
			} else {
				await this.pc.setRemoteDescription({
					type: (desc.type as RTCSdpType) || "answer",
					sdp: desc.sdp,
				});
			}
		}
	}

	private identityFromTrackEvent(event: RTCTrackEvent): string {
		const mid = (event as RTCTrackEvent & { transceiver?: RTCRtpTransceiver }).transceiver?.mid
			|| event.track?.id
			|| "";
		if (mid && this.midToIdentity.has(mid)) {
			return this.midToIdentity.get(mid) as string;
		}
		// Fallback: single pending peer without a track
		const pending: string[] = [];
		for (const [identity, sessionId] of this.peerSessions) {
			if (!this.remoteTracks.has(identity) && sessionId) pending.push(identity);
		}
		if (pending.length === 1) return pending[0];
		return pending[0] || event.track?.id || "remote";
	}

	/** Publish (or re-publish) local mic via Cloudflare tracks/new + setRemoteDescription. */
	private async publishLocalTrack(): Promise<void> {
		if (!this.pc || !this.sessionId) return;
		this.localTrack = await this.createLocalAudioTrack();
		const sender = this.pc.addTrack(this.localTrack);
		const transceiver = this.pc.getTransceivers().find((t) => t.sender === sender);
		const mid = transceiver?.mid || "0";

		const offer = await this.pc.createOffer();
		await this.pc.setLocalDescription(offer);
		await this.waitIceGatheringComplete(this.pc);
		const localDesc = this.pc.localDescription;
		if (!localDesc?.sdp) throw new Error("cloudflare local SDP missing");

		const tracksResp = await this.proxyJSON<TracksResponse>(
			`/api/v1/signal/cloudflare/sessions/${encodeURIComponent(this.sessionId)}/tracks/new`,
			"POST",
			{
				sessionDescription: {
					type: localDesc.type,
					sdp: localDesc.sdp,
				},
				tracks: [
					{
						location: "local",
						mid,
						trackName: LOCAL_TRACK_NAME,
					},
				],
			},
		);
		if (tracksResp.errorCode) {
			throw new Error(
				tracksResp.errorDescription || tracksResp.errorCode || "add tracks failed",
			);
		}
		const answer = tracksResp.sessionDescription;
		if (answer?.sdp) {
			await this.pc.setRemoteDescription({
				type: (answer.type as RTCSdpType) || "answer",
				sdp: answer.sdp,
			});
		}
	}

	private async createLocalAudioTrack(): Promise<MediaStreamTrack> {
		const audioConstraints: MediaTrackConstraints = {
				echoCancellation: this.options.audioCapture?.echoCancellation ?? true,
				noiseSuppression: this.options.audioCapture?.noiseSuppression ?? true,
				autoGainControl: this.options.audioCapture?.autoGainControl ?? true,
			};
		if (this.options.audioCapture?.deviceId) {
			audioConstraints.deviceId = { exact: this.options.audioCapture.deviceId };
		}
		const stream = await navigator.mediaDevices.getUserMedia({
			audio: audioConstraints,
			video: false,
		});
		const track = stream.getAudioTracks()[0];
		if (!track) throw new Error("no local audio track");
		track.enabled = this.micEnabled;
		return track;
	}

	private async cleanupMedia(): Promise<void> {
		if (this.pc && this.onPcStateChangeBound) {
			this.pc.removeEventListener(
				"iceconnectionstatechange",
				this.onPcStateChangeBound,
			);
		}
		this.onPcStateChangeBound = undefined;
		for (const track of this.remoteTracks.values()) track.detach();
		this.remoteTracks.clear();
		this.peerSessions.clear();
		this.midToIdentity.clear();
		if (this.localTrack) {
			this.localTrack.stop();
			this.localTrack = null;
		}
		if (this.pc) {
			this.pc.close();
			this.pc = null;
		}
		if (this.analyzerContext) {
			void this.analyzerContext.close().catch(() => undefined);
			this.analyzerContext = null;
		}
	}

	private startActiveSpeakerLoop(): void {
		this.stopActiveSpeakerLoop();
		this.activeSpeakerTimer = setInterval(() => {
			// Cloudflare 无 SFU 原生 active speaker：本端分析本地麦克风音量并上报信令层。
			if (!this.micEnabled) {
				this.onLocalSpeakingChangeCb?.(false);
				return;
			}
			const ctx = this.ensureAnalyzer();
			if (!ctx || !this.analyzer) {
				this.onLocalSpeakingChangeCb?.(false);
				return;
			}
			const levels = new Uint8Array(this.analyzer.frequencyBinCount);
			this.analyzer.getByteFrequencyData(levels);
			const average =
				levels.reduce((sum, value) => sum + value, 0) / levels.length;
			this.onLocalSpeakingChangeCb?.(average > 10);
		}, 200);
	}

	private ensureAnalyzer(): AudioContext | null {
		if (!this.localTrack) return null;
		if (this.analyzedTrack === this.localTrack && this.analyzerContext && this.analyzer) {
			return this.analyzerContext;
		}
		this.analyzerContext?.close().catch(() => undefined);
		const ctx = new AudioContext();
		const source = ctx.createMediaStreamSource(new MediaStream([this.localTrack]));
		const analyzer = ctx.createAnalyser();
		analyzer.fftSize = 256;
		source.connect(analyzer);
		this.analyzerContext = ctx;
		this.analyzer = analyzer;
		this.analyzedTrack = this.localTrack;
		return ctx;
	}

	private stopActiveSpeakerLoop(): void {
		if (this.activeSpeakerTimer) {
			clearInterval(this.activeSpeakerTimer);
			this.activeSpeakerTimer = null;
		}
	}

	private async waitIceGatheringComplete(pc: RTCPeerConnection): Promise<void> {
		if (pc.iceGatheringState === "complete") return;
		await new Promise<void>((resolve) => {
			const check = () => {
				if (pc.iceGatheringState === "complete") {
					pc.removeEventListener("icegatheringstatechange", check);
					resolve();
				}
			};
			pc.addEventListener("icegatheringstatechange", check);
			setTimeout(() => {
				pc.removeEventListener("icegatheringstatechange", check);
				resolve();
			}, 2000);
		});
	}

	private async proxyJSON<T = unknown>(
		path: string,
		method: string,
		body?: unknown,
	): Promise<T> {
		const headers: Record<string, string> = {
			"Content-Type": "application/json",
		};
		const token = readAccessToken();
		if (token) headers.Authorization = `Bearer ${token}`;

		const resp = await fetch(path, {
			method,
			headers,
			body: body === undefined ? undefined : JSON.stringify(body),
		});
		const json = (await resp.json().catch(() => null)) as APIResult<T> | null;
		if (!resp.ok) {
			throw new Error(
				json?.msg || `cloudflare proxy failed: HTTP ${resp.status}`,
			);
		}
		if (json && typeof json.code === "number" && json.code !== 0) {
			throw new Error(json.msg || `cloudflare proxy error code=${json.code}`);
		}
		return (json?.data ?? (null as T)) as T;
	}
}
