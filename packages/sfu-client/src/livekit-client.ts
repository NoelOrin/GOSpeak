import {
	type LocalVideoTrack,
	type Participant,
	type RemoteAudioTrack,
	type RemoteParticipant,
	type RemoteTrack,
	type RemoteTrackPublication,
	type RemoteVideoTrack,
	LogLevel,
	Room,
	RoomEvent,
	DisconnectReason,
	Track,
	VideoPresets,
	setLogLevel,
} from "livekit-client";
import {
	computeScreenShareLayerEncodings,
	DEFAULT_SCREEN_SHARE_QUALITY,
	SCREEN_SHARE_CAPTURE_RESOLUTION,
	type ScreenShareQuality,
	screenShareQualityPreset,
} from "./screen-share-quality";
import type {
	JoinParams,
	RemoteTrackInfo,
	RemoteVideoTrackInfo,
	RemoteVideoTrackLike,
	SFUClient,
	SFUClientOptions,
	SFUDisconnectInfo,
} from "./types";

export class LiveKitSFUClient implements SFUClient {
	private room: Room;
	private onActiveSpeakersCb?: (ids: string[]) => void;
	private onRemoteAudioTrackCb?: (info: RemoteTrackInfo) => void;
	private onRemoteAudioTrackRemovedCb?: (identity: string) => void;
	private onRemoteVideoTrackCb?: (info: RemoteVideoTrackInfo) => void;
	private onRemoteVideoTrackRemovedCb?: (identity: string) => void;
	private onLocalScreenShareChangedCb?: (sharing: boolean) => void;
	private onDisconnectedCb?: (info?: SFUDisconnectInfo) => void;
	private onReconnectingCb?: () => void;
	private onReconnectedCb?: () => void;
	private screenShareQuality: ScreenShareQuality = DEFAULT_SCREEN_SHARE_QUALITY;
	private hasLeft = false;

