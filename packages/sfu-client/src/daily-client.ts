import type {
	DailyCall,
	DailyEventObjectActiveSpeakerChange,
	DailyEventObjectNetworkConnectionEvent,
	DailyEventObjectParticipant,
	DailyEventObjectParticipantLeft,
	DailyParticipantTracks,
} from "@daily-co/daily-js";
import type {
	JoinParams,
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUClient,
	SFUClientOptions,
} from "./types";

class DailyRemoteAudioTrack implements RemoteAudioTrackLike {
	private elements: HTMLAudioElement[] = [];

	constructor(private readonly track: MediaStreamTrack) {}

	attach(): HTMLMediaElement {
		const element = document.createElement("audio");
		element.autoplay = true;
		element.srcObject = new MediaStream([this.track]);
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

export class DailySFUClient implements SFUClient {
	private callObject: DailyCall | null = null;
	private remoteTracks = new Map<string, DailyRemoteAudioTrack>();
	private onActiveSpeakersCb?: (ids: string[]) => void;
	private onRemoteAudioTrackCb?: (info: RemoteTrackInfo) => void;
	private onRemoteAudioTrackRemovedCb?: (identity: string) => void;
	private onDisconnectedCb?: () => void;
	private onReconnectingCb?: () => void;
	private onReconnectedCb?: () => void;
	private hasJoined = false;

	constructor() {}

	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: url, identity, room } = params;
		const dailyModule = await import("@daily-co/daily-js");
		const daily = dailyModule.default;
		const callObject = daily.createCallObject();
		this.callObject = callObject;
		this.bindEvents(callObject);
		const resolvedURL = this.resolveRoomURL(url, room);
		await callObject.join({
			url: resolvedURL,
			token,
			userName: identity,
		});
		callObject.setLocalAudio(true);
		this.hasJoined = true;
	}

	async leaveRoom(): Promise<void> {
		this.hasJoined = false;
		if (!this.callObject) return;
		const callObject = this.callObject;
		this.callObject = null;
		this.onReconnectingCb = undefined;
		this.onReconnectedCb = undefined;
		this.remoteTracks.forEach((track) => track.detach());
		this.remoteTracks.clear();
		await callObject.leave();
		await callObject.destroy();
	}

	async setMicEnabled(enabled: boolean): Promise<void> {
		this.callObject?.setLocalAudio(enabled);
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

	isConnected(): boolean {
		return this.hasJoined;
	}

	async destroy(): Promise<void> {
		await this.leaveRoom();
	}

	private bindEvents(callObject: DailyCall): void {
		callObject.on("active-speaker-change", (event: DailyEventObjectActiveSpeakerChange) => {
			const id = event?.activeSpeaker?.peerId;
			this.onActiveSpeakersCb?.(id ? [id] : []);
		});

		callObject.on("participant-joined", (event: DailyEventObjectParticipant) => {
			this.publishRemoteTrack(event.participant.session_id, event.participant.tracks);
		});

		callObject.on("participant-updated", (event: DailyEventObjectParticipant) => {
			this.publishRemoteTrack(event.participant.session_id, event.participant.tracks);
		});

		callObject.on("participant-left", (event: DailyEventObjectParticipantLeft) => {
			const id = event.participant.session_id;
			this.remoteTracks.get(id)?.detach();
			this.remoteTracks.delete(id);
			this.onRemoteAudioTrackRemovedCb?.(id);
		});

		// 网络层断开/恢复：Daily SDK 自动重连，此处映射为 reconnecting 信号
		callObject.on("network-connection", (event: DailyEventObjectNetworkConnectionEvent) => {
			if (!this.hasJoined) return;
			this.onReconnectingCb?.();
		});

		// joined-meeting 在已 joined 状态下复触发 = 重连后重新加入
		callObject.on("joined-meeting", () => {
			if (!this.hasJoined) return;
			this.onReconnectedCb?.();
		});

		// 非主动 leave 触发的离会视为异常断连
		callObject.on("left-meeting", () => {
			if (this.callObject) {
				this.onDisconnectedCb?.();
			}
		});
	}

	private publishRemoteTrack(identity: string, tracks?: DailyParticipantTracks): void {
		const audioTrack = tracks?.audio?.track;
		if (!audioTrack) {
			return;
		}
		if (this.remoteTracks.has(identity)) {
			return;
		}
		const remoteTrack = new DailyRemoteAudioTrack(audioTrack);
		this.remoteTracks.set(identity, remoteTrack);
		this.onRemoteAudioTrackCb?.({ identity, track: remoteTrack });
	}

	private resolveRoomURL(url: string, room?: string): string {
		const trimmed = url.replace(/\/$/, "");
		if (!room) {
			return trimmed;
		}
		return `${trimmed}/${room}`;
	}
}
