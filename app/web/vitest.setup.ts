// vitest.setup.ts — node 环境无 localStorage，userStore 等模块顶层读取会崩溃；
// 在测试加载前注入最小内存实现。
class MemoryStorage implements Storage {
	private data = new Map<string, string>();

	get length(): number {
		return this.data.size;
	}
	clear(): void {
		this.data.clear();
	}
	getItem(key: string): string | null {
		return this.data.get(key) ?? null;
	}
	key(index: number): string | null {
		return Array.from(this.data.keys())[index] ?? null;
	}
	removeItem(key: string): void {
		this.data.delete(key);
	}
	setItem(key: string, value: string): void {
		this.data.set(key, String(value));
	}
}

Object.defineProperty(globalThis, "localStorage", {
	value: new MemoryStorage(),
	writable: true,
});
