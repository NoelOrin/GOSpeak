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

// jsdom 未实现 matchMedia，themeStore 等模块顶层会读取 prefers-color-scheme
if (typeof window !== "undefined" && !window.matchMedia) {
	Object.defineProperty(window, "matchMedia", {
		writable: true,
		value: (query: string): MediaQueryList =>
			({
				matches: false,
				media: query,
				onchange: null,
				addListener: () => {},
				removeListener: () => {},
				addEventListener: () => {},
				removeEventListener: () => {},
				dispatchEvent: () => false,
			}) as unknown as MediaQueryList,
	});
}


// sonner 内部会调用 React.act，需显式声明 act 环境，否则测试输出被警告刷屏
(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT =
	true;
