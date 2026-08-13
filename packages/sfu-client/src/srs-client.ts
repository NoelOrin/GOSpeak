import type {
	JoinParams,
	RemoteTrackInfo,
	SFUClient,
	SFUClientOptions,
	SignalSocket,
} from "./types";

import { acquireStream, appendStream, enqueueStreamOp, isWhipBusyError, releaseStream, sleep, WHIP_BUSY_RETRY, WHIP_BUSY_RETRY_MS } from "./srs-stream-gate";
import { SRSRemoteAudioTrack } from "./srs-track";
import { LocalSpeakingMeter } from "./speaking-meter";

interface PeerSub {
	identity: string;
	stream: string;
	pc: RTCPeerConnection | null;
	resourceUrl: string;
	retryCount: number;
	retryTimer: ReturnType<typeof setTimeout> | null;
	connecting: boolean;
}


export class SRSSFUClient implements SFUClient {
	private publishPc: RTCPeerConnection | null = null;
	private localStream: MediaStream | null = null;
	private remoteTracks = new Map<string, SRSRemoteAudioTrack>();
	private peerSubs = new Map<string, PeerSub>();
	private onActiveSpeakersCb?: (ids: string[]) => void;
	private onLocalSpeakingChangeCb?: (speaking: boolean) => void;
	private onRemoteAudioTrackCb?: (info: RemoteTrackInfo) => void;
	private onRemoteAudioTrackRemovedCb?: (identity: string) => void;
	private onDisconnectedCb?: () => void;
	private onReconnectingCb?: () => void;
	private onReconnectedCb?: () => void;
	private hasJoined = false;
	private isReconnecting = false;
	private onPcStateChangeBound?: () => void;
	// 本地发言检测：ScriptProcessorNode 事件驱动采样（无 JS 定时器轮询），
	// 起麦需连续 120ms、停麦需连续 300ms 才翻转，抑制阈值抖动闪烁。
	private speakingMeter: LocalSpeakingMeter;
	public static readonly MAX_SUBSCRIBE_RETRIES = 60;
	private publishResourceUrl = "";
	private identity = "";
	private room = "";
	private ownStream = "";
	private streamToken = "";
	private roomToken = "";
	private whipUrl = "";
	private baseWhepUrl = "";
	private socket?: SignalSocket | null;
	private leaving = false;
	private leavePromise: Promise<void> | null = null;
	// 递增代数：abort/leave 与 in-flight WHIP 竞态时，旧 publish 不得回写状态。
	private joinGeneration = 0;
	private mediaAbort: AbortController | null = null;
	private activeStreamKey = "";
	// leave 在 join 早期 abort 时，用 pendingStreamKey 命中正确 stream 闸门。
	private pendingStreamKey = "";
	private micEnabled = true;
	private socketMemberJoinedCleanup?: () => void;
	private socketMemberLeftCleanup?: () => void;

	constructor(private readonly options: SFUClientOptions = {}) {
		this.socket = options.socket;
		this.speakingMeter = new LocalSpeakingMeter({
			threshold: 10,
			holdOnMs: 120,
			holdOffMs: 300,
			onSpeakingChange: (speaking) => this.onLocalSpeakingChangeCb?.(speaking),
		});
	}

	async joinRoom(params: JoinParams): Promise<void> {
		const streamKey = params.stream || `${params.room || ""}:${params.identity || ""}`;
		// 同 client 已在同 stream：幂等返回，避免 effect 重入二次 WHIP。
		if (
			this.hasJoined &&
			!this.leaving &&
			this.activeStreamKey === streamKey &&
			this.publishPc &&
			this.publishResourceUrl
		) {
			return;
		}
		// 尽早绑定 stream key，leave 在 join 中途 abort 时仍能命中同一闸门。
		this.pendingStreamKey = streamKey;
		this.identity = params.identity || this.identity;
		this.room = params.room || this.room;
		this.ownStream = params.stream || streamKey;
		await enqueueStreamOp(streamKey, () => this.joinRoomLocked(params, streamKey));
	}

