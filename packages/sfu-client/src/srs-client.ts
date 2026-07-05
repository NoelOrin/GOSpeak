import type {
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUClient,
	SFUClientOptions,
} from "./types";

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
	private subscribePc: RTCPeerConnection | null = null;
	private localStream: MediaStream | null = null;
	private remoteTracks = new Map<string, SRSRemoteAudioTrack>();
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
	private subscribeResourceUrl = "";
	private identity = "";

	constructor(private readonly options: SFUClientOptions = {}) {}

	async joinRoom(token: string, url: string, identity: string): Promise<void> {
		this.identity = identity;
		this.localStream = await navigator.mediaDevices.getUserMedia({
			audio: {
				echoCancellation: this.options.audioCapture?.echoCancellation ?? true,
				noiseSuppression: this.options.audioCapture?.noiseSuppression ?? true,
				autoGainControl: this.options.audioCapture?.autoGainControl ?? true,
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
			await this.exchangeSdp(this.publishPc, url, token, true);
			// 等 publish PC ICE 连通后再 WHEP：SRS 源未激活时 WHEP 撞 "no play relations"(5018)，关连接不回 HTTP
			await this.waitForPublishIceConnected();

			this.subscribePc = new RTCPeerConnection();
			this.subscribePc.addTransceiver("audio", { direction: "recvonly" });
			this.subscribePc.ontrack = (event) => {
				const stream = event.streams[0];
				if (!stream) return;
				const key = event.track.id || `remote-${this.remoteTracks.size}`;
				const remoteTrack = new SRSRemoteAudioTrack(stream);
				this.remoteTracks.set(key, remoteTrack);
				this.onRemoteAudioTrackCb?.({ identity: key, track: remoteTrack });
				event.track.addEventListener("ended", () => {
					remoteTrack.detach();
					this.remoteTracks.delete(key);
					this.onRemoteAudioTrackRemovedCb?.(key);
				});
			};
			await this.exchangeSdp(this.subscribePc, url.replace(/\/whip\/?$/, "/whep/"), token, false);
		} catch (err) {
			// 失败必须清理已建 publish session，否则 SRS 流孤儿，retry 撞 RtcStreamBusy(5020)
			await this.cleanupPartialJoin();
			throw err;
		}
		this.startAudioLevelLoop();
		this.hasJoined = true;
		this.onPcStateChangeBound = () => {
			if (!this.hasJoined) return;
			const pub = this.publishPc?.iceConnectionState;
			const sub = this.subscribePc?.iceConnectionState;
			// 任一 PC failed → 终态断连（WHIP/WHEP 无 restartIce 语义，重新 exchange 风险高）
			if (pub === "failed" || sub === "failed") {
				this.hasJoined = false;
				this.isReconnecting = false;
				this.onDisconnectedCb?.();
				return;
			}
			// 任一 PC disconnected → reconnecting（ICE 多数自恢复，不主动 restartIce 避免竞态）
			const anyDown = pub === "disconnected" || sub === "disconnected";
			const bothUp = (pub === "connected" || pub === "completed") && (sub === "connected" || sub === "completed");
			if (anyDown && !this.isReconnecting) {
				this.isReconnecting = true;
				this.onReconnectingCb?.();
			} else if (bothUp && this.isReconnecting) {
				this.isReconnecting = false;
				this.onReconnectedCb?.();
			}
		};
		this.publishPc?.addEventListener("iceconnectionstatechange", this.onPcStateChangeBound);
		this.subscribePc?.addEventListener("iceconnectionstatechange", this.onPcStateChangeBound);
	}

	private async waitForPublishIceConnected(timeoutMs = 10_000): Promise<void> {
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
		this.subscribeResourceUrl = "";
		this.publishPc?.close();
		this.subscribePc?.close();
		this.publishPc = null;
		this.subscribePc = null;
		if (this.localStream) {
			this.localStream.getTracks().forEach((track) => track.stop());
			this.localStream = null;
		}
	}

	async leaveRoom(): Promise<void> {
		this.hasJoined = false;
		this.isReconnecting = false;
		if (this.onPcStateChangeBound) {
			this.publishPc?.removeEventListener("iceconnectionstatechange", this.onPcStateChangeBound);
			this.subscribePc?.removeEventListener("iceconnectionstatechange", this.onPcStateChangeBound);
			this.onPcStateChangeBound = undefined;
		}
		if (this.activeSpeakerRaf !== null) {
			cancelAnimationFrame(this.activeSpeakerRaf);
			this.activeSpeakerRaf = null;
		}
		await this.deleteResource(this.publishResourceUrl);
		await this.deleteResource(this.subscribeResourceUrl);
		this.publishResourceUrl = "";
		this.subscribeResourceUrl = "";
		this.publishPc?.close();
		this.subscribePc?.close();
		this.publishPc = null;
		this.subscribePc = null;
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
	}

	async setMicEnabled(enabled: boolean): Promise<void> {
		this.localStream?.getAudioTracks().forEach((track) => {
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

	destroy(): void {
		void this.leaveRoom();
	}

	private async exchangeSdp(pc: RTCPeerConnection, endpoint: string, token: string, publishing: boolean): Promise<void> {
		const offer = await pc.createOffer();
		await pc.setLocalDescription(offer);
		const resp = await fetch(endpoint, {
			method: "POST",
			headers: {
				Authorization: `Bearer ${token}`,
				"Content-Type": "application/sdp",
			},
			body: offer.sdp || "",
		});
		if (!resp.ok) {
			throw new Error(`SRS ${publishing ? "WHIP" : "WHEP"} request failed: ${resp.status}`);
		}
		const location = resp.headers.get("Location") || "";
		if (publishing) {
			this.publishResourceUrl = location;
		} else {
			this.subscribeResourceUrl = location;
		}
		const answer = await resp.text();
		await pc.setRemoteDescription({ type: "answer", sdp: answer });
	}

	private startAudioLevelLoop(): void {
		if (!this.localStream || !this.onActiveSpeakersCb) return;
		this.analyzerContext = new AudioContext();
		const source = this.analyzerContext.createMediaStreamSource(this.localStream);
		const analyser = this.analyzerContext.createAnalyser();
		source.connect(analyser);
		analyser.fftSize = 256;
		const levels = new Uint8Array(analyser.frequencyBinCount);
		const tick = () => {
			analyser.getByteFrequencyData(levels);
			const average = levels.reduce((sum, value) => sum + value, 0) / levels.length;
			this.onActiveSpeakersCb?.(average > 10 ? [this.identity] : []);
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
