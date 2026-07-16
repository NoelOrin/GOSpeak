import type { Logger } from "../core/context";
import type { ListenRoomRegistry } from "./listenRegistry";
import type {
	SFUListenAdapter,
	SFUListenJoinParams,
	SFUProviderName,
} from "./listenTypes";
import { SFUListenRouter } from "./sfuListenRouter";
import type { AudioFrameEvent, PcmStreamSink } from "./types";

export interface MediaListenDeps {
	registry: ListenRoomRegistry;
	pcmSink: PcmStreamSink;
	logger: Logger;
	identity: string;
	getSFUToken: (room: string) => Promise<{
		token: string;
		serverUrl: string;
		provider?: string;
		stream?: string;
		streamToken?: string;
	}>;
	joinSignaling: (room: string) => Promise<void> | void;
	leaveSignaling: (room: string) => void;
	/** default provider when token omits it */
	defaultProvider?: SFUProviderName;
	router?: SFUListenRouter;
	onFrame?: (frame: AudioFrameEvent) => void;
	onTrackEnded?: (info: { room: string; identity: string }) => void;
}

/**
 * Reconciles desired listen rooms vs active SFU listen sessions.
 * Frames are written to the in-process pcm hub (not published to SFU).
 */
export class MediaListenService {
	private deps: MediaListenDeps;
	private router: SFUListenRouter;
	private active = new Map<string, { provider: SFUProviderName }>();
	private started = false;
	private unsubRegistry: (() => void) | null = null;
	private frameWired = new WeakSet<SFUListenAdapter>();

	constructor(deps: MediaListenDeps) {
		this.deps = deps;
		this.router = deps.router ?? new SFUListenRouter({ logger: deps.logger });
	}

	async start(): Promise<void> {
		if (this.started) return;
		this.started = true;
		this.unsubRegistry = this.deps.registry.onChange(() => {
			void this.reconcile().catch((err) => {
				this.deps.logger.error("[listen] reconcile error:", err);
			});
		});
		await this.reconcile();
	}

	async stop(): Promise<void> {
		this.unsubRegistry?.();
		this.unsubRegistry = null;
		for (const room of [...this.active.keys()]) {
			await this.leaveRoom(room);
		}
		await this.router.disposeAll();
		this.started = false;
	}

	get activeRooms(): string[] {
		return [...this.active.keys()].sort();
	}

	async reconcile(): Promise<void> {
		const desired = new Set(this.deps.registry.list());
		// leave removed
		for (const room of [...this.active.keys()]) {
			if (!desired.has(room)) await this.leaveRoom(room);
		}
		// join added
		for (const room of desired) {
			if (!this.active.has(room)) await this.joinRoom(room);
		}
	}

	private async joinRoom(room: string): Promise<void> {
		try {
			await this.deps.joinSignaling(room);
			const token = await this.deps.getSFUToken(room);
			const provider = (token.provider ||
				this.deps.defaultProvider ||
				"livekit") as SFUProviderName;
			const adapter = this.router.get(provider);
			this.wireAdapter(adapter);
			const params: SFUListenJoinParams = {
				room,
				identity: this.deps.identity,
				token: token.token,
				serverUrl: token.serverUrl,
				provider,
				stream: token.stream,
				streamToken: token.streamToken,
			};
			await adapter.join(params);
			this.active.set(room, { provider });
			this.deps.logger.info(`[listen] listening on ${room} via ${provider}`);
		} catch (err) {
			this.deps.logger.error(`[listen] failed to join ${room}:`, err);
		}
	}

	private async leaveRoom(room: string): Promise<void> {
		const state = this.active.get(room);
		if (!state) return;
		try {
			const adapter = this.router.get(state.provider);
			await adapter.leave(room);
		} catch (err) {
			this.deps.logger.warn(`[listen] leave adapter error for ${room}:`, err);
		}
		this.active.delete(room);
		try {
			this.deps.leaveSignaling(room);
		} catch {
			// ignore
		}
		this.deps.logger.info(`[listen] stopped listening on ${room}`);
	}

	private wireAdapter(adapter: SFUListenAdapter): void {
		if (this.frameWired.has(adapter)) return;
		this.frameWired.add(adapter);
		adapter.onAudioFrame((frame) => {
			// ignore bot self identity to reduce self-feedback risk early
			if (frame.identity === this.deps.identity) return;
			this.deps.pcmSink.publish(frame);
			this.deps.onFrame?.(frame);
		});
		adapter.onTrackEnded((info) => {
			this.deps.onTrackEnded?.(info);
		});
	}
}
