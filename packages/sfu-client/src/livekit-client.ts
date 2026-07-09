import {
	type Participant,
	type RemoteAudioTrack,
	type RemoteParticipant,
	type RemoteTrack,
	type RemoteTrackPublication,
	LogLevel,
	Room,
	RoomEvent,
	DisconnectReason,
	Track,
	VideoPresets,
	setLogLevel,
} from "livekit-client";
import type {
	JoinParams,
	RemoteTrackInfo,
	SFUClient,
	SFUClientOptions,
	SFUDisconnectInfo,
} from "./types";

export class LiveKitSFUClient implements SFUClient {
	private room: Room;
	private onActiveSpeakersCb?: (ids: string[]) => void;
	private onRemoteAudioTrackCb?: (info: RemoteTrackInfo) => void;
	private onRemoteAudioTrackRemovedCb?: (identity: string) => void;
	private onDisconnectedCb?: (info?: SFUDisconnectInfo) => void;
	private onReconnectingCb?: () => void;
	private onReconnectedCb?: () => void;
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
			audioCaptureDefaults: options.audioCapture,
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
					_publication: RemoteTrackPublication,
					participant: RemoteParticipant,
				) => {
					if (track.kind !== Track.Kind.Audio) return;
					this.onRemoteAudioTrackCb?.({
						identity: participant.identity,
						track: track as RemoteAudioTrack,
					});
				},
			)
			.on(
				RoomEvent.TrackUnsubscribed,
				(
					track: RemoteTrack,
					_publication: RemoteTrackPublication,
					participant: RemoteParticipant,
				) => {
					if (track.kind !== Track.Kind.Audio) return;
					this.onRemoteAudioTrackRemovedCb?.(participant.identity);
				},
			);
	}

	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: url, identity: _identity } = params;
		await this.room.prepareConnection(url, token);
		await this.room.connect(url, token);
		// connect 失败会抛异常，此处仅防御性检查 room 仍 connected 才启麦
		if (this.room.state === "connected") {
			await this.room.localParticipant.setMicrophoneEnabled(true);
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
		this.onDisconnectedCb = undefined;
		this.onReconnectingCb = undefined;
		this.onReconnectedCb = undefined;
		await this.room.localParticipant.setMicrophoneEnabled(false);
		await this.room.localParticipant.setCameraEnabled(false);
		this.room.localParticipant.trackPublications.forEach((publication) => {
			publication.track?.stop();
		});
		await this.room.disconnect();
	}

	async setMicEnabled(enabled: boolean): Promise<void> {
		await this.room.localParticipant.setMicrophoneEnabled(enabled);
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
	if (reason == null || reason === DisconnectReason.CLIENT_INITIATED) return undefined;
	return DISCONNECT_REASON_NAMES[reason] ?? "UNKNOWN_REASON";
}
