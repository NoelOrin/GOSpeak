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
		handle("srs");
		await vi.advanceTimersByTimeAsync(500);
		expect(reload).toHaveBeenCalledTimes(1);
	});
});
