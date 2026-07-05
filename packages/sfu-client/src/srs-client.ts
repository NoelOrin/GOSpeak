import type {
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUClient,
	SFUClientOptions,
} from "./types";

interface PeerSub {
	identity: string;
	stream: string;
	pc: RTCPeerConnection | null;
	resourceUrl: string;
	retryCount: number;
	retryTimer: ReturnType<typeof setTimeout> | null;
	connecting: boolean;
}

function appendStream(
	url: string,
	stream: string | undefined,
	token: string | undefined,
	withToken: boolean,
): string {
	if (!stream) return url;
	const sep = url.includes("?") ? "&" : "?";
	let q = `app=live&stream=${encodeURIComponent(stream)}`;
	if (withToken && token) q += `&token=${encodeURIComponent(token)}`;
	return url + sep + q;
}

class SRSRemoteAudioTrack implements RemoteAudioTrackLike {
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

export class SRSSFUClient implements SFUClient {
	private publishPc: RTCPeerConnection | null = null;
	private localStream: MediaStream | null = null;
	private remoteTracks = new Map<string, SRSRemoteAudioTrack>();
	private peerSubs = new Map<string, PeerSub>();
	private onActiveSpeakersCb?: (ids: string[]) => void;
	private onRemoteAudioTrackCb?: (info: RemoteTrackInfo) => void;
	private onRemoteAudioTrackRemovedCb?: (identity: string) => void;
	private onDisconnectedCb?: () => void;
	private onReconnectingCb?: () => void;
	private onReconnectedCb?: () => void;
	private hasJoined = false;
	private isReconnecting = false;
	private onPcStateChangeBound?: () => void;
	private activeSpeakerRaf: number | null = null;
	private analyzerContext: AudioContext | null = null;
	private publishResourceUrl = "";
	private identity = "";
	private ownStream = "";
	private streamToken = "";
	private baseWhepUrl = "";
	private socket?: any;
	private leaving = false;
	private socketMemberJoinedBound?: (...args: unknown[]) => void;
	private socketMemberLeftBound?: (...args: unknown[]) => void;

	constructor(private readonly options: SFUClientOptions = {}) {
		this.socket = options.socket;
	}

	async joinRoom(
		token: string,
		url: string,
		identity: string,
		room?: string,
		stream?: string,
		streamToken?: string,
	): Promise<void> {
		this.identity = identity;
		this.ownStream = stream || "";
		this.streamToken = streamToken || "";
		this.baseWhepUrl = url.replace(/\/whip\/?$/, "/whep/");

		this.localStream = await navigator.mediaDevices.getUserMedia({
			audio: {
				echoCancellation: this.options.audioCapture?.echoCancellation ?? true,
				noiseSuppression:
					this.options.audioCapture?.noiseSuppression ?? true,
				autoGainControl:
					this.options.audioCapture?.autoGainControl ?? true,
				sampleRate: this.options.audioCapture?.sampleRate,
				sampleSize: this.options.audioCapture?.sampleSize,
				channelCount: this.options.audioCapture?.channelCount,
			},
			video: false,
		});

		this.publishPc = new RTCPeerConnection();
		for (const track of this.localStream.getAudioTracks()) {
			this.publishPc.addTrack(track, this.localStream);
		}
		try {
			const whipUrl = appendStream(url, stream, streamToken, true);
			this.publishResourceUrl = await this.exchangeSdp(
				this.publishPc,
				whipUrl,
				token,
				true,
			);
			await this.waitForPublishIceConnected();
		} catch (err) {
			await this.cleanupPartialJoin();
			throw err;
		}

		this.startAudioLevelLoop();
		this.hasJoined = true;

		if (this.socket) {
			this.socketMemberJoinedBound = (data: any) => {
				if (data.identity && data.stream) {
					this.subscribePeer(data.identity, data.stream);
				}
			};
			this.socketMemberLeftBound = (data: any) => {
				if (data.identity) {
					this.unsubscribePeer(data.identity);
				}
			};
			this.socket.on("member:joined", this.socketMemberJoinedBound);
			this.socket.on("member:left", this.socketMemberLeftBound);
		}

		this.onPcStateChangeBound = () => {
			if (!this.hasJoined) return;
			const pub = this.publishPc?.iceConnectionState;
			if (pub === "failed") {
				this.hasJoined = false;
				this.isReconnecting = false;
				this.onDisconnectedCb?.();
				return;
			}
			const anyDown = pub === "disconnected";
			const bothUp =
				pub === "connected" || pub === "completed";
			if (anyDown && !this.isReconnecting) {
				this.isReconnecting = true;
				this.onReconnectingCb?.();
			} else if (bothUp && this.isReconnecting) {
				this.isReconnecting = false;
				this.onReconnectedCb?.();
			}
		};
		this.publishPc?.addEventListener(
			"iceconnectionstatechange",
			this.onPcStateChangeBound,
		);
	}

