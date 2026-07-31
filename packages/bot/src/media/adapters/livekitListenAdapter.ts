import type {
	AudioFrame as LiveKitAudioFrame,
	RemoteAudioTrack as LiveKitRemoteAudioTrack,
	RemoteParticipant as LiveKitRemoteParticipant,
	RemoteTrack as LiveKitRemoteTrack,
	Room as LiveKitRoom,
	RoomEvent as LiveKitRoomEvent,
} from "@livekit/rtc-node";
import type {
	SFUListenAdapter,
	SFUListenJoinParams,
	SFUProviderName,
} from "../listenTypes";
import type { AudioFrameEvent } from "../types";

type FrameCb = (frame: AudioFrameEvent) => void;
type EndCb = (info: { room: string; identity: string }) => void;

interface Logger {
	info: (...a: unknown[]) => void;
	warn: (...a: unknown[]) => void;
	error: (...a: unknown[]) => void;
}

export interface LiveKitListenRtcModule {
	Room: new () => LiveKitRoom;
	RoomEvent: typeof LiveKitRoomEvent;
	RemoteAudioTrack: typeof LiveKitRemoteAudioTrack;
	AudioStream: new (
		track: LiveKitRemoteAudioTrack,
		sampleRate?: number,
		numChannels?: number,
	) => ReadableStream<LiveKitAudioFrame>;
}

interface RoomState {
	room: LiveKitRoom;
	token: string;
	serverUrl: string;
	identities: Set<string>;
	readers: Map<ReadableStreamDefaultReader<LiveKitAudioFrame>, string>;
}

/**
 * LiveKit listen adapter using @livekit/rtc-node.
 *
 * Joins the room as a real LiveKit participant, subscribes to remote audio
 * tracks and emits 16 kHz mono PCM frames into the in-process media hub.
 */
export class LiveKitListenAdapter implements SFUListenAdapter {
	readonly provider: SFUProviderName = "livekit";
	private frameCb: FrameCb | null = null;
	private endCb: EndCb | null = null;
	private rooms = new Map<string, RoomState>();
	private logger?: Logger;
	private rtc?: LiveKitListenRtcModule;
	private rtcPromise: Promise<LiveKitListenRtcModule> | null = null;

	constructor(logger?: Logger, rtc?: LiveKitListenRtcModule) {
		this.logger = logger;
		this.rtc = rtc;
	}

	async join(params: SFUListenJoinParams): Promise<void> {
		if (this.rooms.has(params.room)) return;
		const rtc = await this.loadRtc();
		const room = new rtc.Room();

		const state: RoomState = {
			room,
			token: params.token,
			serverUrl: params.serverUrl,
			identities: new Set(),
			readers: new Map(),
		};
		this.rooms.set(params.room, state);

		room.on(
			rtc.RoomEvent.TrackSubscribed,
			(
				track: LiveKitRemoteTrack,
				_publication: unknown,
				participant: LiveKitRemoteParticipant,
			) => {
				if (track instanceof rtc.RemoteAudioTrack) {
					void this.consumeTrack(
						params.room,
						track,
						participant.identity,
						track.sid ?? "",
						rtc,
						state,
					);
				}
			},
		);

		room.on(
			rtc.RoomEvent.TrackUnsubscribed,
			(
				_track: LiveKitRemoteTrack,
				publication: { sid?: string },
				participant: LiveKitRemoteParticipant,
			) => {
				this.cancelReaders(state, publication.sid ?? "");
				if (state.identities.delete(participant.identity)) {
					this.endCb?.({ room: params.room, identity: participant.identity });
				}
			},
		);

		room.on(rtc.RoomEvent.Disconnected, () => {
			const current = this.rooms.get(params.room);
			if (current !== state) return;
			this.cancelReaders(state);
			for (const identity of [...state.identities]) {
				this.endCb?.({ room: params.room, identity });
			}
			state.identities.clear();
			this.rooms.delete(params.room);
		});

		try {
			await room.connect(params.serverUrl, params.token, {
				autoSubscribe: true,
				dynacast: true,
			});
			this.logger?.info(
				`[livekit-listen] joined ${params.room} as ${params.identity}`,
			);
		} catch (err) {
			this.rooms.delete(params.room);
			this.logger?.error(`[livekit-listen] join failed ${params.room}:`, err);
			throw err;
		}
	}

	async leave(room: string): Promise<void> {
		const state = this.rooms.get(room);
		if (!state) return;
		this.cancelReaders(state);
		try {
			await state.room.disconnect();
		} catch (err) {
			this.logger?.warn(`[livekit-listen] disconnect error ${room}:`, err);
		}
		for (const identity of [...state.identities]) {
			this.endCb?.({ room, identity });
		}
		state.identities.clear();
		this.rooms.delete(room);
		this.logger?.info(`[livekit-listen] left ${room}`);
	}

	onAudioFrame(cb: FrameCb): void {
		this.frameCb = cb;
	}

	onTrackEnded(cb: EndCb): void {
		this.endCb = cb;
	}

	listActiveIdentities(room: string): string[] {
		return [...(this.rooms.get(room)?.identities ?? [])];
	}

	async dispose(): Promise<void> {
		for (const room of [...this.rooms.keys()]) await this.leave(room);
		this.frameCb = null;
		this.endCb = null;
	}

	/** Allow tests / external bridges to push decoded frames */
	publishFrame(frame: AudioFrameEvent): void {
		const state = this.rooms.get(frame.room);
		if (state) state.identities.add(frame.identity);
		this.frameCb?.(frame);
	}

	private async consumeTrack(
		roomName: string,
		track: LiveKitRemoteAudioTrack,
		identity: string,
		trackSid: string,
		rtc: LiveKitListenRtcModule,
		state: RoomState,
	): Promise<void> {
		let reader: ReadableStreamDefaultReader<LiveKitAudioFrame> | undefined;
		try {
			const stream = new rtc.AudioStream(track, 16000, 1);
			reader = stream.getReader();
			state.readers.set(reader, trackSid);
			while (true) {
				const { value, done } = await reader.read();
				if (done) break;
				if (!value || value.samplesPerChannel === 0) continue;
				state.identities.add(identity);
				this.frameCb?.({
					room: roomName,
					identity,
					pcm16: value.data,
					sampleRate: value.sampleRate,
					channels: value.channels,
					timestamp: Date.now(),
					mediaProvider: "livekit",
				});
			}
			state.readers.delete(reader);
			if (state.identities.delete(identity)) {
				this.endCb?.({ room: roomName, identity });
			}
		} catch (err) {
			this.logger?.warn(
				`[livekit-listen] audio stream ended with error ${roomName} ${identity}:`,
				err,
			);
			if (reader) state.readers.delete(reader);
			if (state.identities.delete(identity)) {
				this.endCb?.({ room: roomName, identity });
			}
		}
	}

	private cancelReaders(state: RoomState, trackSid?: string): void {
		for (const [reader, sid] of [...state.readers]) {
			if (trackSid !== undefined && sid !== trackSid) continue;
			state.readers.delete(reader);
			void reader.cancel().catch(() => {});
		}
	}

	private async loadRtc(): Promise<LiveKitListenRtcModule> {
		if (this.rtc) return this.rtc;
		this.rtcPromise ??= import("@livekit/rtc-node");
		return this.rtcPromise;
	}
}
