import type { AxiosResponse } from "axios";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Result } from "@/api/apiClient";

// Mock apiClient 模块
vi.mock("@/api/apiClient", () => {
	const mockPost = vi.fn();
	return {
		default: {
			post: mockPost,
		},
		mockPost,
	};
});

import apiClient from "@/api/apiClient";
import {
	createGuild,
	deleteGuild,
	getGuild,
	guildMembers,
	joinGuild,
	kickGuildMember,
	leaveGuild,
	listGuilds,
	listPublicGuilds,
	myGuilds,
	updateGuild,
} from "@/api/guild";

function mockResult<T>(data: T): AxiosResponse<Result<T>> {
	return {
		data: { code: 0, msg: "success", data },
		status: 200,
		statusText: "OK",
		headers: {},
		config: {} as any,
	};
}

describe("guildApi", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("createGuild calls correct endpoint", async () => {
		const guild = { uuid: "g-1", name: "Test" } as any;
		(apiClient.post as any).mockResolvedValue(mockResult(guild));
		const result = await createGuild({ name: "Test", is_public: true });
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/create",
			data: { name: "Test", is_public: true },
		});
		expect(result.uuid).toBe("g-1");
	});

	it("getGuild calls correct endpoint", async () => {
		const guild = { uuid: "g-1", name: "Test" } as any;
		(apiClient.post as any).mockResolvedValue(mockResult(guild));
		const result = await getGuild("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/get",
			data: { uuid: "g-1" },
		});
		expect(result.name).toBe("Test");
	});

	it("listGuilds builds query params", async () => {
		const data = { guilds: [], total: 0 };
		(apiClient.post as any).mockResolvedValue(mockResult(data));
		await listGuilds(2, 10);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/list",
			data: { page: 2, page_size: 10 },
		});
	});

	it("listPublicGuilds builds query params", async () => {
		const data = { guilds: [], total: 0 };
		(apiClient.post as any).mockResolvedValue(mockResult(data));
		await listPublicGuilds(1, 20);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/list-public",
			data: { page: 1, page_size: 20 },
		});
	});

	it("myGuilds returns UUIDs from response", async () => {
		(apiClient.post as any).mockResolvedValue(
			mockResult({ guild_uuids: ["g-1", "g-2"] }),
		);
		const uuids = await myGuilds();
		expect(uuids).toEqual(["g-1", "g-2"]);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/my-guilds",
			data: {},
		});
	});

	it("joinGuild calls with invite_code", async () => {
		const guild = { uuid: "g-1", name: "Joined" } as any;
		(apiClient.post as any).mockResolvedValue(mockResult(guild));
		const result = await joinGuild("CODE123");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/join",
			data: { invite_code: "CODE123" },
		});
		expect(result.name).toBe("Joined");
	});

	it("leaveGuild calls with uuid", async () => {
		(apiClient.post as any).mockResolvedValue(mockResult(null));
		await leaveGuild("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/leave",
			data: { uuid: "g-1" },
		});
	});

	it("updateGuild sends only provided fields", async () => {
		const guild = { uuid: "g-1", name: "Updated" } as any;
		(apiClient.post as any).mockResolvedValue(mockResult(guild));
		const result = await updateGuild({ uuid: "g-1", name: "Updated" });
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/update",
			data: { uuid: "g-1", name: "Updated" },
		});
		expect(result.name).toBe("Updated");
	});

	it("deleteGuild calls with uuid", async () => {
		(apiClient.post as any).mockResolvedValue(mockResult(null));
		await deleteGuild("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/delete",
			data: { uuid: "g-1" },
		});
	});

	it("kickGuildMember calls with guild_uuid and user_uuid", async () => {
		(apiClient.post as any).mockResolvedValue(mockResult(null));
		await kickGuildMember("g-1", "u-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/kick",
			data: { guild_uuid: "g-1", user_uuid: "u-1" },
		});
	});

	it("guildMembers returns member list", async () => {
		const members = [{ id: 1, user_uuid: "u-1", role_name: "member" }];
		(apiClient.post as any).mockResolvedValue(mockResult({ members }));
		const result = await guildMembers("g-1");
		expect(result).toEqual(members);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/guild/members",
			data: { guild_uuid: "g-1" },
		});
	});
});
