import { describe, expect, it } from "vitest";
import {
	getSFUProviderCapabilities,
	SFU_ENFORCEMENT_PROFILES,
} from "./sfuProfiles";

describe("SFU capability tables match backend CapabilitiesFor", () => {
	it("livekit serverMute is enabled", () => {
		expect(getSFUProviderCapabilities("livekit").serverMute).toBe(true);
	});

	it("livekit list capabilities are hard", () => {
		const caps = getSFUProviderCapabilities("livekit");
		expect(caps.listRooms).toBe(true);
		expect(caps.listMembers).toBe(true);
		const details = SFU_ENFORCEMENT_PROFILES.livekit.details;
		const listRooms = details.find((d) => d.key === "listRooms");
		const listMembers = details.find((d) => d.key === "listMembers");
		expect(listRooms?.level).toBe("hard");
		expect(listMembers?.level).toBe("hard");
	});
});