	private subscribePeer(identity: string, stream: string): void {
		if (!this.hasJoined || this.leaving) return;
		if (identity === this.identity || stream === this.ownStream) return;

		const existing = this.peerSubs.get(identity);
		// Already actively subscribed (has a live PC) or connecting
		if (existing && (existing.pc !== null || existing.connecting)) return;

		// Clear pending retry timer
		if (existing?.retryTimer) {
			clearTimeout(existing.retryTimer);
		}

		// Create entry immediately so concurrent calls skip and retry timer knows
		const sub: PeerSub = {
			identity,
			stream,
			pc: null,
			resourceUrl: "",
			retryCount: existing?.retryCount ?? 0,
			retryTimer: null,
			connecting: true,
		};
		this.peerSubs.set(identity, sub);

		const pc = new RTCPeerConnection();
		pc.addTransceiver("audio", { direction: "recvonly" });

		pc.ontrack = (event: any) => {
			const ms = event.streams?.[0];
			if (!ms) return;
			const remoteTrack = new SRSRemoteAudioTrack(ms);
			this.remoteTracks.set(identity, remoteTrack);
			this.onRemoteAudioTrackCb?.({ identity, track: remoteTrack });
			event.track.addEventListener("ended", () => {
				remoteTrack.detach();
				this.remoteTracks.delete(identity);
				const s = this.peerSubs.get(identity);
				if (!s || this.leaving || !this.hasJoined) return;
				if (s.retryTimer) clearTimeout(s.retryTimer);
				if (s.pc) {
					s.pc.close();
				}
				this.deleteResource(s.resourceUrl);
				this.peerSubs.delete(identity);
				this.scheduleRetry(identity, stream);
			});
		};

		pc.addEventListener("iceconnectionstatechange", () => {
			if (pc.iceConnectionState !== "failed") return;
			const s = this.peerSubs.get(identity);
			if (!s || s.pc !== pc) return;

			this.deleteResource(s.resourceUrl);
			pc.close();
			this.peerSubs.delete(identity);
			if (this.remoteTracks.has(identity)) {
				const t = this.remoteTracks.get(identity)!;
				t.detach();
				this.remoteTracks.delete(identity);
			}

			this.scheduleRetry(identity, stream);
		});

		const whepUrl = appendStream(this.baseWhepUrl, stream, "", false);

		this.exchangeSdp(pc, whepUrl, "", false)
			.then((resourceUrl) => {
				const s = this.peerSubs.get(identity);
				// If the entry was removed (unsubscribed) during exchange, discard
				if (!s) {
					pc.close();
					this.deleteResource(resourceUrl);
					return;
				}
				s.pc = pc;
				s.connecting = false;
				s.resourceUrl = resourceUrl;
				s.retryCount = 0;
			})
			.catch(() => {
				const s = this.peerSubs.get(identity);
				if (!s) { pc.close(); return; } // cleanup already happened (unsubscribePeer)
				if (s.pc !== null) { pc.close(); return; } // a successful sub completed; this catch is stale
				const prevRetryCount = s.retryCount;
				if (s.retryTimer) clearTimeout(s.retryTimer);
				this.peerSubs.delete(identity);
				this.scheduleRetry(identity, stream, prevRetryCount);
			});
	}

