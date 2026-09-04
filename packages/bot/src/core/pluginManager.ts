import * as fs from "node:fs";
import * as path from "node:path";
import type { BotContext, Logger } from "./context";
import {
	initPlugin,
	type LoadedPlugin,
	loadPlugin,
	unloadPlugin,
} from "./loader";
import type { PluginMetadata } from "./plugin";
import {
	findModulePathByName,
	getPluginInstance,
	listPlugins,
	setPluginActivated,
} from "./registry";
import {
	copyRecursive,
	discoverPlugins,
	PLUGIN_EXTS,
	pluginModulePath,
	resolvePluginEntry,
} from "./pluginDiscovery";

export interface PluginManagerOptions {
	/** 用户插件目录（data/plugins 对应） */
	pluginDir?: string;
	/** 是否监视目录变化并热重载，默认 true（有 pluginDir 时） */
	watch?: boolean;
	/** 文件变化防抖 ms，默认 300 */
	debounceMs?: number;
	/** 构建插件 ctx */
	buildContext: (pluginName: string) => BotContext;
	logger?: Logger;
	/** 生命周期钩子 */
	onLoaded?: (name: string) => void | Promise<void>;
	onUnloaded?: (name: string) => void | Promise<void>;
}

export interface ManagedPlugin {
	name: string;
	modulePath: string;
	absPath: string;
	metadata: PluginMetadata;
	loaded: LoadedPlugin;
}

/**
 * 运行时插件管理器（对齐 AstrBot PluginManager 核心能力）：
 * - 扫描 pluginDir 下的 .ts/.js 与 子目录/main.ts
 * - load / unload / reload / installFromPath
 * - 可选 fs.watch 热重载
 */
export class PluginManager {
	private opts: PluginManagerOptions;
	private logger: Logger;
	private plugins = new Map<string, ManagedPlugin>(); // name -> managed
	private pathIndex = new Map<string, string>(); // absPath -> name
	private watchers: fs.FSWatcher[] = [];
	private debounceTimers = new Map<string, NodeJS.Timeout>();
	private loading = new Set<string>(); // 防并发
	private started = false;

	constructor(opts: PluginManagerOptions) {
		this.opts = opts;
		this.logger = opts.logger ?? console;
	}

	get pluginCount(): number {
		return this.plugins.size;
	}

	list(): PluginMetadata[] {
		return listPlugins();
	}

	get(name: string): ManagedPlugin | undefined {
		return this.plugins.get(name);
	}

	/** 启动：扫描并加载全部插件，可选开启 watch */
	async start(): Promise<void> {
		if (this.started) return;
		this.started = true;
		await this.loadAll();
		if (this.opts.watch !== false && this.opts.pluginDir) {
			this.startWatch();
		}
	}

	async stop(): Promise<void> {
		this.stopWatch();
		const names = [...this.plugins.keys()];
		for (const name of names) {
			await this.unload(name).catch((err) => {
				this.logger.error(`Failed to unload plugin ${name}:`, err);
			});
		}
		this.started = false;
	}

	/** 扫描 pluginDir 并加载所有未加载的插件 */
	async loadAll(): Promise<void> {
		const dir = this.opts.pluginDir;
		if (!dir) {
			this.logger.info("No pluginDir configured; skipping auto-discovery");
			return;
		}
		if (!fs.existsSync(dir)) {
			this.logger.warn(`Plugin directory not found: ${dir}`);
			return;
		}
		const entries = discoverPlugins(dir, resolvePluginEntry);
		for (const absPath of entries) {
			if (this.pathIndex.has(absPath)) continue;
			try {
				await this.loadFromPath(absPath);
			} catch (err) {
				this.logger.error(`Failed to load plugin ${absPath}:`, err);
			}
		}
	}

	/**
	 * 从绝对路径加载插件。
	 * 支持单文件（echo.ts）或目录入口（foo/main.ts / foo/foo.ts）。
	 */
	async loadFromPath(
		absPath: string,
		cacheBust = false,
	): Promise<ManagedPlugin> {
		const resolved = path.resolve(absPath);
		if (this.loading.has(resolved)) {
			throw new Error(`plugin is already loading: ${resolved}`);
		}
		this.loading.add(resolved);
		try {
			const modulePath = pluginModulePath(resolved, this.opts.pluginDir);
			const loaded = await loadPlugin(resolved, modulePath, cacheBust);
			initPlugin(loaded, this.opts.buildContext);

			// 同名已存在 → 先卸旧
			const existing = this.plugins.get(loaded.metadata.name);
			if (existing && existing.absPath !== resolved) {
				await this.unload(existing.name);
			}

			const managed: ManagedPlugin = {
				name: loaded.metadata.name,
				modulePath: loaded.modulePath,
				absPath: resolved,
				metadata: loaded.metadata,
				loaded,
			};
			this.plugins.set(managed.name, managed);
			this.pathIndex.set(resolved, managed.name);
			await this.opts.onLoaded?.(managed.name);
			this.logger.info(
				`Loaded plugin: ${managed.name} (${managed.modulePath}) from ${resolved}`,
			);
			return managed;
		} finally {
			this.loading.delete(resolved);
		}
	}

