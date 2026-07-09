import * as fs from "node:fs";
import * as path from "node:path";
import { EventBus } from "../core/eventBus";
import { EventType, createBotEvent, type LifecycleEvent } from "../core/types";
import type { BotContext, Logger as ILogger } from "../core/context";
import { Plugin } from "../core/plugin";
import {
	getHandlersByEventType,
	clearRegistry,
	removeHandlersByModule,
} from "../core/registry";
import { loadPlugin, initPlugin } from "../core/loader";
import { GOSpeakApiClient } from "./apiClient";
import { GOSpeakSocketClient } from "./socketClient";
import { createKVStore } from "./apiClient";
import { AuthClient, type AuthCredentials } from "./authClient";

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
	private _running = false;
	private _plugins: { instance: Plugin; name: string }[] = [];
	private _loadedPaths = new Set<string>();
	private _refreshTimer: NodeJS.Timeout | null = null;

	constructor(config: BotConfig, logger?: ILogger) {
		this.config = config;
		this.logger = logger ?? console;
	}

	get status(): BotStatus {
		return {
			connected: this.socket?.isConnected ?? false,
			pluginCount: this._plugins.length,
			handlerCount: getHandlersByEventType(EventType.AdapterMessage, false).length,
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
			getPluginConfig: (pluginName) => this.config.pluginConfigs?.[pluginName] ?? {},
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

		await this.loadAllPlugins();
		this.socket.connect();

		this._running = true;
		this.logger.info(
			`Bot started — ${this._plugins.length} plugins, ${this.status.handlerCount} handlers`,
		);

		await this.fireLifecycleEvent(EventType.OnBotLoaded);
	}

	async stop(): Promise<void> {
		this._running = false;
		if (this._refreshTimer) {
			clearInterval(this._refreshTimer);
			this._refreshTimer = null;
		}
		this.socket?.disconnect();
		for (const p of this._plugins) {
			await p.instance.onUnload?.();
		}
		await this.auth?.logout();
		clearRegistry();
		this._plugins = [];
		this._loadedPaths.clear();
		this.logger.info("Bot stopped");
	}

	async sendChat(roomId: string, content: string): Promise<void> {
		await this.api.send(roomId, content);
	}

	async joinRoom(roomName: string, opts?: { sfu?: boolean }): Promise<void> {
		const identity = this.config.identity;

		if (opts?.sfu) {
			const sfuToken = await this.api.getSFUToken(roomName, identity);
			await this.socket.joinRoomSFU(roomName, identity, sfuToken.stream);
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

	async reloadPlugin(name: string): Promise<void> {
		const old = this._plugins.find((p) => p.name === name);
		if (!old) return;
		const absPath = [...this._loadedPaths].find((p) =>
			path.basename(p, path.extname(p)) === name,
		);
		if (!absPath) {
			this.logger.warn(`Cannot reload plugin ${name}: module path not tracked`);
			return;
		}
		const modulePath = `user_plugins/${name}`;
		removeHandlersByModule(modulePath);
		await old.instance.onUnload?.();
		this._plugins = this._plugins.filter((p) => p.name !== name);
		await this.loadSinglePlugin(absPath);
		this.logger.info(`Reloaded plugin: ${name}`);
	}

	private buildPluginCtx(pluginName: string): BotContext {
		return {
			logger: this.logger,
			config: this.config.pluginConfigs?.[pluginName] ?? {},
			pluginName,
			chat: this.api,
			rooms: Object.assign(this.api, {
				join: (name: string, o?: { sfu?: boolean }) => this.joinRoom(name, o),
				leave: (name: string) => this.leaveRoom(name),
				joined: () => this.joinedRooms,
			}) as BotContext['rooms'],
			voice: this.api,
			kv: createKVStore(),
			hasPermission: (_level, _member) => true,
		};
	}

	private async loadAllPlugins(): Promise<void> {
		const pluginDir = this.config.pluginDir;
		if (!pluginDir) {
			this.logger.info("No pluginDir configured; skipping auto-discovery");
			return;
		}
		if (!fs.existsSync(pluginDir)) {
			this.logger.warn(`Plugin directory not found: ${pluginDir}`);
			return;
		}

		const entries = fs.readdirSync(pluginDir, { withFileTypes: true });
		const pluginFiles = entries.filter(
			(e: fs.Dirent) =>
				e.isFile() && (e.name.endsWith(".ts") || e.name.endsWith(".js")),
		);

		for (const file of pluginFiles) {
			const absPath = path.resolve(pluginDir, file.name);
			if (this._loadedPaths.has(absPath)) continue;
			await this.loadSinglePlugin(absPath);
		}
	}

	private async loadSinglePlugin(absPath: string): Promise<void> {
		try {
			const modulePath = `user_plugins/${path.basename(absPath, path.extname(absPath))}`;
			const loaded = await loadPlugin(absPath, modulePath);
			initPlugin(loaded, (name) => this.buildPluginCtx(name));
			this._plugins.push({ instance: loaded.instance, name: loaded.metadata.name });
			this._loadedPaths.add(absPath);
			await this.fireLifecycleEvent(EventType.OnPluginLoaded, loaded.metadata.name);
			this.logger.info(`Loaded plugin: ${loaded.metadata.name} (${modulePath})`);
		} catch (err) {
			this.logger.error(`Failed to load plugin ${absPath}:`, err);
		}
	}

	private async fireLifecycleEvent(
		eventType: LifecycleEvent["eventType"],
		pluginName?: string,
	): Promise<void> {
		await this.bus.dispatch(createBotEvent(eventType, pluginName));
	}
}
