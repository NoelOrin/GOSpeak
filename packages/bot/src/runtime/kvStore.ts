import type { KeyValueStore } from "../core/context";

/**
 * In-memory JSON KV. Host keeps one root store and hands plugins
 * either a private namespaced view or the shared root.
 */
export class MemoryKVStore implements KeyValueStore {
	private store: Map<string, string>;

	constructor(store?: Map<string, string>) {
		this.store = store ?? new Map();
	}

	/** Underlying map — shared across namespaced views of the same host. */
	get raw(): Map<string, string> {
		return this.store;
	}

	async get<T = unknown>(key: string): Promise<T | undefined> {
		const v = this.store.get(key);
		return v !== undefined ? (JSON.parse(v) as T) : undefined;
	}

	async set<T = unknown>(key: string, value: T): Promise<void> {
		this.store.set(key, JSON.stringify(value));
	}

	async delete(key: string): Promise<void> {
		this.store.delete(key);
	}

	async has(key: string): Promise<boolean> {
		return this.store.has(key);
	}

	async keys(prefix?: string): Promise<string[]> {
		const all = [...this.store.keys()];
		if (prefix === undefined || prefix === "") return all;
		return all.filter((k) => k.startsWith(prefix));
	}

	async clear(): Promise<void> {
		this.store.clear();
	}
}

/**
 * Namespace wrapper: private plugin keys are stored as `${prefix}${key}`.
 * list/clear only touch keys under this namespace.
 */
export class NamespacedKVStore implements KeyValueStore {
	constructor(
		private readonly root: MemoryKVStore,
		private readonly prefix: string,
	) {}

	private full(key: string): string {
		return `${this.prefix}${key}`;
	}

	async get<T = unknown>(key: string): Promise<T | undefined> {
		return this.root.get<T>(this.full(key));
	}

	async set<T = unknown>(key: string, value: T): Promise<void> {
		await this.root.set(this.full(key), value);
	}

	async delete(key: string): Promise<void> {
		await this.root.delete(this.full(key));
	}

	async has(key: string): Promise<boolean> {
		return this.root.has(this.full(key));
	}

	async keys(prefix?: string): Promise<string[]> {
		const fullPrefix = `${this.prefix}${prefix ?? ""}`;
		const keys = await this.root.keys(fullPrefix);
		return keys.map((k) => k.slice(this.prefix.length));
	}

	async clear(): Promise<void> {
		const keys = await this.root.keys(this.prefix);
		for (const k of keys) await this.root.delete(k);
	}
}

/** Backward-compatible factory used by tests and external callers. */
export function createKVStore(store?: Map<string, string>): KeyValueStore {
	return new MemoryKVStore(store);
}

/** Private namespace for one plugin on a host root store. */
export function createPluginPrivateKV(
	root: MemoryKVStore,
	pluginName: string,
): KeyValueStore {
	return new NamespacedKVStore(root, `plugin:${pluginName}:`);
}

/** Shared namespace (no plugin prefix) on the same host root store. */
export function createSharedKV(root: MemoryKVStore): KeyValueStore {
	// Shared keys live under "shared:" so they never collide with private ones.
	return new NamespacedKVStore(root, "shared:");
}
