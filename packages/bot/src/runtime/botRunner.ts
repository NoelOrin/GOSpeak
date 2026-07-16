import type { BotContext, Logger as ILogger } from "../core/context";
import { EventBus } from "../core/eventBus";
import type { PluginMetadata } from "../core/plugin";
import { PluginManager } from "../core/pluginManager";
import {
	clearRegistry,
	getHandlersByEventType,
	listPlugins,
} from "../core/registry";
import {
	createBotEvent,
	EventType,
	type LifecycleEvent,
	type SpeechEvent,
} from "../core/types";
import {
	ListenRoomRegistry,
	LiveKitPublishAdapter,
	MediaListenService,
	type PcmStream,
	PcmStreamHub,
	parseRoomList,
	type SFUPublishAdapter,
} from "../media";
import {
	createSpeechBusBridge,
	PassthroughSpeechPipeline,
	type SpeechPipeline,
} from "../speech";
import { ASRManager, createASRProvider } from "../speech/asr";
import { SineTTSProvider, type TTSProvider } from "../tts";
import { createKVStore, GOSpeakApiClient } from "./apiClient";
import { AuthClient, type AuthCredentials } from "./authClient";
import { CapabilityRouter } from "./capabilityRouter";
import { Scheduler } from "./scheduler";
import { GOSpeakSocketClient } from "./socketClient";

export interface BotConfig {
	/** GOSpeak server base URL, e.g. http://localhost:8998 */
	serverUrl: string;
	/** WebSocket/Socket.IO endpoint */
	socketUrl: string;
	/** Bot account JWT — if provided, skips login */
	accessToken?: string;
	/** Bot login credentials — used when accessToken is not provided */
	credentials?: AuthCredentials;
	/** Auto-refresh interval (ms). Default: 20 minutes */
	refreshIntervalMs?: number;
	/** Bot identity name */
	identity: string;
	/** Bot display name */
	displayName: string;
	/** Directory to scan for plugin modules */
	pluginDir?: string;
	/** Watch pluginDir for runtime load/reload (default true when pluginDir set) */
	watchPlugins?: boolean;
	/** Auto-join these rooms on start (signaling only) */
	autoJoinRooms?: string[];
	/** Rooms to listen (SFU side) on start */
	listenRooms?: string[];
	/** Enable media listen + speech pipeline (default true when listenRooms set) */
	enableListen?: boolean;
	/** Use real ASR manager instead of passthrough mock pipeline */
	enableASR?: boolean;
	/** Enable TTS speak / publishPcm (Phase 4) */
	enableSpeak?: boolean;
	/** Debounce for plugin file changes (ms) */
	pluginWatchDebounceMs?: number;
	/** Per-plugin configuration map, keyed by plugin name */
	pluginConfigs?: Record<string, Record<string, unknown>>;
}

export interface BotStatus {
	connected: boolean;
	pluginCount: number;
	handlerCount: number;
	startedAt: number;
	loggedIn: boolean;
	listenRooms: string[];
}

export class BotRunner {
	private config: BotConfig;
	private logger: ILogger;
	private bus!: EventBus;
	private api!: GOSpeakApiClient;
	private socket!: GOSpeakSocketClient;
	private auth!: AuthClient;
	private startedAt = 0;
	private pluginManager: PluginManager | null = null;
	private _refreshTimer: NodeJS.Timeout | null = null;
	private _pcmHub = new PcmStreamHub();
	private _caps!: CapabilityRouter;
	private _scheduler = new Scheduler();
	private _listenRegistry = new ListenRoomRegistry();
	private _listenService: MediaListenService | null = null;
	private _speechPipeline: SpeechPipeline | null = null;
	private _asrManager: ASRManager | null = null;
	private _unsubPcm: (() => void) | null = null;
	private _unsubSpeech: (() => void) | null = null;
	private _publishAdapter: SFUPublishAdapter | null = null;
	private _tts: TTSProvider | null = null;
	private _speakQueues = new Map<string, Promise<void>>();
	private _speakingRooms = new Set<string>();

	constructor(config: BotConfig, logger?: ILogger) {
		this.config = config;
		this.logger = logger ?? console;
	}

	get status(): BotStatus {
		return {
			connected: this.socket?.isConnected ?? false,
			pluginCount: this.pluginManager?.pluginCount ?? 0,
			handlerCount: getHandlersByEventType(EventType.AdapterMessage, false)
				.length,
			startedAt: this.startedAt,
			loggedIn: this.auth?.isLoggedIn ?? false,
			listenRooms: this._listenRegistry.list(),
		};
	}

