import AgoraRTC, {
	type IAgoraRTCClient,
	type ILocalAudioTrack,
	type IRemoteAudioTrack,
} from "agora-rtc-sdk-ng";
import type {
	JoinParams,
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUClient,
	SFUClientOptions,
} from "./types";

class AgoraRemoteAudioTrack implements RemoteAudioTrackLike {
	private elements: HTMLAudioElement[] = [];

	constructor(private track: IRemoteAudioTrack) {}

	attach(): HTMLMediaElement {
		const audioElement = document.createElement("audio");
		audioElement.autoplay = true;
		audioElement.srcObject = new MediaStream([this.track.getMediaStreamTrack()]);
		this.elements.push(audioElement);
		return audioElement;
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
		// Agora SDK 音量为 0-100，统一对外契约 0-1 → 内部 *100。
		this.track.setVolume(Math.max(0, Math.min(1, volume)) * 100);
	}

	stop(): void {
		this.track.stop();
		this.detach();
	}
}

export class AgoraSFUClient implements SFUClient {
	private client: IAgoraRTCClient;
	private localAudioTrack: ILocalAudioTrack | null = null;
	private remoteAudioTracks = new Map<string, AgoraRemoteAudioTrack>();
	private onActiveSpeakersCb?: (ids: string[]) => void;
	private onRemoteAudioTrackCb?: (info: RemoteTrackInfo) => void;
	private onRemoteAudioTrackRemovedCb?: (identity: string) => void;
	private onDisconnectedCb?: () => void;
	private onReconnectingCb?: () => void;
	private onReconnectedCb?: () => void;
	private hasJoined = false;
	private prevConnState?: "DISCONNECTED" | "CONNECTING" | "RECONNECTING" | "CONNECTED" | "DISCONNECTING";

	constructor(private options: SFUClientOptions = {}) {
		this.client = AgoraRTC.createClient({ mode: "rtc", codec: "vp8" });
		this.client.enableAudioVolumeIndicator();
		this.registerClientEvents();
	}

	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: appId, identity, room } = params;
		const channelName = room || appId;
		const resolvedAppId = appId || this.envAgoraAppId();
		if (!resolvedAppId) {
			throw new Error("Agora App ID is required");
		}

		await this.client.join(resolvedAppId, channelName, token, identity);
		this.hasJoined = true;
		this.localAudioTrack = await AgoraRTC.createMicrophoneAudioTrack({
			microphoneId: this.options.audioCapture?.deviceId || undefined,
			AEC: this.options.audioCapture?.echoCancellation ?? true,
			ANS: this.options.audioCapture?.noiseSuppression ?? true,
			AGC: this.options.audioCapture?.autoGainControl ?? true,
			encoderConfig: {
				sampleRate: this.options.audioCapture?.sampleRate,
				stereo: this.options.audioCapture?.channelCount === 2,
				bitrate: this.options.publishAudio?.maxBitrate,
			},
		});
		await this.client.publish(this.localAudioTrack);
	}

	async leaveRoom(): Promise<void> {
		this.hasJoined = false;
		this.prevConnState = undefined;
		if (this.localAudioTrack) {
			this.localAudioTrack.close();
			this.localAudioTrack = null;
		}
		this.remoteAudioTracks.forEach((track) => track.stop());
		this.remoteAudioTracks.clear();
		this.onReconnectingCb = undefined;
		this.onReconnectedCb = undefined;
		this.client?.removeAllListeners?.();
		await this.client.leave();
	}

	async setMicEnabled(enabled: boolean): Promise<void> {
		await this.localAudioTrack?.setEnabled(enabled);
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
		return Array.from(this.remoteAudioTracks.entries()).map(([identity, track]) => ({
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

	isConnected(): boolean {
		return this.hasJoined;
	}

	async destroy(): Promise<void> {
		await this.leaveRoom();
	}

	private registerClientEvents(): void {
		this.client.on("user-published", async (user, mediaType) => {
			await this.client.subscribe(user, mediaType);
			if (mediaType !== "audio" || !user.audioTrack) return;

			const identity = user.uid.toString();
			const track = new AgoraRemoteAudioTrack(user.audioTrack);
			this.remoteAudioTracks.set(identity, track);
			this.onRemoteAudioTrackCb?.({ identity, track });
		});

		this.client.on("user-unpublished", (user, mediaType) => {
			if (mediaType !== "audio") return;

			const identity = user.uid.toString();
			this.remoteAudioTracks.get(identity)?.stop();
			this.remoteAudioTracks.delete(identity);
			this.onRemoteAudioTrackRemovedCb?.(identity);
		});

		this.client.on("volume-indicator", (volumes) => {
			this.onActiveSpeakersCb?.(
				volumes
					.filter((volume) => volume.level > 5)
					.map((volume) => volume.uid.toString()),
			);
		});

		this.client.on("connection-state-change", (state) => {
			if (!this.hasJoined) {
				this.prevConnState = state;
				return;
			}
			if (state === "RECONNECTING") {
				this.onReconnectingCb?.();
			} else if (state === "CONNECTED" && this.prevConnState === "RECONNECTING") {
				this.onReconnectedCb?.();
			} else if (state === "DISCONNECTED") {
				this.hasJoined = false;
				this.onDisconnectedCb?.();
			}
			this.prevConnState = state;
		});
	}

	private envAgoraAppId(): string {
		return (
			(import.meta as ImportMeta & { env?: Record<string, string> }).env
				?.VITE_AGORA_APP_ID || ""
		);
	}
}
