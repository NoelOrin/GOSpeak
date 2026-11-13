import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * 覆盖层接力的启动期行为。
 *
 * 重点回归两件事：
 *   1. 两条启动 IPC（getConfig / isVisible）失败时不能产生 unhandled rejection；
 *   2. effect 只在挂载时跑一次 —— 之前依赖 [location.pathname]，每次导航都会
 *      重建 relay（重置 lastKey、重跑启动 IPC、重复订阅 store）。
 */

const overlayMock = vi.hoisted(() => {
	const unsub = () => {};
	const enabledHandlers: ((enabled: boolean) => void)[] = [];
	const visibilityHandlers: ((visible: boolean) => void)[] = [];
	return {
		unsub,
		enabledHandlers,
		visibilityHandlers,
		instance: {
			getConfig: vi.fn(),
			isVisible: vi.fn(),
			pushState: vi.fn(),
			onEnabledChange: vi.fn((handler: (enabled: boolean) => void) => {
				enabledHandlers.push(handler);
				return unsub;
			}),
			onVisibilityChange: vi.fn((handler: (visible: boolean) => void) => {
				visibilityHandlers.push(handler);
				return unsub;
			}),
		},
	};
});

vi.mock("@gospeak/desktop", () => ({
	inDesktop: () => true,
	createOverlay: () => overlayMock.instance,
}));

import { useOverlayRelay } from "./useOverlayRelay";

afterEach(() => {
	vi.clearAllMocks();
});

function resetOverlay() {
	overlayMock.enabledHandlers.length = 0;
	overlayMock.visibilityHandlers.length = 0;
	overlayMock.instance.getConfig.mockReset();
	overlayMock.instance.isVisible.mockReset();
	overlayMock.instance.pushState.mockReset();
	overlayMock.instance.onEnabledChange.mockClear();
	overlayMock.instance.onVisibilityChange.mockClear();
}

describe("useOverlayRelay", () => {
	it("启动 IPC 失败时不产生未处理的 rejection，也不推送", async () => {
		resetOverlay();
		overlayMock.instance.getConfig.mockRejectedValue(new Error("bridge down"));
		overlayMock.instance.isVisible.mockRejectedValue(new Error("bridge down"));

		const rejections: unknown[] = [];
		const onRejection = (reason: unknown) => rejections.push(reason);
		process.on("unhandledRejection", onRejection);

		try {
			const { unmount } = renderHook(() => useOverlayRelay());
			// 让两条被 catch 的 promise 走完。
			await Promise.resolve();
			await Promise.resolve();
			await new Promise((resolve) => setTimeout(resolve, 0));

			expect(overlayMock.instance.getConfig).toHaveBeenCalledTimes(1);
			expect(overlayMock.instance.isVisible).toHaveBeenCalledTimes(1);
			expect(rejections).toEqual([]);
			expect(overlayMock.instance.pushState).not.toHaveBeenCalled();
			unmount();
		} finally {
			process.off("unhandledRejection", onRejection);
		}
	});

	it("挂载一次即订阅，重渲染不会重复订阅或重跑启动 IPC", () => {
		resetOverlay();
		overlayMock.instance.getConfig.mockResolvedValue({ enabled: false });
		overlayMock.instance.isVisible.mockResolvedValue(false);

		const { rerender, unmount } = renderHook(() => useOverlayRelay());
		expect(overlayMock.instance.onEnabledChange).toHaveBeenCalledTimes(1);
		expect(overlayMock.instance.onVisibilityChange).toHaveBeenCalledTimes(1);
		expect(overlayMock.instance.getConfig).toHaveBeenCalledTimes(1);

		rerender();
		rerender();

		expect(overlayMock.instance.onEnabledChange).toHaveBeenCalledTimes(1);
		expect(overlayMock.instance.onVisibilityChange).toHaveBeenCalledTimes(1);
		expect(overlayMock.instance.getConfig).toHaveBeenCalledTimes(1);
		expect(overlayMock.instance.isVisible).toHaveBeenCalledTimes(1);

		unmount();
	});

	it("开关关闭时不推送；开启后立即补推一次", async () => {
		resetOverlay();
		overlayMock.instance.getConfig.mockResolvedValue({ enabled: false });
		overlayMock.instance.isVisible.mockResolvedValue(false);

		const { unmount } = renderHook(() => useOverlayRelay());
		await Promise.resolve();
		expect(overlayMock.instance.pushState).not.toHaveBeenCalled();

		const handler = overlayMock.enabledHandlers[0];
		handler(true);

		expect(overlayMock.instance.pushState).toHaveBeenCalledTimes(1);
		const snapshot = overlayMock.instance.pushState.mock.calls[0][0];
		expect(snapshot).toEqual({
			roomName: null,
			members: [],
			speakingIdentities: [],
			connected: false,
		});
		unmount();
	});
});
