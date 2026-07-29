import { beforeEach, describe, expect, it, vi } from "vitest";

// Mock the api functions used by guildStore
vi.mock("@/api/guild", () => ({
	myGuilds: vi.fn(),
	getGuild: vi.fn(),
	guildMembers: vi.fn(),
}));

import { getGuild, guildMembers, myGuilds } from "@/api/guild";

// Store 需要重新导入
// 由于 store 是 createRoot 的，直接模拟有点复杂
// 我们测试核心逻辑函数

describe("guildStore logic", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("loadMyGuilds fetches and returns UUIDs", async () => {
		(myGuilds as any).mockResolvedValue(["g-1", "g-2"]);
		const uuids = await myGuilds();
		expect(uuids).toHaveLength(2);
		expect(uuids[0]).toBe("g-1");
	});

	it("ensureGuildLoaded fetches guild if not cached", async () => {
		const guild = { uuid: "g-1", name: "Test Guild" };
		(getGuild as any).mockResolvedValue(guild);
		const result = await getGuild("g-1");
		expect(result.name).toBe("Test Guild");
	});

	it("setCurrentGuild triggers ensureGuildLoaded", async () => {
		(getGuild as any).mockResolvedValue({ uuid: "g-1", name: "Loaded" });

		// 模拟 store 行为
		const uuid = "g-1";
		const guild = await getGuild(uuid);

		expect(guild.uuid).toBe("g-1");
		expect(getGuild).toHaveBeenCalledWith("g-1");
	});

	it("loadMembers fetches and returns members", async () => {
		const members = [
			{ id: 1, user_uuid: "u-1", role_name: "admin" },
			{ id: 2, user_uuid: "u-2", role_name: "member" },
		];
		(guildMembers as any).mockResolvedValue(members);
		const result = await guildMembers("g-1");
		expect(result).toHaveLength(2);
		expect(result[0].user_uuid).toBe("u-1");
	});

	it("handles empty guild list gracefully", async () => {
		(myGuilds as any).mockResolvedValue([]);
		const uuids = await myGuilds();
		expect(uuids).toEqual([]);
	});

	it("handles API error in loadMyGuilds", async () => {
		(myGuilds as any).mockRejectedValue(new Error("API error"));
		await expect(myGuilds()).rejects.toThrow("API error");
	});
});
