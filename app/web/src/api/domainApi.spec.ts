import { beforeEach, describe, expect, it, vi } from "vitest";

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
	createDomain,
	createDomainRole,
	deleteDomain,
	getDomain,
	domainMembers,
	joinDomain,
	kickDomainMember,
	leaveDomain,
	listDomainRoles,
	listDomains,
	listPublicDomains,
	myDomainPermissions,
	myDomains,
	getMyDomainsDetailed,
	previewDomainInvite,
	updateDomain,
	updateDomainMemberRole,
} from "@/api/domain";

describe("domainApi", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("createDomain calls correct endpoint", async () => {
		const domain = { uuid: "g-1", name: "Test" } as any;
		(apiClient.post as any).mockResolvedValue(domain);
		const result = await createDomain({ name: "Test", is_public: true });
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/create",
			data: { name: "Test", is_public: true },
		});
		expect(result.uuid).toBe("g-1");
	});

	it("getDomain calls correct endpoint", async () => {
		const domain = { uuid: "g-1", name: "Test" } as any;
		(apiClient.post as any).mockResolvedValue(domain);
		const result = await getDomain("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/get",
			data: { domain_uuid: "g-1" },
		});
		expect(result.name).toBe("Test");
	});

	it("listDomains builds query params", async () => {
		const data = { domains: [], total: 0 };
		(apiClient.post as any).mockResolvedValue(data);
		await listDomains(2, 10);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/list",
			data: { page: 2, page_size: 10 },
		});
	});

	it("listPublicDomains builds query params", async () => {
		const data = { domains: [], total: 0 };
		(apiClient.post as any).mockResolvedValue(data);
		await listPublicDomains(1, 20);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/list-public",
			data: { page: 1, page_size: 20 },
		});
	});

	it("listPublicDomains sends keyword when provided", async () => {
		const data = { domains: [], total: 0 };
		(apiClient.post as any).mockResolvedValue(data);
		await listPublicDomains(2, 10, "Alpha");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/list-public",
			data: { page: 2, page_size: 10, keyword: "Alpha" },
		});
	});

	it("listPublicDomains trims keyword and preserves pagination", async () => {
		(apiClient.post as any).mockResolvedValue({ domains: [], total: 0 });

		await listPublicDomains(2, 12, "  alpha  ");

		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/list-public",
			data: { page: 2, page_size: 12, keyword: "alpha" },
		});
	});

	it("getMyDomainsDetailed returns batch domain details", async () => {
		const details = [
			{ uuid: "g-1", name: "G1", member_count: 3, room_count: 2 },
			{ uuid: "g-2", name: "G2", member_count: 2, room_count: 1 },
		];
		(apiClient.post as any).mockResolvedValue(details);
		const result = await getMyDomainsDetailed();
		expect(result).toEqual(details);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/my-domains",
			data: {},
		});
	});

	it("myDomains returns UUIDs from batch details", async () => {
		(apiClient.post as any).mockResolvedValue([
			{ uuid: "g-1", name: "G1", member_count: 1, room_count: 0 },
			{ uuid: "g-2", name: "G2", member_count: 1, room_count: 1 },
		]);
		const uuids = await myDomains();
		expect(uuids).toEqual(["g-1", "g-2"]);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/my-domains",
			data: {},
		});
	});

	it("previewDomainInvite sends the normalized invite code", async () => {
		const domain = { uuid: "g-1", name: "Preview" } as any;
		(apiClient.post as any).mockResolvedValue(domain);
		const result = await previewDomainInvite("ABCDEFGH");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/preview",
			data: { invite_code: "ABCDEFGH" },
		});
		expect(result.name).toBe("Preview");
	});

	it("joinDomain calls with invite_code", async () => {
		const domain = { uuid: "g-1", name: "Joined" } as any;
		(apiClient.post as any).mockResolvedValue(domain);
		const result = await joinDomain("CODE123");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/join",
			data: { invite_code: "CODE123" },
		});
		expect(result.name).toBe("Joined");
	});

	it("leaveDomain calls with domain_uuid", async () => {
		(apiClient.post as any).mockResolvedValue(null);
		await leaveDomain("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/leave",
			data: { domain_uuid: "g-1" },
		});
	});

	it("updateDomain sends only provided fields", async () => {
		const domain = { uuid: "g-1", name: "Updated" } as any;
		(apiClient.post as any).mockResolvedValue(domain);
		const result = await updateDomain({
			domain_uuid: "g-1",
			name: "Updated",
		});
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/update",
			data: { domain_uuid: "g-1", name: "Updated" },
		});
		expect(result.name).toBe("Updated");
	});

	it("deleteDomain calls with domain_uuid", async () => {
		(apiClient.post as any).mockResolvedValue(null);
		await deleteDomain("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/delete",
			data: { domain_uuid: "g-1" },
		});
	});

	it("kickDomainMember calls with domain_uuid and user_uuid", async () => {
		(apiClient.post as any).mockResolvedValue(null);
		await kickDomainMember("g-1", "u-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/kick",
			data: { domain_uuid: "g-1", user_uuid: "u-1" },
		});
	});

	it("domainMembers returns member list", async () => {
		const members = [
			{
				id: 1,
				user_uuid: "u-1",
				nickname: "",
				role_name: "member",
				joined_at: "2026-01-01",
				name: "alice",
				display_name: "Alice",
			},
		];
		(apiClient.post as any).mockResolvedValue({ members });
		const result = await domainMembers("g-1");
		expect(result).toEqual(members);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/members",
			data: { domain_uuid: "g-1" },
		});
	});
});

describe("domainRoleApi", () => {
	it("listDomainRoles calls correct endpoint", async () => {
		const data = { roles: [], assignable: ["room:read"] };
		(apiClient.post as any).mockResolvedValue(data);
		const result = await listDomainRoles("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/roles/list",
			data: { domain_uuid: "g-1" },
		});
		expect(result.assignable).toEqual(["room:read"]);
	});

	it("createDomainRole sends name and permissions", async () => {
		(apiClient.post as any).mockResolvedValue(null);
		await createDomainRole("g-1", "moderator", ["room:read"]);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/roles/create",
			data: {
				domain_uuid: "g-1",
				name: "moderator",
				permissions: ["room:read"],
			},
		});
	});

	it("updateDomainMemberRole sends role name", async () => {
		(apiClient.post as any).mockResolvedValue(null);
		await updateDomainMemberRole("g-1", "u-2", "admin");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/members/update-role",
			data: { domain_uuid: "g-1", user_uuid: "u-2", role_name: "admin" },
		});
	});

	it("myDomainPermissions returns role and codes", async () => {
		const data = { role_name: "admin", permissions: ["room:read"] };
		(apiClient.post as any).mockResolvedValue(data);
		const result = await myDomainPermissions("g-1");
		expect(result.role_name).toBe("admin");
	});
});
