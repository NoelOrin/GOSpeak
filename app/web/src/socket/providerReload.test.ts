import { afterEach, describe, expect, it, vi } from "vitest";

describe("handleProviderChanged", () => {
	afterEach(() => {
		vi.useRealTimers();
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
	});

	it("toasts, preloads provider client, then reloads after 500ms", async () => {
		vi.useFakeTimers();
		const showToast = vi.fn();
		const preloadSfuClient = vi.fn().mockResolvedValue(undefined);
		const reload = vi.fn();
		vi.stubGlobal("location", { reload });

		const { createProviderReloadHandler } = await import("./providerReload");
		const handle = createProviderReloadHandler({
			showToast,
			preloadSfuClient,
			reload: () => reload(),
			delayMs: 500,
		});

		handle("livekit");
		expect(showToast).toHaveBeenCalled();
		expect(preloadSfuClient).toHaveBeenCalledWith("livekit");
		expect(reload).not.toHaveBeenCalled();

		await vi.advanceTimersByTimeAsync(500);
		expect(reload).toHaveBeenCalledTimes(1);
	});

	it("still reloads when preload fails", async () => {
		vi.useFakeTimers();
		const preloadSfuClient = vi.fn().mockRejectedValue(new Error("boom"));
		const reload = vi.fn();
		const { createProviderReloadHandler } = await import("./providerReload");
		const handle = createProviderReloadHandler({
			showToast: vi.fn(),
			preloadSfuClient,
			reload: () => reload(),
			delayMs: 500,
		});
		handle("livekit");
		await vi.advanceTimersByTimeAsync(500);
		expect(reload).toHaveBeenCalledTimes(1);
	});

	it("prefers in-place rejoin over reload when rejoin is available", async () => {
		vi.useFakeTimers();
		const showToast = vi.fn();
		const preloadSfuClient = vi.fn().mockResolvedValue(undefined);
		const attemptRejoin = vi.fn();
		const reload = vi.fn();

		const { createProviderReloadHandler } = await import("./providerReload");
		const handle = createProviderReloadHandler({
			showToast,
			preloadSfuClient,
			canRejoin: () => true,
			attemptRejoin,
			isConnected: () => true,
			reload: () => reload(),
			delayMs: 500,
		});

		handle("livekit");
		expect(attemptRejoin).toHaveBeenCalledTimes(1);
		expect(reload).not.toHaveBeenCalled();

		// rejoin 成功路径：超过 rejoin 兜底窗口也不刷新
		await vi.advanceTimersByTimeAsync(30000);
		expect(reload).not.toHaveBeenCalled();
	});

	it("falls back to reload when rejoin does not restore the connection", async () => {
		vi.useFakeTimers();
		const attemptRejoin = vi.fn();
		const reload = vi.fn();

		const { createProviderReloadHandler } = await import("./providerReload");
		const handle = createProviderReloadHandler({
			showToast: vi.fn(),
			preloadSfuClient: vi.fn().mockResolvedValue(undefined),
			canRejoin: () => true,
			attemptRejoin,
			isConnected: () => false,
			reload: () => reload(),
			rejoinTimeoutMs: 10000,
		});

		handle("livekit");
		expect(attemptRejoin).toHaveBeenCalledTimes(1);
		expect(reload).not.toHaveBeenCalled();

		await vi.advanceTimersByTimeAsync(10000);
		expect(reload).toHaveBeenCalledTimes(1);
	});

	it("does not preload a provider outside the runtime whitelist", async () => {
		vi.useFakeTimers();
		const preloadSfuClient = vi.fn().mockResolvedValue(undefined);
		const reload = vi.fn();

		const { createProviderReloadHandler } = await import("./providerReload");
		const handle = createProviderReloadHandler({
			showToast: vi.fn(),
			preloadSfuClient,
			reload: () => reload(),
			delayMs: 500,
		});

		handle("not-a-provider");
		expect(preloadSfuClient).not.toHaveBeenCalled();
		await vi.advanceTimersByTimeAsync(500);
		expect(reload).toHaveBeenCalledTimes(1);
	});
});