	subscribePeers(
		peers: { identity: string; stream: string }[],
	): void {
		for (const p of peers) {
			this.subscribePeer(p.identity, p.stream);
		}
	}

	unsubscribePeer(identity: string): void {
		const sub = this.peerSubs.get(identity);
		if (!sub) return;

		if (sub.retryTimer) {
			clearTimeout(sub.retryTimer);
		}
		if (sub.pc) {
			this.deleteResource(sub.resourceUrl);
			sub.pc.close();
		}
		if (this.remoteTracks.has(identity)) {
			const t = this.remoteTracks.get(identity)!;
			t.detach();
			this.remoteTracks.delete(identity);
		}

		this.peerSubs.delete(identity);

		if (!this.leaving) {
			this.onRemoteAudioTrackRemovedCb?.(identity);
		}
	}

	private scheduleRetry(
		identity: string,
		stream: string,
		prevRetryCount?: number,
	): void {
		let sub = this.peerSubs.get(identity);
		if (!sub) {
			sub = {
				identity,
				stream,
				pc: null,
				resourceUrl: "",
				retryCount: prevRetryCount ?? 0,
				retryTimer: null,
				connecting: false,
			};
			this.peerSubs.set(identity, sub);
		}

		if (sub.retryCount >= 5) {
			if (sub.pc) {
				this.deleteResource(sub.resourceUrl);
				sub.pc.close();
			}
			if (this.remoteTracks.has(identity)) {
				const t = this.remoteTracks.get(identity)!;
				t.detach();
				this.remoteTracks.delete(identity);
			}
			this.peerSubs.delete(identity);
			this.onRemoteAudioTrackRemovedCb?.(identity);
			return;
		}

		const delay = Math.pow(2, sub.retryCount) * 1000;

		sub.retryTimer = setTimeout(() => {
			this.subscribePeer(identity, stream);
		}, delay);
		sub.retryCount++;
	}

	async leaveRoom(): Promise<void> {
		// Remove socket listeners BEFORE setting leaving flag
		if (this.socket) {
			if (this.socketMemberJoinedBound) {
				this.socket.off(
					"member:joined",
					this.socketMemberJoinedBound,
				);
			}
			if (this.socketMemberLeftBound) {
				this.socket.off("member:left", this.socketMemberLeftBound);
			}
		}
		this.socketMemberJoinedBound = undefined;
		this.socketMemberLeftBound = undefined;

		this.leaving = true;
		this.hasJoined = false;
		this.isReconnecting = false;

		// Unsubscribe all peers (callbacks suppressed via leaving flag)
		for (const [identity] of this.peerSubs) {
			this.unsubscribePeer(identity);
		}

		if (this.onPcStateChangeBound) {
			this.publishPc?.removeEventListener(
				"iceconnectionstatechange",
				this.onPcStateChangeBound,
			);
			this.onPcStateChangeBound = undefined;
		}
		if (this.activeSpeakerRaf !== null) {
			cancelAnimationFrame(this.activeSpeakerRaf);
			this.activeSpeakerRaf = null;
		}
		await this.deleteResource(this.publishResourceUrl);
		this.publishResourceUrl = "";
		this.publishPc?.close();
		this.publishPc = null;
		this.remoteTracks.forEach((track) => track.detach());
		this.remoteTracks.clear();
		if (this.localStream) {
			this.localStream.getTracks().forEach((track) => track.stop());
			this.localStream = null;
		}
		this.onReconnectingCb = undefined;
		this.onReconnectedCb = undefined;
		await this.analyzerContext?.close();
		this.analyzerContext = null;
		this.leaving = false;
	}

	async setMicEnabled(enabled: boolean): Promise<void> {
		this.localStream
			?.getAudioTracks()
			.forEach((track) => {
				track.enabled = enabled;
			});
	}

	onActiveSpeakers(cb: (identities: string[]) => void): void {
		this.onActiveSpeakersCb = cb;
	}

