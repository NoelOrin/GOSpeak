import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RemoteAudioTrackLike } from "@gospeak/sfu-client/types";
import { cleanupAudioHandler, setupAudioHandler } from "./index";

function makeClient(track: RemoteAudioTrackLike) {
	return {
		onRemoteAudioTrack: (
			cb: (info: { identity: string; track: RemoteAudioTrackLike }) => void,
		) => {
			cb({ identity: "alice", track });
		},
		onRemoteAudioTrackRemoved: vi.fn(),
		onActiveSpeakers: vi.fn(),
		getExistingRemoteAudioTracks: () => [],
	};
}

describe("setServerMutedByIdentity", () => {
	let mod: typeof import("./index");
	let track: RemoteAudioTrackLike & { setVolume: ReturnType<typeof vi.fn> };

	beforeEach(async () => {
		mod = await import("./index");
		cleanupAudioHandler();
		document.body.innerHTML = "";
		track = {
			attach: vi.fn(() => document.createElement("audio")),
			detach: vi.fn(),
			setVolume: vi.fn(),
		} as unknown as RemoteAudioTrackLike & {
			setVolume: ReturnType<typeof vi.fn>;
		};
		setupAudioHandler(makeClient(track) as any);
	});

	it("server mute zeroes volume and unmute restores it", () => {
		mod.setServerMutedByIdentity("alice", true);
		expect(track.setVolume).toHaveBeenLastCalledWith(0);
		mod.setServerMutedByIdentity("alice", false);
		expect(track.setVolume).toHaveBeenLastCalledWith(1);
	});

	it("server mute is independent from personal volume", () => {
		mod.setVolumeByIdentity("alice", 0.5);
		mod.setServerMutedByIdentity("alice", true);
		expect(track.setVolume).toHaveBeenLastCalledWith(0);
		mod.setServerMutedByIdentity("alice", false);
		expect(track.setVolume).toHaveBeenLastCalledWith(0.5);
	});

	it("unmuting server mute does not clear personal mute", () => {
		mod.setMutedByIdentity("alice", true);
		mod.setServerMutedByIdentity("alice", false);
		expect(track.setVolume).toHaveBeenLastCalledWith(0);
		mod.setMutedByIdentity("alice", false);
		expect(track.setVolume).toHaveBeenLastCalledWith(1);
	});

	it("clears server mute state when remote track unsubscribed", () => {
		const client = makeClient(track);
		setupAudioHandler(client as any);
		mod.setServerMutedByIdentity("alice", true);
		expect(mod.getServerMutedIdentities().has("alice")).toBe(true);

		const removeCb = vi.mocked(client.onRemoteAudioTrackRemoved).mock
			.calls[0][0] as (identity: string) => void;
		removeCb("alice");

		expect(mod.getServerMutedIdentities().has("alice")).toBe(false);
	});
});
