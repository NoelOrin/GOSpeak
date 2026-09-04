import { describe, expect, it, vi } from "vitest";
import { cleanupAudioHandler, setupAudioHandler } from "./index";

describe("setupAudioHandler", () => {
	it("does not rebind when called twice with same client", () => {
		const client = {
			onRemoteAudioTrack: vi.fn(),
			onRemoteAudioTrackRemoved: vi.fn(),
			onActiveSpeakers: vi.fn(),
			getExistingRemoteAudioTracks: vi.fn(() => []),
		};
		setupAudioHandler(client as any);
		setupAudioHandler(client as any);
		expect(client.onRemoteAudioTrack).toHaveBeenCalledTimes(1);
		cleanupAudioHandler();
	});

	it("rebinds when client instance changes", () => {
		const a = {
			onRemoteAudioTrack: vi.fn(),
			onRemoteAudioTrackRemoved: vi.fn(),
			onActiveSpeakers: vi.fn(),
			getExistingRemoteAudioTracks: vi.fn(() => []),
		};
		const b = {
			onRemoteAudioTrack: vi.fn(),
			onRemoteAudioTrackRemoved: vi.fn(),
			onActiveSpeakers: vi.fn(),
			getExistingRemoteAudioTracks: vi.fn(() => []),
		};
		setupAudioHandler(a as any);
		setupAudioHandler(b as any);
		expect(a.onRemoteAudioTrack).toHaveBeenCalledTimes(1);
		expect(b.onRemoteAudioTrack).toHaveBeenCalledTimes(1);
		cleanupAudioHandler();
	});
});
