import { describe, expect, it } from "vitest";
import {
	isVoiceInteractive,
	isVoiceLoading,
	voicePhaseLabel,
} from "./voiceSessionTypes";

describe("voice session phase helpers", () => {
	it("marks joining phases as loading", () => {
		expect(isVoiceLoading("resolving")).toBe(true);
		expect(isVoiceLoading("loading_sfu")).toBe(true);
		expect(isVoiceLoading("joining_media")).toBe(true);
		expect(isVoiceLoading("joining_signal")).toBe(true);
		expect(isVoiceLoading("media_ready")).toBe(false);
		expect(isVoiceLoading("ready")).toBe(false);
		expect(isVoiceLoading("failed")).toBe(false);
	});

	it("marks ready/media_ready/reconnecting as interactive", () => {
		expect(isVoiceInteractive("ready")).toBe(true);
		expect(isVoiceInteractive("media_ready")).toBe(true);
		expect(isVoiceInteractive("reconnecting")).toBe(true);
		expect(isVoiceInteractive("joining_media")).toBe(false);
		expect(isVoiceInteractive("joining_signal")).toBe(false);
	});

	it("returns stable Chinese labels", () => {
		expect(voicePhaseLabel("loading_sfu")).toBe("加载语音引擎...");
		expect(voicePhaseLabel("joining_media")).toBe("连接媒体...");
		expect(voicePhaseLabel("joining_signal")).toBe("加入房间...");
		expect(voicePhaseLabel("failed")).toBe("加入失败");
	});
});