	get apiClient(): GOSpeakApiClient {
		return this.api;
	}

	get authClient(): AuthClient {
		return this.auth;
	}

	get listenRegistry(): ListenRoomRegistry {
		return this._listenRegistry;
	}

	/**
	 * 进程内 PCM 可读流接口。
	 * 旁听 adapter: `pcmHub.publish(frame)`
	 * 外部/ASR: `pcmStream.subscribe(...)` / `pcmStream.open(...)`
	 */
	get pcmStream(): PcmStream {
		return this._pcmHub;
	}

	/** 同 pcmStream，暴露 publish 给旁听写入侧 */
	get pcmHub(): PcmStreamHub {
		return this._pcmHub;
	}

	async start(): Promise<void> {
		this.startedAt = Date.now();
		this.logger.info(
			`Bot starting — server: ${this.config.serverUrl}, identity: ${this.config.identity}`,
		);

		this.auth = new AuthClient({
			baseUrl: this.config.serverUrl,
			logger: this.logger,
		});

		let accessToken: string;

		if (this.config.accessToken) {
			accessToken = this.config.accessToken;
			this.logger.info("Using pre-provided access token");
		} else if (this.config.credentials) {
			this.logger.info(`Logging in as ${this.config.credentials.username}...`);
			const result = await this.auth.login(this.config.credentials);
			accessToken = result.accessToken;
			this.logger.info(
				`Login successful — user: ${result.username}, role: ${result.role}`,
			);
		} else {
			throw new Error(
				"Either accessToken or credentials must be provided in BotConfig",
			);
		}

		this.api = new GOSpeakApiClient({
			baseUrl: this.config.serverUrl,
			accessToken,
			logger: this.logger,
		});

		if (this.config.credentials && this.config.refreshIntervalMs !== 0) {
			const interval = this.config.refreshIntervalMs ?? 20 * 60 * 1000;
			this._refreshTimer = this.auth.startAutoRefresh(interval, (newToken) => {
				this.api.setAccessToken(newToken);
			});
		}

		this.socket = new GOSpeakSocketClient({
			url: this.config.socketUrl,
			token: accessToken,
			logger: this.logger,
		});

		this._caps = new CapabilityRouter(this.api, this.socket, this, {
			speak: (room, text) => this.speak(room, text),
			publishPcm: (room, pcm, rate) => this.publishPcm(room, pcm, rate),
			stopSpeaking: (room) => this.stopSpeaking(room),
		});

		this.bus = new EventBus({
			buildContext: (pluginName) => this.buildPluginCtx(pluginName),
			getPluginConfig: (pluginName) =>
				this.config.pluginConfigs?.[pluginName] ?? {},
		});

		this.socket.setEventHandler((event) => {
			void this.bus.dispatch(event).catch((err) => {
				this.logger.error("Event dispatch error:", err);
			});
		});

		// Seed listen rooms from config (env merged in main.ts)
		const initialListen = parseRoomList(this.config.listenRooms);
		if (initialListen.length) this._listenRegistry.setAll(initialListen);

		this.pluginManager = new PluginManager({
			pluginDir: this.config.pluginDir,
			watch: this.config.watchPlugins,
			debounceMs: this.config.pluginWatchDebounceMs,
			buildContext: (pluginName) => this.buildPluginCtx(pluginName),
			logger: this.logger,
			onLoaded: async (name) => {
				await this.fireLifecycleEvent(EventType.OnPluginLoaded, name);
			},
			onUnloaded: async (name) => {
				await this.fireLifecycleEvent(EventType.OnPluginUnloaded, name);
			},
		});
		await this.pluginManager.start();
		await this.socket.connect();

		// Auto-join configured rooms (signaling)
		if (this.config.autoJoinRooms && this.config.autoJoinRooms.length > 0) {
			for (const room of this.config.autoJoinRooms) {
				try {
					this.socket.joinRoom(room, this.config.identity);
					this.logger.info(`Auto-joined room: ${room}`);
				} catch (err) {
					this.logger.error(`Failed to auto-join room ${room}:`, err);
				}
			}
		}

		await this.startMediaPipeline();

		this.logger.info(
			`Bot started — ${this.status.pluginCount} plugins, ${this.status.handlerCount} handlers, listen=${this._listenRegistry.list().join(",") || "-"}`,
		);

		await this.fireLifecycleEvent(EventType.OnBotLoaded);
	}

