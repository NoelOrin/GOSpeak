import type {
	AudioFrame as LiveKitAudioFrame,
	AudioSource as LiveKitAudioSource,
	LocalAudioTrack as LiveKitLocalAudioTrack,
	Room as LiveKitRoom,
	RoomEvent as LiveKitRoomEvent,
	TrackPublishOptions as LiveKitTrackPublishOptions,
	TrackSource as LiveKitTrackSource,
} from "@livekit/rtc-node";
import type { SFUPublishAdapter } from "./types";

interface Logger {
	info: (...a: unknown[]) => void;
	warn: (...a: unknown[]) => void;
}

export interface LiveKitPublishRtcModule {
	Room: new () => LiveKitRoom;
	RoomEvent: typeof LiveKitRoomEvent;
	AudioSource: typeof LiveKitAudioSource;
	LocalAudioTrack: typeof LiveKitLocalAudioTrack;
	TrackPublishOptions: typeof LiveKitTrackPublishOptions;
	TrackSource: typeof LiveKitTrackSource;
	AudioFrame: typeof LiveKitAudioFrame;
}

interface RoomState {
	room: LiveKitRoom;
	token: string;
	serverUrl: string;
	source: LiveKitAudioSource | null;
	track: LiveKitLocalAudioTrack | null;
	trackSid: string | null;
	sampleRate: number;
}

/**
 * LiveKit publish adapter using @livekit/rtc-node.
 *
 * Connects as a real participant, publishes a local microphone audio track and
 * feeds captured PCM frames into the LiveKit room.
 */
export class LiveKitPublishAdapter implements SFUPublishAdapter {
	private rooms = new Map<string, RoomState>();
	private logger?: Logger;
	private rtc?: LiveKitPublishRtcModule;
	private rtcPromise: Promise<LiveKitPublishRtcModule> | null = null;

	constructor(logger?: Logger, rtc?: LiveKitPublishRtcModule) {
		this.logger = logger;
		this.rtc = rtc;
	}

	async join(params: {
		room: string;
		identity: string;
		token: string;
		serverUrl: string;
	}): Promise<void> {
		const existing = this.rooms.get(params.room);
		if (existing) {
			if (existing.room.isConnected) return;
			await this.leave(params.room);
		}

		const rtc = await this.loadRtc();
		const room = new rtc.Room();
		const state: RoomState = {
			room,
			token: params.token,
			serverUrl: params.serverUrl,
			source: null,
			track: null,
			trackSid: null,
			sampleRate: 0,
		};
		this.rooms.set(params.room, state);

		room.on(rtc.RoomEvent.Disconnected, () => {
			const current = this.rooms.get(params.room);
			if (current !== state) return;
			void state.source?.close().catch(() => {});
			this.rooms.delete(params.room);
		});

		try {
			await room.connect(params.serverUrl, params.token, {
				autoSubscribe: false,
				dynacast: true,
			});
			this.logger?.info(
				`[livekit-publish] joined ${params.room} as ${params.identity}`,
			);
		} catch (err) {
			this.rooms.delete(params.room);
			throw err;
		}
	}

	async publishPcm(
		room: string,
		pcm16: Int16Array,
		sampleRate = 16000,
	): Promise<void> {
		const state = this.rooms.get(room);
		if (!state?.room.isConnected) {
			throw new Error(`not joined for publish: ${room}`);
		}
		const rtc = await this.loadRtc();
		if (!state.source) {
			state.sampleRate = sampleRate;
			const source = new rtc.AudioSource(sampleRate, 1);
			const track = rtc.LocalAudioTrack.createAudioTrack(
				"gospeak-bot-voice",
				source,
			);
			state.source = source;
			state.track = track;
			const options = new rtc.TrackPublishOptions();
			options.source = rtc.TrackSource.SOURCE_MICROPHONE;
			if (!state.room.localParticipant) {
				throw new Error(`livekit room has no local participant: ${room}`);
			}
			const publication = await state.room.localParticipant.publishTrack(
				track,
				options,
			);
			state.trackSid = publication?.sid ?? null;
		}
		if (sampleRate !== state.sampleRate) {
			throw new Error(
				`livekit publish source is ${state.sampleRate} Hz, got ${sampleRate} Hz`,
			);
		}
		if (pcm16.length === 0) return;
		if (!state.source) {
			throw new Error(`livekit publish source missing: ${room}`);
		}

		const frame = new rtc.AudioFrame(pcm16, sampleRate, 1, pcm16.length);
		await state.source.captureFrame(frame);
		this.logger?.info(
			`[livekit-publish] publishPcm room=${room} samples=${pcm16.length} rate=${sampleRate}`,
		);
	}

	async unpublish(room: string): Promise<void> {
		const state = this.rooms.get(room);
		if (!state) return;
		if (state.trackSid) {
			try {
				await state.room.localParticipant?.unpublishTrack(state.trackSid);
			} catch (err) {
				this.logger?.warn(`[livekit-publish] unpublish error ${room}:`, err);
			}
		}
		await state.track?.close().catch(() => {});
		await state.source?.close().catch(() => {});
		state.track = null;
		state.source = null;
		state.trackSid = null;
		this.logger?.info(`[livekit-publish] unpublish ${room}`);
	}

	async leave(room: string): Promise<void> {
		const state = this.rooms.get(room);
		if (!state) return;
		await this.unpublish(room);
		try {
			await state.room.disconnect();
		} catch (err) {
			this.logger?.warn(`[livekit-publish] disconnect error ${room}:`, err);
		}
		this.rooms.delete(room);
	}

	async dispose(): Promise<void> {
		for (const room of [...this.rooms.keys()]) await this.leave(room);
		this.rooms.clear();
	}

	private async loadRtc(): Promise<LiveKitPublishRtcModule> {
		if (this.rtc) return this.rtc;
		this.rtcPromise ??= import("@livekit/rtc-node");
		return this.rtcPromise;
	}
}
