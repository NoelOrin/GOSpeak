import { afterEach, describe, expect, it } from "vitest";
import { SERVER_BASE_URL_KEY, createServerConfig } from "./server";

/**
 * 双实现的两条路径都要能被测到：
 *   - 浏览器：localStorage 就是全部；
 *   - 桌面端：宿主是权威，localStorage 是同步镜像（并在漂移时被修复）。
 *
 * node 测试环境没有 localStorage / mygo，这里手工装一个最小实现。
 */
class MemoryStorage {
	private readonly items = new Map<string, string>();

	getItem(key: string): string | null {
		return this.items.get(key) ?? null;
	}
	setItem(key: string, value: string): void {
		this.items.set(key, String(value));
	}
	removeItem(key: string): void {
		this.items.delete(key);
	}
	clear(): void {
		this.items.clear();
	}
	get length(): number {
		return this.items.size;
	}
	key(index: number): string | null {
		return [...this.items.keys()][index] ?? null;
	}
}

function installStorage(): MemoryStorage {
	const storage = new MemoryStorage();
	(globalThis as unknown as { localStorage: MemoryStorage }).localStorage =
		storage;
	return storage;
}

/** 装一个假宿主：GetBaseURL 读 value，SetBaseURL 交给 onSet。 */
function installHost(
	value: () => string,
	onSet: (url: string) => void | Promise<void> = (url) => {
		currentHostValue = url;
	},
): void {
	hostCalls = 0;
	(globalThis as unknown as { mygo: unknown }).mygo = {
		isMyGo: () => true,
		call: async (method: string, ...args: unknown[]) => {
			hostCalls += 1;
			if (method === "ServerConfig.GetBaseURL") return value();
			if (method === "ServerConfig.SetBaseURL") {
				await onSet(String(args[0] ?? ""));
				return null;
			}
			throw new Error(`not bound: ${method}`);
		},
		on: () => () => {},
	};
}

let currentHostValue = "";
let hostCalls = 0;

function clearGlobals(): void {
	delete (globalThis as unknown as { mygo?: unknown }).mygo;
	delete (globalThis as unknown as { localStorage?: unknown }).localStorage;
	currentHostValue = "";
	hostCalls = 0;
}

afterEach(clearGlobals);

describe("server config / 浏览器实现（localStorage）", () => {
	it("写入后同步/异步都能读回，空串表示清除", async () => {
		const storage = installStorage();
		const config = createServerConfig();

		expect(config.getBaseUrlSync()).toBe("");
		expect(await config.getBaseUrl()).toBe("");

		await config.setBaseUrl("  https://voice.example.com  ");
		expect(storage.getItem(SERVER_BASE_URL_KEY)).toBe(
			"https://voice.example.com",
		);
		expect(config.getBaseUrlSync()).toBe("https://voice.example.com");
		expect(await config.getBaseUrl()).toBe("https://voice.example.com");

		await config.setBaseUrl("");
		expect(storage.getItem(SERVER_BASE_URL_KEY)).toBeNull();
		expect(config.getBaseUrlSync()).toBe("");
	});

	it("localStorage 不可用时安全降级（隐私模式 / SSR）", async () => {
		// 不装 storage：readMirror/writeMirror 必须吞掉异常而不是抛。
		const config = createServerConfig();
		expect(config.getBaseUrlSync()).toBe("");
		await expect(
			config.setBaseUrl("https://a.example"),
		).resolves.toBeUndefined();
		expect(await config.getBaseUrl()).toBe("");
	});
});

