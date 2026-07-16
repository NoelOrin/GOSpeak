import type { BotContext, Logger as ILogger } from "../core/context";
import { EventBus } from "../core/eventBus";
import type { PluginMetadata } from "../core/plugin";
import { PluginManager } from "../core/pluginManager";
import {
	clearRegistry,
	getHandlersByEventType,
	listPlugins,
} from "../core/registry";
import { createBotEvent, EventType, type LifecycleEvent } from "../core/types";
import { type PcmStream, PcmStreamHub } from "../media";
import { createKVStore, GOSpeakApiClient } from "./apiClient";
import { AuthClient, type AuthCredentials } from "./authClient";
import { GOSpeakSocketClient } from "./socketClient";

export interface BotConfig {
	/** GOSpeak server base URL, e.g. http://localhost:8998  */
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
		};
	}

	get apiClient(): GOSpeakApiClient {
		return this.api;
	}

	get authClient(): AuthClient {
		return this.auth;
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

		this.bus = new EventBus({
			buildContext: (pluginName) => this.buildPluginCtx(pluginName),
			getPluginConfig: (pluginName) =>
				this.config.pluginConfigs?.[pluginName] ?? {},
		});

		this.socket = new GOSpeakSocketClient({
			url: this.config.socketUrl,
			token: accessToken,
			logger: this.logger,
		});
		this.socket.setEventHandler((event) => {
			void this.bus.dispatch(event).catch((err) => {
				this.logger.error("Event dispatch error:", err);
			});
		});

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

		this.logger.info(
			`Bot started — ${this.status.pluginCount} plugins, ${this.status.handlerCount} handlers`,
		);

		await this.fireLifecycleEvent(EventType.OnBotLoaded);
	}

	async stop(): Promise<void> {
		if (this._refreshTimer) {
			clearInterval(this._refreshTimer);
			this._refreshTimer = null;
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
		await this.api.send(roomId, content);
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

	private buildPluginCtx(pluginName: string): BotContext {
		return {
			logger: this.logger,
			config: this.config.pluginConfigs?.[pluginName] ?? {},
			pluginName,
			chat: this.api,
			rooms: {
				listRooms: () => this.api.listRooms(),
				getMembers: (roomId: string) => this.api.getMembers(roomId),
				createRoom: (name: string, limit?: number) =>
					this.api.createRoom(name, limit),
				join: (name: string, o?: { sfu?: boolean }) => this.joinRoom(name, o),
				leave: (name: string) => this.leaveRoom(name),
				joined: () => this.joinedRooms,
			},
			voice: this.api,
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