	private async joinRoomLocked(
		params: JoinParams,
		streamKey: string,
	): Promise<void> {
		// 新 join 代数先占坑：并发 leave 只会 bump 代数，不会被后面 leaving=false 抹掉。
		const joinGen = ++this.joinGeneration;
		this.leaving = false;
		this.pendingStreamKey = streamKey;

		// 同 client 复用前先拆旧会话，避免 WHIP stream busy / 双 publish。
		if (this.hasJoined || this.publishPc || this.publishResourceUrl) {
			await this.cleanupPartialJoin();
		}
		if (this.leaving || joinGen !== this.joinGeneration) {
			throw new Error("SRS join aborted");
		}

		// 等旧 holder（可能是被 abort 的另一实例）释放 stream 后再 WHIP。
		await acquireStream(
			streamKey,
			this,
			() => this.leaving || joinGen !== this.joinGeneration,
		);
		if (this.leaving || joinGen !== this.joinGeneration) {
			releaseStream(streamKey, this);
			throw new Error("SRS join aborted");
		}
		this.activeStreamKey = streamKey;

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
			this.socketMemberJoinedCleanup = this.socket.onServerEvent(
				"member:joined",
				(data: any) => {
					if (!this.isSameRoomEvent(data)) return;
					if (data.identity && data.stream) {
						this.subscribePeer(data.identity, data.stream);
					}
				},
			);
			this.socketMemberLeftCleanup = this.socket.onServerEvent(
				"member:left",
				(data: any) => {
					if (!this.isSameRoomEvent(data)) return;
					if (data.identity) {
						this.unsubscribePeer(data.identity);
					}
				},
			);
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
		const domain = typeof data.domain_uuid === "string" ? data.domain_uuid : "";
		const composite = domain ? `${domain}:${data.room}` : data.room;
		return data.room === this.room || composite === this.room;
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
		// leave 必须不入队：
		// 新 join 可能正卡在 acquire 等本 holder 释放；若 leave 再入同一 FIFO，会死锁。
		// beginLeaveAbort 打断 in-flight WHIP，doLeave 直接 DELETE + release gate。
		this.beginLeaveAbort();
		const streamKey =
			this.activeStreamKey ||
			this.pendingStreamKey ||
			this.ownStream ||
			`${this.room || ""}:${this.identity || ""}` ||
			"__empty__";
		await this.leaveRoomLocked(streamKey);
	}

	private beginLeaveAbort(): void {
		this.leaving = true;
		this.joinGeneration++;
		this.hasJoined = false;
		this.isReconnecting = false;
		this.mediaAbort?.abort();
		this.mediaAbort = null;
	}

	private async leaveRoomLocked(streamKeyHint = ""): Promise<void> {
		if (this.leavePromise) {
			await this.leavePromise;
			return;
		}

		this.leavePromise = this.doLeaveRoom(streamKeyHint);
		try {
			await this.leavePromise;
		} finally {
			this.leavePromise = null;
		}
	}

	private async doLeaveRoom(streamKeyHint = ""): Promise<void> {
		// leaveRoom 已 beginLeaveAbort；这里只做资源拆卸。
		this.leaving = true;
		this.hasJoined = false;
		this.isReconnecting = false;

		const streamKey =
			streamKeyHint ||
			this.activeStreamKey ||
			this.pendingStreamKey ||
			this.ownStream ||
			"";

		// Remove socket listeners BEFORE tearing media
		this.socketMemberJoinedCleanup?.();
		this.socketMemberLeftCleanup?.();
		this.socketMemberJoinedCleanup = undefined;
		this.socketMemberLeftCleanup = undefined;

		this.room = "";
		this.roomToken = "";
		this.streamToken = "";
		this.ownStream = "";
		this.pendingStreamKey = "";

		// Unsubscribe all peers (callbacks suppressed via leaving flag)
		for (const [identity] of this.peerSubs) {
			this.unsubscribePeer(identity);
		}

		// 先释放闸门，让排队的新 join 能进入；DELETE 异步收尾。
		if (streamKey) {
			releaseStream(streamKey, this);
		}
		this.activeStreamKey = "";
		this.pendingStreamKey = "";

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
			// 停麦：立即上报停麦并拆除分析，静音期间不再有空转回调。
			this.speakingMeter.stop();
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
			this.startSpeakingMeter();
			return;
		}

