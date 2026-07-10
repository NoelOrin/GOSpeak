import type {
	JoinParams,
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUClient,
	SFUClientOptions,
} from "./types";

// 同 stream 全局闸门（不改 useVoiceSession）：
// orchestrator abort 会并发“旧 leave + 新 join”，若双 WHIP 同时 POST 同 stream → 5020/500。
// 规则：
// 1) 同一 stream 同一时刻最多一个 holder
// 2) leave 立即 abort in-flight WHIP
// 3) 新 join 必须等旧 holder 释放后再 POST
type StreamGate = {
	holder: SRSSFUClient | null;
	free: Promise<void>;
	resolveFree: (() => void) | null;
	tail: Promise<unknown>;
};

const streamGates = new Map<string, StreamGate>();

function getStreamGate(streamKey: string): StreamGate {
	const key = streamKey || "__empty__";
	let gate = streamGates.get(key);
	if (!gate) {
		gate = {
			holder: null,
			free: Promise.resolve(),
			resolveFree: null,
			tail: Promise.resolve(),
		};
		streamGates.set(key, gate);
	}
	return gate;
}

function enqueueStreamOp<T>(streamKey: string, op: () => Promise<T>): Promise<T> {
	const gate = getStreamGate(streamKey);
	const run = gate.tail.catch(() => undefined).then(op);
	gate.tail = run.then(
		() => undefined,
		() => undefined,
	);
	return run;
}

async function acquireStream(
	streamKey: string,
	self: SRSSFUClient,
	isAborted: () => boolean,
): Promise<void> {
	const gate = getStreamGate(streamKey);
	for (;;) {
		if (isAborted()) throw new Error("SRS join aborted");
		if (!gate.holder || gate.holder === self) {
			gate.holder = self;
			if (!gate.resolveFree) {
				gate.free = new Promise<void>((resolve) => {
					gate.resolveFree = resolve;
				});
			}
			return;
		}
		await gate.free;
	}
}