	constructor(options: SFUClientOptions = {}) {
		setLogLevel(LogLevel.warn);
		this.room = new Room({
			adaptiveStream: true,
			dynacast: true,
			stopLocalTrackOnUnpublish: true,
			disconnectOnPageLeave: true,
			videoCaptureDefaults: {
				resolution: VideoPresets.h720.resolution,
			},
			// 必须给对象而不是 undefined：switchActiveDevice 会读写 audioCaptureDefaults.deviceId
			audioCaptureDefaults: options.audioCapture ?? {},
			publishDefaults: {
				audioPreset: options.publishAudio?.maxBitrate
					? { maxBitrate: options.publishAudio.maxBitrate }
					: undefined,
				dtx: options.publishAudio?.dtx,
				red: options.publishAudio?.red,
				forceStereo: options.publishAudio?.forceStereo,
			},
			webAudioMix: true,
			reconnectPolicy: {
				nextRetryDelayInMs: (context: { retryCount: number }) => {
					if (context.retryCount >= 3) return null;
					return Math.min(1000 * 2 ** context.retryCount, 8000);
				},
			},
		});

		this.room
			.on(RoomEvent.Disconnected, (reason?: DisconnectReason) => {
				if (this.hasLeft) return;
				const unrecoverable = isUnrecoverableDisconnect(reason);
				if (unrecoverable) {
					// 不可恢复断连（如重复身份）：立即强制断开，阻止 LiveKit 用同一 token 自动重连
					// 再次创建 duplicate participant 进而形成踢人风暴。close() 异步，提前置 hasLeft
					// 防止 WebSocket onClose 触发的 handleDisconnect 竞态重连。
					this.hasLeft = true;
					this.room.removeAllListeners();
					this.onReconnectingCb = undefined;
					this.onReconnectedCb = undefined;
					this.onActiveSpeakersCb = undefined;
					this.onRemoteAudioTrackCb = undefined;
					this.onRemoteAudioTrackRemovedCb = undefined;
					this.onRemoteVideoTrackCb = undefined;
					this.onRemoteVideoTrackRemovedCb = undefined;
					this.onLocalScreenShareChangedCb = undefined;
					void this.room.disconnect().catch(() => {});
				}
				this.onDisconnectedCb?.({
					reason: reasonName(reason),
					unrecoverable,
				});
			})
			.on(RoomEvent.Reconnecting, () => {
				this.onReconnectingCb?.();
			})
			.on(RoomEvent.Reconnected, () => {
				this.onReconnectedCb?.();
			})
			.on(RoomEvent.ActiveSpeakersChanged, (speakers: Participant[]) => {
				this.onActiveSpeakersCb?.(speakers.map((speaker) => speaker.identity));
			})
			.on(
				RoomEvent.TrackSubscribed,
				(
					track: RemoteTrack,
					publication: RemoteTrackPublication,
					participant: RemoteParticipant,
				) => {
					if (track.kind === Track.Kind.Audio) {
						this.onRemoteAudioTrackCb?.({
							identity: participant.identity,
							track: track as RemoteAudioTrack,
						});
						return;
					}
					if (track.kind === Track.Kind.Video) {
						this.onRemoteVideoTrackCb?.({
							identity: participant.identity,
							track: track as RemoteVideoTrack,
							source:
								publication.source === Track.Source.ScreenShare
									? "screen"
									: "camera",
						});
					}
				},
			)
			.on(
				RoomEvent.TrackUnsubscribed,
				(
					track: RemoteTrack,
					_publication: RemoteTrackPublication,
					participant: RemoteParticipant,
				) => {
					if (track.kind === Track.Kind.Audio) {
						this.onRemoteAudioTrackRemovedCb?.(participant.identity);
						return;
					}
					if (track.kind === Track.Kind.Video) {
						this.onRemoteVideoTrackRemovedCb?.(participant.identity);
					}
				},
			)
			.on(RoomEvent.LocalTrackPublished, (publication) => {
				if (publication.source === Track.Source.ScreenShare) {
					this.onLocalScreenShareChangedCb?.(true);
				}
			})
			.on(RoomEvent.LocalTrackUnpublished, (publication) => {
				if (publication.source === Track.Source.ScreenShare) {
					this.onLocalScreenShareChangedCb?.(false);
				}
			});
	}

	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: url, identity: _identity } = params;
		await this.room.prepareConnection(url, token);
		await this.room.connect(url, token);
		// connect 失败会抛异常，此处仅防御性检查 room 仍 connected 才启麦
		if (this.room.state === "connected") {
			try {
				await this.room.localParticipant.setMicrophoneEnabled(true);
			} catch (err) {
				// 麦克风权限被拒**不应该**导致整次进房失败：
				// 语音产品的底线是"能听见别人"，先把人放进房间，麦克风由用户后续自行开启。
				console.warn("[sfu] microphone unavailable, joined as listener:", err);
			}
		}
	}

	async leaveRoom(): Promise<void> {
		if (this.hasLeft) return;
		this.hasLeft = true;
		// Remove all listeners first to prevent stale reconnect handling
		this.room.removeAllListeners();
		this.onActiveSpeakersCb = undefined;
		this.onRemoteAudioTrackCb = undefined;
		this.onRemoteAudioTrackRemovedCb = undefined;
		this.onRemoteVideoTrackCb = undefined;
		this.onRemoteVideoTrackRemovedCb = undefined;
		this.onLocalScreenShareChangedCb = undefined;
		this.onDisconnectedCb = undefined;
		this.onReconnectingCb = undefined;
		this.onReconnectedCb = undefined;
		await this.room.localParticipant.setMicrophoneEnabled(false);
		await this.room.localParticipant.setCameraEnabled(false);
		await this.room.localParticipant
			.setScreenShareEnabled(false)
			.catch(() => {});
		this.room.localParticipant.trackPublications.forEach((publication) => {
			publication.track?.stop();
		});
		await this.room.disconnect();
	}

	async setMicEnabled(enabled: boolean): Promise<void> {
		await this.room.localParticipant.setMicrophoneEnabled(enabled);
	}

	/**
	 * 热切换麦克风：走 LiveKit 的 switchActiveDevice，内部对已发布的
	 * 麦克风 track 调 setDeviceId，保留发布状态与静音状态。
	 * 未发布时只更新 audioCaptureDefaults，下次开麦即用新设备。
	 */
	async setAudioInputDevice(deviceId: string): Promise<void> {
		// exact=false：设备刚被拔掉时不要因 constraint 报错中断，交给浏览器回退默认
		await this.room.switchActiveDevice(
			"audioinput",
			deviceId || "default",
			false,
		);
	}

	async setScreenShareEnabled(enabled: boolean): Promise<void> {
		// 只共享画面，不采集系统音频，避免与麦克风链路混在一起
		const preset = screenShareQualityPreset(this.screenShareQuality);
		await this.room.localParticipant.setScreenShareEnabled(
			enabled,
			{
				audio: false,
				contentHint: preset.contentHint,
				resolution: SCREEN_SHARE_CAPTURE_RESOLUTION,
			},
			{ screenShareEncoding: preset.encoding },
		);
		if (enabled) {
			await this.applyScreenShareQuality(this.screenShareQuality);
		}
	}

	async setScreenShareQuality(quality: ScreenShareQuality): Promise<void> {
		this.screenShareQuality = quality;
		await this.applyScreenShareQuality(quality);
	}

	getScreenShareQuality(): ScreenShareQuality {
		return this.screenShareQuality;
	}

	private localScreenShareVideoTrack(): LocalVideoTrack | null {
		const publication = this.room.localParticipant.getTrackPublication(
			Track.Source.ScreenShare,
		);
		return publication?.videoTrack ?? null;
	}

	/**
	 * 共享中改档：只重设已发布 track 的编码参数，不重新采集。
	 *
	 * 重新走 getDisplayMedia 会再弹一次窗口选择框，而且共享会话会断；
	 * 这里保持采集尺寸不变，用 scaleResolutionDownBy 让编码器输出档位目标高度。
	 * 失败只降级（保持原画质继续共享），不打断共享。
	 */
	private async applyScreenShareQuality(
		quality: ScreenShareQuality,
	): Promise<void> {
		const track = this.localScreenShareVideoTrack();
		const sender = track?.sender;
		if (!track || !sender) return;
		try {
			const capturedHeight =
				track.mediaStreamTrack.getSettings?.().height ?? undefined;
			const params = sender.getParameters();
			const current = params.encodings ?? [];
			if (current.length === 0) return;
			const next = computeScreenShareLayerEncodings(
				current.length,
				quality,
				capturedHeight,
			);
			params.encodings = current.map((existing, index) => {
				const layer = next[index];
				// dynacast 会把被停用的层标 active=false，改它的参数没有意义
				if (!layer || existing.active === false) return existing;
				return {
					...existing,
					scaleResolutionDownBy: layer.scaleResolutionDownBy,
					maxBitrate: layer.maxBitrate,
					maxFramerate: layer.maxFramerate,
				};
			});
			await sender.setParameters(params);
			track.mediaStreamTrack.contentHint =
				screenShareQualityPreset(quality).contentHint;
		} catch (error) {
			console.warn("[sfu] apply screen share quality failed:", error);
		}
	}

	getLocalScreenShareTrack(): RemoteVideoTrackLike | null {
		const publication = this.room.localParticipant.getTrackPublication(
			Track.Source.ScreenShare,
		);
		return (
			(publication?.videoTrack as RemoteVideoTrackLike | undefined) ?? null
		);
	}

	onLocalScreenShareChanged(cb: (sharing: boolean) => void): void {
		this.onLocalScreenShareChangedCb = cb;
	}

	onRemoteVideoTrack(cb: (info: RemoteVideoTrackInfo) => void): void {
		this.onRemoteVideoTrackCb = cb;
	}

	onRemoteVideoTrackRemoved(cb: (identity: string) => void): void {
		this.onRemoteVideoTrackRemovedCb = cb;
	}

	getExistingRemoteVideoTracks(): RemoteVideoTrackInfo[] {
		const out: RemoteVideoTrackInfo[] = [];
		for (const participant of this.room.remoteParticipants.values()) {
			for (const publication of participant.videoTrackPublications.values()) {
				const track = publication.track;
				if (!track) continue;
				out.push({
					identity: participant.identity,
					track: track as RemoteVideoTrack,
					source:
						publication.source === Track.Source.ScreenShare
							? "screen"
							: "camera",
				});
			}
		}
		return out;
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
		const out: RemoteTrackInfo[] = [];
		for (const participant of this.room.remoteParticipants.values()) {
			for (const publication of participant.audioTrackPublications.values()) {
				const track = publication.track;
				if (track && track.kind === Track.Kind.Audio) {
					out.push({
						identity: participant.identity,
						track: track as RemoteAudioTrack,
					});
				}
			}
		}
		return out;
	}

	onDisconnected(cb: (info?: SFUDisconnectInfo) => void): void {
		this.onDisconnectedCb = cb;
	}

	onReconnecting(cb: () => void): void {
		this.onReconnectingCb = cb;
	}

	onReconnected(cb: () => void): void {
		this.onReconnectedCb = cb;
	}

	isConnected(): boolean {
		return !this.hasLeft && this.room.state === "connected";
	}

	async destroy(): Promise<void> {
		await this.leaveRoom();
	}
}

