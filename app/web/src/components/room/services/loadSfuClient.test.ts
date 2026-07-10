import { beforeEach, describe, expect, it, vi } from "vitest";

const preloadSFUClient = vi.fn(async () => {});
const createSFUClient = vi.fn(async () => ({ id: "c1" }));

vi.mock("@gospeak/sfu-client/client", () => ({
	preloadSFUClient,
	createSFUClient,
}));

describe("loadSfuClient", () => {
	beforeEach(() => {
		preloadSFUClient.mockClear();
		createSFUClient.mockClear();
		const store = new Map<string, string>();
		vi.stubGlobal("localStorage", {
			getItem: (key: string) => store.get(key) ?? null,
			setItem: (key: string, value: string) => {
				store.set(key, value);
			},
			removeItem: (key: string) => {
				store.delete(key);
			},
			clear: () => {
				store.clear();
			},
		});
		vi.resetModules();
	});

	it("preloads provider before create", async () => {
		const mod = await import("./loadSfuClient");
		const callOrder: string[] = [];
		preloadSFUClient.mockImplementation(async () => {
			callOrder.push("preload");
		});
		createSFUClient.mockImplementation(async () => {
			callOrder.push("create");
			return { id: "c1" };
		});

		await mod.loadSfuClient("srs");

		expect(preloadSFUClient).toHaveBeenCalledWith("srs");
		expect(createSFUClient).toHaveBeenCalledWith("srs", undefined);
		expect(callOrder).toEqual(["preload", "create"]);
	});
});