	/** 按插件名卸载 */
	async unload(name: string): Promise<void> {
		const managed = this.plugins.get(name);
		if (!managed) {
			// 兜底：registry 里可能有残留
			const mp = findModulePathByName(name);
			if (mp) {
				const inst = getPluginInstance(mp);
				if (inst) await unloadPlugin({ instance: inst, modulePath: mp });
			}
			return;
		}
		await unloadPlugin(managed.loaded);
		this.plugins.delete(name);
		this.pathIndex.delete(managed.absPath);
		await this.opts.onUnloaded?.(name);
		this.logger.info(`Unloaded plugin: ${name}`);
	}

	/** 重载：卸载 + 带 cacheBust 重新 import */
	async reload(name?: string): Promise<void> {
		if (!name) {
			// 全量重载
			const snapshot = [...this.plugins.values()];
			for (const p of snapshot) {
				await this.reload(p.name);
			}
			return;
		}
		const managed = this.plugins.get(name);
		if (!managed) {
			this.logger.warn(`Cannot reload unknown plugin: ${name}`);
			return;
		}
		const absPath = managed.absPath;
		this.logger.info(`Reloading plugin: ${name}`);
		await this.unload(name);
		if (!fs.existsSync(absPath)) {
			this.logger.warn(`Plugin file gone after unload: ${absPath}`);
			return;
		}
		await this.loadFromPath(absPath, true);
	}

	/** 启用/禁用（不卸载，只改 activated） */
	setActivated(name: string, activated: boolean): void {
		setPluginActivated(name, activated);
		const managed = this.plugins.get(name);
		if (managed) managed.metadata.activated = activated;
	}

	/**
	 * 安装：把外部文件/目录复制进 pluginDir，再加载。
	 * source 可以是 .ts/.js 文件或包含 main.ts 的目录。
	 */
	async installFromPath(source: string): Promise<ManagedPlugin> {
		const pluginDir = this.opts.pluginDir;
		if (!pluginDir) throw new Error("pluginDir is not configured");
		fs.mkdirSync(pluginDir, { recursive: true });

		const src = path.resolve(source);
		const base = path.basename(src, path.extname(src));
		const dest = fs.statSync(src).isDirectory()
			? path.join(pluginDir, base)
			: path.join(pluginDir, path.basename(src));

		if (fs.existsSync(dest)) {
			// 覆盖安装
			fs.rmSync(dest, { recursive: true, force: true });
		}
		copyRecursive(src, dest);

		const entry = resolvePluginEntry(dest);
		if (!entry) throw new Error(`no plugin entry found under ${dest}`);
		return this.loadFromPath(entry, true);
	}

	// ── watch ──

	private startWatch(): void {
		const dir = this.opts.pluginDir;
		if (!dir || !fs.existsSync(dir)) return;
		try {
			const watcher = fs.watch(dir, { recursive: true }, (_event, filename) => {
				if (!filename) return;
				const abs = path.resolve(dir, filename);
				// 忽略非代码文件
				const ext = path.extname(abs);
				if (ext && !PLUGIN_EXTS.has(ext)) return;
				this.scheduleChange(abs);
			});
			this.watchers.push(watcher);
			this.logger.info(`Watching plugin dir: ${dir}`);
		} catch (err) {
			this.logger.warn(`Failed to watch plugin dir ${dir}:`, err);
		}
	}

	private stopWatch(): void {
		for (const w of this.watchers) {
			try {
				w.close();
			} catch {
				// ignore
			}
		}
		this.watchers = [];
		for (const t of this.debounceTimers.values()) clearTimeout(t);
		this.debounceTimers.clear();
	}

	private scheduleChange(absPath: string): void {
		// 找到受影响的插件根路径
		const target = this.findManagedPathFor(absPath) ?? absPath;
		const ms = this.opts.debounceMs ?? 300;
		const prev = this.debounceTimers.get(target);
		if (prev) clearTimeout(prev);
		this.debounceTimers.set(
			target,
			setTimeout(() => {
				this.debounceTimers.delete(target);
				void this.handleChange(target);
			}, ms),
		);
	}

	private findManagedPathFor(filePath: string): string | null {
		const resolved = path.resolve(filePath);
		if (this.pathIndex.has(resolved)) return resolved;
		// 文件落在某个已加载插件目录下
		for (const [abs, name] of this.pathIndex) {
			const dir = path.dirname(abs);
			if (
				resolved === abs ||
				resolved.startsWith(dir + path.sep) ||
				resolved.startsWith(`${dir}/`)
			) {
				return abs;
			}
			// silence unused
			void name;
		}
		return null;
	}

	private async handleChange(absPath: string): Promise<void> {
		const exists = fs.existsSync(absPath);
		const name = this.pathIndex.get(absPath);

		if (!exists && name) {
			this.logger.info(`Plugin file removed, unloading: ${name}`);
			await this.unload(name);
			return;
		}
		if (!exists) return;

		if (name) {
			this.logger.info(`Plugin file changed, reloading: ${name}`);
			await this.reload(name);
			return;
		}

		// 新文件：尝试加载
		const entry = resolvePluginEntry(absPath);
		if (!entry) return;
		// 可能是目录下的附属文件，检查是否属于已加载插件
		const owned = this.findManagedPathFor(entry);
		if (owned && this.pathIndex.has(owned)) {
			const n = this.pathIndex.get(owned);
			if (!n) return;
			this.logger.info(`Plugin dependency changed, reloading: ${n}`);
			await this.reload(n);
			return;
		}
		try {
			this.logger.info(`New plugin detected: ${entry}`);
			await this.loadFromPath(entry, true);
		} catch (err) {
			this.logger.error(`Hot-load failed for ${entry}:`, err);
		}
	}
}