		const media = await this.getLocalAudioStream();
		const newTrack = media.getAudioTracks()[0];
		if (!newTrack) {
			media.getTracks().forEach((t) => t.stop());
			return;
		}
		newTrack.enabled = true;

		// 仅 hard-fail 才重建 WHIP。
		// connecting/disconnected 瞬态很常见；重建会二次 WHIP 撞 5020。
		const pcState = this.publishPc?.connectionState ?? "new";
		const hardFail =
			!this.publishPc ||
			!this.publishResourceUrl ||
			pcState === "failed" ||
			pcState === "closed";
		if (hardFail) {
			this.localStream = media;
			this.micEnabled = true;
			await this.startPublish();
			this.startSpeakingMeter();
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
		this.startSpeakingMeter();
	}


	private async getLocalAudioStream(): Promise<MediaStream> {
		const preferred: MediaTrackConstraints = {
			echoCancellation: this.options.audioCapture?.echoCancellation ?? true,
			noiseSuppression: this.options.audioCapture?.noiseSuppression ?? true,
			autoGainControl: this.options.audioCapture?.autoGainControl ?? true,
		};
		if (this.options.audioCapture?.deviceId) {
			preferred.deviceId = { exact: this.options.audioCapture.deviceId };
		}
		// sampleRate/sampleSize/channelCount 不是所有设备都支持；失败时降级。
		if (this.options.audioCapture?.channelCount) {
			preferred.channelCount = this.options.audioCapture.channelCount;
		}
		try {
			return await this.getUserMediaWithTimeout({
				audio: preferred,
				video: false,
			});
		} catch (err) {
			console.warn("[SRS] getUserMedia preferred constraints failed, fallback", err);
			return await this.getUserMediaWithTimeout({
				audio: true,
				video: false,
			});
		}
	}

	private async getUserMediaWithTimeout(
		constraints: MediaStreamConstraints,
		timeoutMs = 10_000,
	): Promise<MediaStream> {
		// 不用 Promise.race+timer：fake timers 测试下悬挂 timer 会拖垮后续用例。
		// 超时用可取消包装。
		let timer: ReturnType<typeof setTimeout> | null = null;
		let timedOut = false;
		const timeoutPromise = new Promise<never>((_, reject) => {
			timer = setTimeout(() => {
				timedOut = true;
				reject(new Error("getUserMedia timeout"));
			}, timeoutMs);
		});
		try {
			const media = await Promise.race([
				navigator.mediaDevices.getUserMedia(constraints),
				timeoutPromise,
			]);
			return media;
		} finally {
			if (timer) clearTimeout(timer);
			void timedOut;
		}
	}

	private async startPublish(joinGen = this.joinGeneration): Promise<void> {
		if (this.publishPc || this.publishResourceUrl) {
			await this.stopPublish({ stopLocalTracks: true });
		}
		if (this.leaving || joinGen !== this.joinGeneration) {
			throw new Error("SRS join aborted");
		}

		// 约束尽量保守：复杂 sampleRate/sampleSize 在部分浏览器会挂起 getUserMedia。
		const localStream = await this.getLocalAudioStream();
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
			// 旧会话 DELETE 后 SRS 可能短暂 busy；忙时退避重试，避免“第一次成功第二次失败”。
			let lastErr: unknown;
			for (let attempt = 0; attempt <= WHIP_BUSY_RETRY; attempt++) {
				if (this.leaving || joinGen !== this.joinGeneration) {
					throw new Error("SRS join aborted");
				}
				try {
					resourceUrl = await this.exchangeSdp(
						publishPc,
						whipUrl,
						this.roomToken,
						true,
						mediaAbort.signal,
					);
					lastErr = undefined;
					break;
				} catch (err) {
					lastErr = err;
					const busy = isWhipBusyError(err);
					if (!busy || attempt >= WHIP_BUSY_RETRY) throw err;
					await sleep(WHIP_BUSY_RETRY_MS * (attempt + 1));
				}
			}
			if (lastErr) throw lastErr;
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
		this.startSpeakingMeter();
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
		this.speakingMeter.stop();

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

	private startSpeakingMeter(): void {
		if (!this.localStream || !this.micEnabled) return;
		this.speakingMeter.start(this.localStream);
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
