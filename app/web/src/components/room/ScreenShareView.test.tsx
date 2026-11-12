import type { RemoteVideoTrackLike } from "@gospeak/sfu-client/types";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import ScreenShareView from "./ScreenShareView";

function makeTrack() {
	const attach = vi.fn();
	const detach = vi.fn(() => []);
	const track: RemoteVideoTrackLike = { attach, detach };
	return { track, attach, detach };
}

function stubIntrinsicSize(
	video: HTMLVideoElement,
	width: number,
	height: number,
) {
	Object.defineProperty(video, "videoWidth", {
		configurable: true,
		value: width,
	});
	Object.defineProperty(video, "videoHeight", {
		configurable: true,
		value: height,
	});
}

describe("ScreenShareView", () => {
	beforeEach(() => {
		// jsdom 不实现媒体播放，返回 undefined 而不是 Promise
		vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue(undefined);
	});

	afterEach(() => {
		cleanup();
		vi.restoreAllMocks();
	});

	it("把 track 附到 video 上，卸载时 detach", () => {
		const { track, attach, detach } = makeTrack();
		const { container, unmount } = render(<ScreenShareView track={track} />);

		const video = container.querySelector("video");
		expect(attach).toHaveBeenCalledWith(video);

		unmount();
		expect(detach).toHaveBeenCalledWith(video);
	});

	it("forwardRef 暴露的是 video 元素本身", () => {
		const { track } = makeTrack();
		const ref = createRef<HTMLVideoElement>();
		const { container } = render(<ScreenShareView ref={ref} track={track} />);

		expect(ref.current).toBe(container.querySelector("video"));
		expect(ref.current?.tagName).toBe("VIDEO");
	});

	it("没有 onToggleFullscreen 时不渲染全屏按钮", () => {
		const { track } = makeTrack();
		render(<ScreenShareView track={track} />);

		expect(screen.queryByRole("button")).toBeNull();
	});

	it("全屏按钮浮在画面容器右下角，且不触发画面点击", () => {
		const { track } = makeTrack();
		const onClick = vi.fn();
		const onToggleFullscreen = vi.fn();
		render(
			<ScreenShareView
				track={track}
				onClick={onClick}
				onToggleFullscreen={onToggleFullscreen}
			/>,
		);

		const button = screen.getByRole("button", { name: "全屏" });
		expect(button.className).toContain("absolute");
		expect(button.className).toContain("right-3");
		expect(button.className).toContain("bottom-3");

		fireEvent.click(button);
		expect(onToggleFullscreen).toHaveBeenCalledTimes(1);
		expect(onClick).not.toHaveBeenCalled();
	});

	it("点击画面本身触发 onClick", () => {
		const { track } = makeTrack();
		const onClick = vi.fn();
		render(<ScreenShareView track={track} onClick={onClick} />);

		const video = document.querySelector("video") as HTMLVideoElement;
		fireEvent.click(video);
		expect(onClick).toHaveBeenCalledTimes(1);
	});

	it("fullscreen 状态切换按钮图标与文案", () => {
		const { track } = makeTrack();
		const { rerender } = render(
			<ScreenShareView track={track} onToggleFullscreen={() => {}} />,
		);
		expect(screen.getByRole("button", { name: "全屏" })).toBeTruthy();

		rerender(
			<ScreenShareView
				track={track}
				fullscreen
				onToggleFullscreen={() => {}}
			/>,
		);
		expect(screen.getByRole("button", { name: "退出全屏" })).toBeTruthy();
	});

	it("按视频真实分辨率给容器设宽高比，避免按钮落在黑边上", () => {
		const { track } = makeTrack();
		const { container } = render(<ScreenShareView track={track} />);
		const wrapper = container.firstElementChild as HTMLElement;
		const video = container.querySelector("video") as HTMLVideoElement;

		expect(wrapper.style.aspectRatio.replace(/\s/g, "")).toBe("16/9");

		stubIntrinsicSize(video, 1600, 1200);
		fireEvent.loadedMetadata(video);
		expect(wrapper.style.aspectRatio.replace(/\s/g, "")).toBe("1600/1200");
	});
});