	onRemoteAudioTrack(cb: (info: RemoteTrackInfo) => void): void {
		this.onRemoteAudioTrackCb = cb;
	}

	onRemoteAudioTrackRemoved(cb: (identity: string) => void): void {
		this.onRemoteAudioTrackRemovedCb = cb;
	}

	getExistingRemoteAudioTracks(): RemoteTrackInfo[] {
		return Array.from(this.remoteTracks.entries()).map(
			([identity, track]) => ({
				identity,
				track,
			}),
		);
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

	destroy(): void {
		void this.leaveRoom();
	}

	private async exchangeSdp(
		pc: RTCPeerConnection,
		endpoint: string,
		token: string,
		publishing: boolean,
	): Promise<string> {
		const offer = await pc.createOffer();
		await pc.setLocalDescription(offer);
		const headers: Record<string, string> = {
			"Content-Type": "application/sdp",
		};
		if (token) {
			headers["Authorization"] = `Bearer ${token}`;
		}
		const controller = new AbortController();
		const timer = setTimeout(() => controller.abort(), 15_000);
		try {
			const resp = await fetch(endpoint, {
				method: "POST",
				headers,
				body: offer.sdp || "",
				signal: controller.signal,
			});
			if (!resp.ok) {
				throw new Error(
					`SRS ${publishing ? "WHIP" : "WHEP"} request failed: ${resp.status}`,
				);
			}
			const location = resp.headers.get("Location") || "";
			const answer = await resp.text();
			await pc.setRemoteDescription({
				type: "answer",
				sdp: answer,
			});
			return location;
		} finally {
			clearTimeout(timer);
		}
	}

	private async waitForPublishIceConnected(
		timeoutMs = 10_000,
	): Promise<void> {
		const pc = this.publishPc;
		if (!pc) return;
		const state = pc.iceConnectionState;
		if (state === "connected" || state === "completed") return;
		if (state === "failed" || state === "closed") {
			throw new Error(`SRS publish ICE ${state}`);
		}
		await new Promise<void>((resolve, reject) => {
			const onState = () => {
				const s = pc.iceConnectionState;
				if (s === "connected" || s === "completed") {
					cleanup();
					resolve();
				} else if (s === "failed" || s === "closed") {
					cleanup();
					reject(new Error(`SRS publish ICE ${s}`));
				}
			};
			const timer = setTimeout(() => {
				cleanup();
				reject(new Error("SRS publish ICE connect timeout"));
			}, timeoutMs);
			const cleanup = () => {
				clearTimeout(timer);
				pc.removeEventListener("iceconnectionstatechange", onState);
			};
			pc.addEventListener("iceconnectionstatechange", onState);
		});
	}

	private async cleanupPartialJoin(): Promise<void> {
		await this.deleteResource(this.publishResourceUrl);
		this.publishResourceUrl = "";
		this.publishPc?.close();
		this.publishPc = null;
		if (this.localStream) {
			this.localStream.getTracks().forEach((track) => track.stop());
			this.localStream = null;
		}
	}

	private startAudioLevelLoop(): void {
		if (!this.localStream || !this.onActiveSpeakersCb) return;
		this.analyzerContext = new AudioContext();
		const source = this.analyzerContext.createMediaStreamSource(
			this.localStream,
		);
		const analyser = this.analyzerContext.createAnalyser();
		source.connect(analyser);
		analyser.fftSize = 256;
		const levels = new Uint8Array(analyser.frequencyBinCount);
		const tick = () => {
			analyser.getByteFrequencyData(levels);
			const average =
				levels.reduce((sum, value) => sum + value, 0) /
				levels.length;
			this.onActiveSpeakersCb?.(
				average > 10 ? [this.identity] : [],
			);
			this.activeSpeakerRaf = requestAnimationFrame(tick);
		};
		tick();
	}

	private async deleteResource(url: string): Promise<void> {
		if (!url) return;
		try {
			await fetch(url, { method: "DELETE" });
		} catch {
			return;
		}
	}
}