	async stop(): Promise<void> {
		if (this._refreshTimer) {
			clearInterval(this._refreshTimer);
			this._refreshTimer = null;
		}
		this._scheduler.clearAll();
		this._unsubPcm?.();
		this._unsubPcm = null;
		this._unsubSpeech?.();
		this._unsubSpeech = null;
		this._speechPipeline?.dispose();
		this._speechPipeline = null;
		this._asrManager?.dispose();
		this._asrManager = null;
		if (this._listenService) {
			await this._listenService.stop();
			this._listenService = null;
		}
		if (this._publishAdapter) {
			await this._publishAdapter.dispose();
			this._publishAdapter = null;
		}
		this._pcmHub.clear();
		this.socket?.disconnect();
		if (this.pluginManager) {
			await this.pluginManager.stop();
			this.pluginManager = null;
		}
		await this.auth?.logout();
		clearRegistry();
		this.logger.info("Bot stopped");
	}

	async sendChat(roomId: string, content: string): Promise<void> {
		this.socket.sendBotMessage(roomId, content);
	}

	async joinRoom(roomName: string, opts?: { sfu?: boolean }): Promise<void> {
		const identity = this.config.identity;
		if (!this.socket.isConnected) {
			this.logger.warn("socket not connected; cannot join room");
			return;
		}

		if (opts?.sfu) {
			const sfuToken = await this.api.getSFUToken(roomName);
			await this.socket.joinRoomSFU(roomName, identity, sfuToken.serverUrl);
		} else {
			this.socket.joinRoom(roomName, identity);
		}
	}

	leaveRoom(roomName: string): void {
		this.socket.leaveRoom(roomName);
	}

	get joinedRooms(): string[] {
		return this.socket.rooms;
	}

	/** Phase 4: synthesize + publish PCM to room */
	async speak(roomId: string, text: string): Promise<void> {
		if (!this.config.enableSpeak && !this._tts) {
			// lazy init if not configured but called
			this._tts = new SineTTSProvider();
			this._publishAdapter =
				this._publishAdapter ?? new LiveKitPublishAdapter(this.logger);
		}
		if (!this._tts || !this._publishAdapter) {
			throw new Error("speak not enabled");
		}
		const prev = this._speakQueues.get(roomId) ?? Promise.resolve();
		const next = prev
			.catch(() => {})
			.then(async () => {
				this._speakingRooms.add(roomId);
				try {
					const publisher = this._publishAdapter;
					const tts = this._tts;
					if (!publisher || !tts) throw new Error("speak not enabled");
					const token = await this.api.getSFUToken(roomId);
					await publisher.join({
						room: roomId,
						identity: this.config.identity,
						token: token.token,
						serverUrl: token.serverUrl,
					});
					const pcm = await tts.synthesize(text);
					await publisher.publishPcm(roomId, pcm, 16000);
				} finally {
					this._speakingRooms.delete(roomId);
				}
			});
		this._speakQueues.set(roomId, next);
		await next;
	}

	async publishPcm(
		roomId: string,
		pcm16: Int16Array,
		sampleRate = 16000,
	): Promise<void> {
		if (!this._publishAdapter) {
			this._publishAdapter = new LiveKitPublishAdapter(this.logger);
		}
		const token = await this.api.getSFUToken(roomId);
		await this._publishAdapter.join({
			room: roomId,
			identity: this.config.identity,
			token: token.token,
			serverUrl: token.serverUrl,
		});
		await this._publishAdapter.publishPcm(roomId, pcm16, sampleRate);
	}

	async stopSpeaking(roomId: string): Promise<void> {
		await this._publishAdapter?.unpublish(roomId);
		this._speakingRooms.delete(roomId);
	}

	/** 运行时加载：从绝对路径或 pluginDir 内相对路径加载插件 */
	async loadPlugin(pathOrName: string): Promise<PluginMetadata> {
		if (!this.pluginManager) {
			throw new Error("Bot not started");
		}
		const managed = await this.pluginManager.loadFromPath(pathOrName, true);
		return managed.metadata;
	}

	/** 运行时卸载 */
	async unloadPlugin(name: string): Promise<void> {
		if (!this.pluginManager) return;
		await this.pluginManager.unload(name);
	}

