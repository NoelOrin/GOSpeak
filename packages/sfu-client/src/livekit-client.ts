import {
	type Participant,
	type RemoteAudioTrack,
	type RemoteParticipant,
	type RemoteTrack,
	type RemoteTrackPublication,
	Room,
	RoomEvent,
	Track,
	VideoPresets,
} from "livekit-client";
import type { JoinParams, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";

export class LiveKitSFUClient implements SFUClient {
	private room: Room;
	private onActiveSpeakersCb?: (ids: string[]) => void;
	private onRemoteAudioTrackCb?: (info: RemoteTrackInfo) => void;
	private onRemoteAudioTrackRemovedCb?: (identity: string) => void;
	private onDisconnectedCb?: () => void;
	private onReconnectingCb?: () => void;
	private onReconnectedCb?: () => void;
	private hasLeft = false;

	constructor(options: SFUClientOptions = {}) {
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
			.on(RoomEvent.Disconnected, () => {
				this.onDisconnectedCb?.();
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
		await this.room.localParticipant.setMicrophoneEnabled(true);
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
		return !this.hasLeft && this.room.state === "connected";
	}

	async destroy(): Promise<void> {
		await this.leaveRoom();
	}
}
