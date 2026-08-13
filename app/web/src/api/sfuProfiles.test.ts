import { describe, expect, it } from "vitest";
import {
	getSFUProviderCapabilities,
	SFU_ENFORCEMENT_PROFILES,
} from "./sfuProfiles";

describe("SFU capability tables match backend CapabilitiesFor", () => {
	it("cloudflare serverMute is enabled (backend: ServerMute=true, degraded)", () => {
		expect(getSFUProviderCapabilities("cloudflare").serverMute).toBe(true);
	});

	it("srs list capabilities are hard (backend: SRS API direct, ListLevel=hard)", () => {
		const caps = getSFUProviderCapabilities("srs");
		expect(caps.listRooms).toBe(true);
		expect(caps.listMembers).toBe(true);
		const details = SFU_ENFORCEMENT_PROFILES.srs.details;
		const listRooms = details.find((d) => d.key === "listRooms");
		const listMembers = details.find((d) => d.key === "listMembers");
		expect(listRooms?.level).toBe("hard");
		expect(listMembers?.level).toBe("hard");
	});

	it("srs serverMute impl describes publish-block semantics, not kick", () => {
		const detail = SFU_ENFORCEMENT_PROFILES.srs.details.find(
			(d) => d.key === "serverMute",
		);
		expect(detail?.impl).toContain("禁推");
	});
});