	/** 运行时重载（不传 name 则全量） */
	async reloadPlugin(name?: string): Promise<void> {
		if (!this.pluginManager) return;
		await this.pluginManager.reload(name);
	}

	/** 安装外部插件到 pluginDir 并加载 */
	async installPlugin(sourcePath: string): Promise<PluginMetadata> {
		if (!this.pluginManager) throw new Error("Bot not started");
		const managed = await this.pluginManager.installFromPath(sourcePath);
		return managed.metadata;
	}

	/** 启用/禁用插件（不卸载） */
	setPluginActivated(name: string, activated: boolean): void {
		this.pluginManager?.setActivated(name, activated);
	}

	/** 当前已加载插件元数据 */
	listPlugins(): PluginMetadata[] {
		return this.pluginManager?.list() ?? listPlugins();
	}

	private async startMediaPipeline(): Promise<void> {
		const enableListen =
			this.config.enableListen ??
			(this._listenRegistry.list().length > 0 ||
				Boolean(this.config.listenRooms?.length));

		if (!enableListen) return;

		// Speech / ASR path
		const bridge = createSpeechBusBridge((event: SpeechEvent) => {
			void this.bus.dispatch(event).catch((err) => {
				this.logger.error("Speech event dispatch error:", err);
			});
		});

		if (this.config.enableASR) {
			const provider = createASRProvider(process.env);
			this._asrManager = new ASRManager({
				provider,
				botIdentity: this.config.identity,
				onResult: bridge,
			});
			this._unsubPcm = this._pcmHub.subscribe((frame) => {
				// pause ASR while speaking in same room to reduce self-feedback
				if (this._speakingRooms.has(frame.room)) return;
				this._asrManager?.write(frame);
			});
			this.logger.info(`[asr] provider=${provider.name}`);
		} else {
			this._speechPipeline = new PassthroughSpeechPipeline({
				framesPerFinal: 5,
			});
			this._unsubSpeech = this._speechPipeline.onResult(bridge);
			this._unsubPcm = this._pcmHub.subscribe((frame) => {
				if (this._speakingRooms.has(frame.room)) return;
				this._speechPipeline?.write(frame);
			});
			this.logger.info("[speech] using passthrough mock pipeline");
		}

		if (this.config.enableSpeak) {
			this._tts = new SineTTSProvider();
			this._publishAdapter = new LiveKitPublishAdapter(this.logger);
		}

		this._listenService = new MediaListenService({
			registry: this._listenRegistry,
			pcmSink: this._pcmHub,
			logger: this.logger,
			identity: this.config.identity,
			getSFUToken: (room) => this.api.getSFUToken(room),
			joinSignaling: (room) => this.joinRoom(room),
			leaveSignaling: (room) => this.leaveRoom(room),
			onTrackEnded: (info) => {
				this._asrManager?.endTrack(info.room, info.identity);
				this._speechPipeline?.endTrack(info.room, info.identity);
			},
		});
		await this._listenService.start();
	}

	private buildPluginCtx(pluginName: string): BotContext {
		return {
			logger: this.logger,
			config: this.config.pluginConfigs?.[pluginName] ?? {},
			pluginName,
			chat: this._caps,
			rooms: this._caps,
			voice: this._caps,
			users: this._caps,
			mutes: {
				list: () => this._caps.listMutes(),
				status: (userId: number) => this._caps.getMuteStatus(userId),
			},
			scheduler: {
				every: (id, ms, fn) =>
					this._scheduler.every(`${pluginName}:${id}`, ms, fn),
				once: (id, ms, fn) =>
					this._scheduler.once(`${pluginName}:${id}`, ms, fn),
				clear: (id) => this._scheduler.clear(`${pluginName}:${id}`),
				clearAll: () => this._scheduler.clearByPrefix(`${pluginName}:`),
			},
			listen: {
				add: (room) => this._listenRegistry.add(room),
				remove: (room) => this._listenRegistry.remove(room),
				list: () => this._listenRegistry.list(),
				clear: () => this._listenRegistry.clear(),
			},
			kv: createKVStore(),
			hasPermission: (_level, _member) => true,
		};
	}

	private async fireLifecycleEvent(
		eventType: LifecycleEvent["eventType"],
		pluginName?: string,
	): Promise<void> {
		await this.bus.dispatch(createBotEvent(eventType, pluginName));
	}
}