function releaseStream(streamKey: string, self: SRSSFUClient): void {
	const gate = getStreamGate(streamKey);
	if (gate.holder !== self) return;
	gate.holder = null;
	const resolve = gate.resolveFree;
	gate.resolveFree = null;
	resolve?.();
	// free promise 重置为已完成，后续 wait 不挂
	if (!gate.resolveFree) {
		gate.free = Promise.resolve();
	}
}

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
	private static MAX_SUBSCRIBE_RETRIES = 60;
	private analyzerContext: AudioContext | null = null;
	private publishResourceUrl = "";
	private identity = "";
	private room = "";
	private ownStream = "";
	private streamToken = "";
	private roomToken = "";
	private whipUrl = "";
	private baseWhepUrl = "";
	private socket?: any;
	private leaving = false;
	private leavePromise: Promise<void> | null = null;
	// 递增代数：abort/leave 与 in-flight WHIP 竞态时，旧 publish 不得回写状态。
	private joinGeneration = 0;
	private mediaAbort: AbortController | null = null;
	private activeStreamKey = "";
	private micEnabled = true;
	private socketMemberJoinedBound?: (...args: unknown[]) => void;
	private socketMemberLeftBound?: (...args: unknown[]) => void;

	constructor(private readonly options: SFUClientOptions = {}) {
		this.socket = options.socket;
	}

	async joinRoom(params: JoinParams): Promise<void> {
		const streamKey = params.stream || `${params.room || ""}:${params.identity || ""}`;
		await enqueueStreamOp(streamKey, () => this.joinRoomLocked(params, streamKey));
	}

	private async joinRoomLocked(
		params: JoinParams,
		streamKey: string,
	): Promise<void> {
		// 同 client 复用前先拆旧会话，避免 WHIP stream busy / 双 publish。
		if (this.hasJoined || this.publishPc || this.publishResourceUrl) {
			await this.leaveRoomLocked();
		}
		// 等旧 holder（可能是被 abort 的另一实例）释放 stream 后再 WHIP。
		await acquireStream(streamKey, this, () => this.leaving);
		this.activeStreamKey = streamKey;

		const joinGen = ++this.joinGeneration;
		this.leaving = false;
		const { token, serverUrl: url, identity, room, stream, streamToken } = params;
		this.identity = identity;
		this.room = room || "";
		this.ownStream = stream || streamKey;
		this.streamToken = streamToken || "";
		this.roomToken = token || "";
		this.whipUrl = url;
		this.baseWhepUrl = url.replace(/\/whip\/?$/, "/whep/");
		this.micEnabled = true;

		try {
			await this.startPublish(joinGen);
			if (this.leaving || joinGen !== this.joinGeneration) {
				await this.cleanupPartialJoin();
				releaseStream(streamKey, this);
				if (this.activeStreamKey === streamKey) this.activeStreamKey = "";
				throw new Error("SRS join aborted");
			}
		} catch (err) {
			await this.cleanupPartialJoin();
			releaseStream(streamKey, this);
			if (this.activeStreamKey === streamKey) this.activeStreamKey = "";
			throw err;
		}

		this.hasJoined = true;

		if (this.socket) {
			this.socketMemberJoinedBound = (data: any) => {
				if (!this.isSameRoomEvent(data)) return;
				if (data.identity && data.stream) {
					this.subscribePeer(data.identity, data.stream);
				}
			};
			this.socketMemberLeftBound = (data: any) => {
				if (!this.isSameRoomEvent(data)) return;
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

	private isSameRoomEvent(data: any): boolean {
		if (!this.room) return true;
		if (!data?.room) return true;
		return data.room === this.room;
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

		const whepUrl = appendStream(
			this.baseWhepUrl,
			stream,
			this.roomToken,
			true,
		);

		this.exchangeSdp(pc, whepUrl, this.roomToken, false)
			.then((resourceUrl) => {
				const s = this.peerSubs.get(identity);
				// Stale 守卫：交换期间条目若被移除(unsubscribePeer)或被新订阅替换
				// (retry 后 member:joined 再订阅)，则丢弃本次成功，避免覆盖新 sub 的 pc。
				// 比较 sub 引用而非 s.pc：连接中 s.pc 为 null，仅靠 s.pc!==null 无法捕获
				// "新订阅尚在 connecting" 的 stale 成功(会把新 sub.pc 误置为旧 pc)。
				if (s !== sub) {
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
				// Stale 守卫：新订阅已接管(s!==sub，含已成功与尚在 connecting)或条目已移除，
				// 仅关闭失败的旧 pc，不删条目不重试，避免孤立新 pc 与无谓断连重连。
				if (s !== sub) {
					pc.close();
					return;
				}
				pc.close();
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

		if (sub.retryCount >= SRSSFUClient.MAX_SUBSCRIBE_RETRIES) {
			this.peerSubs.delete(identity);
			if (this.remoteTracks.has(identity)) {
				const t = this.remoteTracks.get(identity)!;
				t.detach();
				this.remoteTracks.delete(identity);
			}
			this.onRemoteAudioTrackRemovedCb?.(identity);
			return;
		}

		const delay = Math.min(2 ** Math.min(sub.retryCount, 3), 8) * 1000;

		sub.retryTimer = setTimeout(() => {
			this.subscribePeer(identity, stream);
		}, delay);
		sub.retryCount++;
	}

	async leaveRoom(): Promise<void> {
		// 立即打断 in-flight join（不入队），避免 leave 排在 join 后死等 WHIP。
		this.beginLeaveAbort();
		const streamKey =
			this.activeStreamKey ||
			this.ownStream ||
			`${this.room || ""}:${this.identity || ""}` ||
			"__empty__";
		await enqueueStreamOp(streamKey, () => this.leaveRoomLocked());
	}

	private beginLeaveAbort(): void {
		this.leaving = true;
		this.joinGeneration++;
		this.hasJoined = false;
		this.isReconnecting = false;
		this.mediaAbort?.abort();
		this.mediaAbort = null;
	}

	private async leaveRoomLocked(): Promise<void> {
		if (this.leavePromise) {
			await this.leavePromise;
			return;
		}

		this.leavePromise = this.doLeaveRoom();
		try {
			await this.leavePromise;
		} finally {
			this.leavePromise = null;
		}
	}

	private async doLeaveRoom(): Promise<void> {
		// leaveRoom 已 beginLeaveAbort；这里只做资源拆卸。
		this.leaving = true;
		this.hasJoined = false;
		this.isReconnecting = false;

		// Remove socket listeners BEFORE tearing media
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

		this.room = "";
		this.roomToken = "";
		this.streamToken = "";
		this.ownStream = "";

		// Unsubscribe all peers (callbacks suppressed via leaving flag)
		for (const [identity] of this.peerSubs) {
			this.unsubscribePeer(identity);
		}

		await this.stopPublish({ stopLocalTracks: true });
		this.onPcStateChangeBound = undefined;
		this.remoteTracks.forEach((track) => track.detach());
		this.remoteTracks.clear();
		this.whipUrl = "";
		this.baseWhepUrl = "";
		this.micEnabled = true;
		this.onReconnectingCb = undefined;
		this.onReconnectedCb = undefined;
		this.onDisconnectedCb = undefined;
		const streamKey = this.activeStreamKey || this.ownStream;
		if (streamKey) {
			releaseStream(streamKey, this);
		}
		this.activeStreamKey = "";
		this.leaving = false;
	}

	async setMicEnabled(enabled: boolean): Promise<void> {
		if (!this.hasJoined || this.leaving) return;
		if (!enabled) {
			this.localStream?.getAudioTracks().forEach((track) => {
				track.enabled = false;
				track.stop();
			});
			this.micEnabled = false;
			if (this.activeSpeakerRaf !== null) {
				cancelAnimationFrame(this.activeSpeakerRaf);
				this.activeSpeakerRaf = null;
			}
			this.onActiveSpeakersCb?.([]);
			return;
		}

		const live = this.localStream
			?.getAudioTracks()
			.some((t) => t.readyState === "live");
		if (live) {
			this.localStream?.getAudioTracks().forEach((track) => {
				track.enabled = true;
			});
			this.micEnabled = true;
			this.startAudioLevelLoop();
			return;
		}

		const media = await navigator.mediaDevices.getUserMedia({
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
		const newTrack = media.getAudioTracks()[0];
		if (!newTrack) {
			media.getTracks().forEach((t) => t.stop());
			return;
		}
		newTrack.enabled = true;

		// 若 PC 断开/失败/关闭，重建 WHIP。
		const pcState = this.publishPc?.connectionState ?? "new";
		if (pcState === "failed" || pcState === "disconnected" || pcState === "closed") {
			this.localStream = media;
			this.micEnabled = true;
			await this.startPublish();
			this.startAudioLevelLoop();
			return;
		}

		const sender = this.publishPc
			?.getSenders()
			.find((s) => !s.track || s.track.kind === "audio");
		if (sender) {
			await sender.replaceTrack(newTrack);
		} else if (this.publishPc) {
			this.publishPc.addTrack(newTrack, media);
		}

		this.localStream?.getTracks().forEach((t) => {
			if (t !== newTrack) t.stop();
		});
		this.localStream = media;
		this.micEnabled = true;
		this.startAudioLevelLoop();
	}

	private async startPublish(joinGen = this.joinGeneration): Promise<void> {
		if (this.publishPc || this.publishResourceUrl) {
			await this.stopPublish({ stopLocalTracks: true });
		}
		if (this.leaving || joinGen !== this.joinGeneration) {
			throw new Error("SRS join aborted");
		}

		const localStream = await navigator.mediaDevices.getUserMedia({
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
		if (this.leaving || joinGen !== this.joinGeneration) {
			localStream.getTracks().forEach((track) => track.stop());
			throw new Error("SRS join aborted");
		}

		this.localStream = localStream;
		const publishPc = new RTCPeerConnection();
		this.publishPc = publishPc;
		for (const track of this.localStream.getAudioTracks()) {
			track.enabled = true;
			publishPc.addTrack(track, this.localStream);
		}

		const whipUrl = appendStream(
			this.whipUrl,
			this.ownStream,
			this.streamToken,
			true,
		);
		const mediaAbort = new AbortController();
		this.mediaAbort = mediaAbort;
		let resourceUrl = "";
		try {
			resourceUrl = await this.exchangeSdp(
				publishPc,
				whipUrl,
				this.roomToken,
				true,
				mediaAbort.signal,
			);
			if (this.leaving || joinGen !== this.joinGeneration) {
				await this.deleteResource(resourceUrl);
				publishPc.close();
				if (this.publishPc === publishPc) {
					this.publishPc = null;
					this.publishResourceUrl = "";
				}
				this.localStream?.getTracks().forEach((track) => track.stop());
				if (this.localStream === localStream) {
					this.localStream = null;
				}
				throw new Error("SRS join aborted");
			}
			this.publishResourceUrl = resourceUrl;
			// WHIP 201 后不等 ICE。SRS ice-lite + HTTP 响应即 publish 成功。
			if (this.leaving || joinGen !== this.joinGeneration) {
				await this.stopPublish({ stopLocalTracks: true });
				throw new Error("SRS join aborted");
			}
		} catch (err) {
			const errResource =
				resourceUrl ||
				(typeof err === "object" && err && "resourceUrl" in err
					? String((err as { resourceUrl?: string }).resourceUrl || "")
					: "");
			if (this.publishPc === publishPc) {
				if (errResource && !this.publishResourceUrl) {
					this.publishResourceUrl = errResource;
				}
				await this.stopPublish({ stopLocalTracks: true });
			} else {
				await this.deleteResource(errResource);
				publishPc.close();
				localStream.getTracks().forEach((track) => track.stop());
			}
			throw err;
		} finally {
			if (this.mediaAbort === mediaAbort) {
				this.mediaAbort = null;
			}
		}

		if (this.onPcStateChangeBound) {
			publishPc.addEventListener(
				"iceconnectionstatechange",
				this.onPcStateChangeBound,
			);
		}
		this.startAudioLevelLoop();
		// WHIP 201 后不等 ICE。ICE 若最终 fail 由 iceconnectionstatechange listener 处理。
	}

	private async stopPublish(opts?: {
		stopLocalTracks?: boolean;
	}): Promise<void> {
		if (this.onPcStateChangeBound) {
			this.publishPc?.removeEventListener(
				"iceconnectionstatechange",
				this.onPcStateChangeBound,
			);
		}
		if (this.activeSpeakerRaf !== null) {
			cancelAnimationFrame(this.activeSpeakerRaf);
			this.activeSpeakerRaf = null;
		}
		await this.analyzerContext?.close().catch(() => {});
		this.analyzerContext = null;

		// 先关 PC / 停轨，再 best-effort DELETE。DELETE 慢不能拖住切房。
		const resourceUrl = this.publishResourceUrl;
		this.publishResourceUrl = "";
		const pc = this.publishPc;
		this.publishPc = null;
		pc?.close();

		if (opts?.stopLocalTracks !== false && this.localStream) {
			this.localStream.getTracks().forEach((track) => track.stop());
			this.localStream = null;
		}

		await this.deleteResource(resourceUrl);
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

	isConnected(): boolean {
		return this.hasJoined && !this.leaving;
	}

	async destroy(): Promise<void> {
		await this.leaveRoom();
	}

	private async exchangeSdp(
		pc: RTCPeerConnection,
		endpoint: string,
		token: string,
		publishing: boolean,
		externalSignal?: AbortSignal,
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
		const onExternalAbort = () => controller.abort();
		if (externalSignal) {
			if (externalSignal.aborted) {
				controller.abort();
			} else {
				externalSignal.addEventListener("abort", onExternalAbort, {
					once: true,
				});
			}
		}
		let location = "";
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
			location = resp.headers.get("Location") || "";
			const answer = await resp.text();
			await pc.setRemoteDescription({
				type: "answer",
				sdp: answer,
			});
			return location;
		} catch (err) {
			// 已拿到 Location 但后续失败/被 abort：仍返回 location 给调用方 DELETE。
			if (location) {
				(err as any).resourceUrl = location;
			}
			throw err;
		} finally {
			clearTimeout(timer);
			externalSignal?.removeEventListener("abort", onExternalAbort);
		}
	}

	private async waitForPublishIceConnected(
		timeoutMs = 10_000,
	): Promise<void> {
		const pc = this.publishPc;
		if (!pc) return;
		// 先注册 listener，再检查 state。否则 ICE 在注册前已 ->connected 则事件丢、promise 永不 resolve。
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
			// listener 注册后再检查一次，防中间态竞态
			const s = pc.iceConnectionState;
			if (s === "connected" || s === "completed") {
				cleanup();
				resolve();
			} else if (s === "failed" || s === "closed") {
				cleanup();
				reject(new Error(`SRS publish ICE ${s}`));
			}
		});
	}

	private async cleanupPartialJoin(): Promise<void> {
		for (const [identity] of this.peerSubs) {
			this.unsubscribePeer(identity);
		}
		await this.stopPublish({ stopLocalTracks: true });
		this.hasJoined = false;
		this.micEnabled = true;
	}

	private startAudioLevelLoop(): void {
		if (!this.localStream || !this.micEnabled || !this.onActiveSpeakersCb) return;
		if (this.activeSpeakerRaf !== null) {
			cancelAnimationFrame(this.activeSpeakerRaf);
			this.activeSpeakerRaf = null;
		}
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
		const controller = new AbortController();
		const timer = setTimeout(() => controller.abort(), 3_000);
		try {
			await fetch(url, { method: "DELETE", signal: controller.signal });
		} catch {
			return;
		} finally {
			clearTimeout(timer);
		}
	}
}