const UNRECOVERABLE_DISCONNECT_REASONS = new Set<DisconnectReason>([
	DisconnectReason.DUPLICATE_IDENTITY,
	DisconnectReason.PARTICIPANT_REMOVED,
	DisconnectReason.ROOM_DELETED,
	DisconnectReason.JOIN_FAILURE,
	DisconnectReason.ROOM_CLOSED,
	DisconnectReason.USER_UNAVAILABLE,
	DisconnectReason.USER_REJECTED,
]);

function isUnrecoverableDisconnect(reason?: DisconnectReason): boolean {
	return !!reason && UNRECOVERABLE_DISCONNECT_REASONS.has(reason);
}

const DISCONNECT_REASON_NAMES: Partial<Record<DisconnectReason, string>> = {
	[DisconnectReason.UNKNOWN_REASON]: "UNKNOWN_REASON",
	[DisconnectReason.CLIENT_INITIATED]: "CLIENT_INITIATED",
	[DisconnectReason.DUPLICATE_IDENTITY]: "DUPLICATE_IDENTITY",
	[DisconnectReason.PARTICIPANT_REMOVED]: "PARTICIPANT_REMOVED",
	[DisconnectReason.ROOM_DELETED]: "ROOM_DELETED",
	[DisconnectReason.JOIN_FAILURE]: "JOIN_FAILURE",
	[DisconnectReason.ROOM_CLOSED]: "ROOM_CLOSED",
	[DisconnectReason.USER_UNAVAILABLE]: "USER_UNAVAILABLE",
	[DisconnectReason.USER_REJECTED]: "USER_REJECTED",
};

function reasonName(reason?: DisconnectReason): string | undefined {
	if (reason == null || reason === DisconnectReason.CLIENT_INITIATED)
		return undefined;
	return DISCONNECT_REASON_NAMES[reason] ?? "UNKNOWN_REASON";
}