describe("server config / 桌面端实现（宿主权威 + 本地镜像）", () => {
	it("宿主值优先，并顺手修复漂移的镜像", async () => {
		const storage = installStorage();
		storage.setItem(SERVER_BASE_URL_KEY, "https://stale.example");
		currentHostValue = "https://host.example";
		installHost(() => currentHostValue);

		const config = createServerConfig();
		// 首帧读到的还是（可能过期的）镜像，不阻塞渲染。
		expect(config.getBaseUrlSync()).toBe("https://stale.example");

		expect(await config.getBaseUrl()).toBe("https://host.example");
		// 权威值一到，镜像被修好。
		expect(storage.getItem(SERVER_BASE_URL_KEY)).toBe("https://host.example");
		expect(config.getBaseUrlSync()).toBe("https://host.example");
	});

	it("写入先落镜像再调宿主（宿主会重载页面，回调可能永不返回）", async () => {
		const storage = installStorage();
		currentHostValue = "";
		let mirrorAtHostCall = "";
		installHost(
			() => currentHostValue,
			(url) => {
				// 宿主处理 SetBaseURL 时读取镜像：必须已经是新值。
				mirrorAtHostCall = storage.getItem(SERVER_BASE_URL_KEY) ?? "";
				currentHostValue = url;
			},
		);

		await createServerConfig().setBaseUrl("https://next.example");
		expect(mirrorAtHostCall).toBe("https://next.example");
		expect(currentHostValue).toBe("https://next.example");
	});

	it("宿主拒绝写入时回滚镜像，避免显示一个没生效的地址", async () => {
		const storage = installStorage();
		storage.setItem(SERVER_BASE_URL_KEY, "https://old.example");
		installHost(
			() => "https://old.example",
			() => {
				throw new Error("invalid url");
			},
		);

		const config = createServerConfig();
		await expect(config.setBaseUrl("https://bad.example")).rejects.toThrow(
			"invalid url",
		);
		expect(storage.getItem(SERVER_BASE_URL_KEY)).toBe("https://old.example");
	});
});

/**
 * 非法 URL 必须在写镜像、调 bridge 之前就被拦下。
 *
 * 旧宿主的 `ServerConfig.SetBaseURL` 没有 error 返回值，MyGo 对这类绑定恒
 * `return nil, nil`，promise 永不 reject —— 单靠「失败回滚」是死代码，非法值
 * 会被原样持久化。这里断言的是 TS 侧的预校验：既不写镜像，也不发 IPC。
 */
describe("server config / 非法 URL 预校验", () => {
	const invalid: [string, string][] = [
		["相对路径", "/api/v1"],
		["缺 scheme", "example.com:8080"],
		["裸 host", "example.com"],
		["非法字符", "http://exa mple.com"],
		["ftp scheme", "ftp://files.example.com"],
		["javascript scheme", "javascript:alert(1)"],
		["file scheme", "file:///etc/passwd"],
		["data scheme", "data:text/plain,hi"],
	];

	for (const [name, url] of invalid) {
		it(`桌面端拒绝 ${name}（${url}），不写镜像也不调宿主`, async () => {
			const storage = installStorage();
			storage.setItem(SERVER_BASE_URL_KEY, "https://old.example");
			installHost(() => "https://old.example");

			const config = createServerConfig();
			await expect(config.setBaseUrl(url)).rejects.toThrow();
			expect(storage.getItem(SERVER_BASE_URL_KEY)).toBe("https://old.example");
			expect(config.getBaseUrlSync()).toBe("https://old.example");
			expect(hostCalls).toBe(0);
		});

		it(`浏览器端拒绝 ${name}（${url}），不写镜像`, async () => {
			const storage = installStorage();
			storage.setItem(SERVER_BASE_URL_KEY, "https://old.example");

			const config = createServerConfig();
			await expect(config.setBaseUrl(url)).rejects.toThrow();
			expect(storage.getItem(SERVER_BASE_URL_KEY)).toBe("https://old.example");
			expect(config.getBaseUrlSync()).toBe("https://old.example");
		});
	}

	it("合法 http / https 仍然写入（含首尾空白裁剪）", async () => {
		const storage = installStorage();
		const config = createServerConfig();

		await config.setBaseUrl("  http://plain.example.com:8080  ");
		expect(storage.getItem(SERVER_BASE_URL_KEY)).toBe(
			"http://plain.example.com:8080",
		);

		await config.setBaseUrl("https://secure.example.com/path?x=1");
		expect(storage.getItem(SERVER_BASE_URL_KEY)).toBe(
			"https://secure.example.com/path?x=1",
		);
	});

	it("空串表示清除，不受校验影响", async () => {
		const storage = installStorage();
		const config = createServerConfig();

		await config.setBaseUrl("https://a.example");
		expect(storage.getItem(SERVER_BASE_URL_KEY)).toBe("https://a.example");

		await config.setBaseUrl("");
		expect(storage.getItem(SERVER_BASE_URL_KEY)).toBeNull();
		expect(config.getBaseUrlSync()).toBe("");
	});
});
