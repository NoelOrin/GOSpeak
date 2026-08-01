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
	createDomain,
	deleteDomain,
	getDomain,
	domainMembers,
	joinDomain,
	kickDomainMember,
	leaveDomain,
	listDomains,
	listPublicDomains,
	myDomains,
	previewDomainInvite,
	updateDomain,
} from "@/api/domain";

function mockResult<T>(data: T): AxiosResponse<Result<T>> {
	return {
		data: { code: 0, msg: "success", data },
		status: 200,
		statusText: "OK",
		headers: {},
		config: {} as any,
	};
}

describe("domainApi", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("createDomain calls correct endpoint", async () => {
		const domain = { uuid: "g-1", name: "Test" } as any;
		(apiClient.post as any).mockResolvedValue(mockResult(domain));
		const result = await createDomain({ name: "Test", is_public: true });
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/create",
			data: { name: "Test", is_public: true },
		});
		expect(result.uuid).toBe("g-1");
	});

	it("getDomain calls correct endpoint", async () => {
		const domain = { uuid: "g-1", name: "Test" } as any;
		(apiClient.post as any).mockResolvedValue(mockResult(domain));
		const result = await getDomain("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/get",
			data: { domain_uuid: "g-1" },
		});
		expect(result.name).toBe("Test");
	});

	it("listDomains builds query params", async () => {
		const data = { domains: [], total: 0 };
		(apiClient.post as any).mockResolvedValue(mockResult(data));
		await listDomains(2, 10);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/list",
			data: { page: 2, page_size: 10 },
		});
	});

	it("listPublicDomains builds query params", async () => {
		const data = { domains: [], total: 0 };
		(apiClient.post as any).mockResolvedValue(mockResult(data));
		await listPublicDomains(1, 20);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/list-public",
			data: { page: 1, page_size: 20 },
		});
	});

	it("listPublicDomains sends keyword when provided", async () => {
		const data = { domains: [], total: 0 };
		(apiClient.post as any).mockResolvedValue(mockResult(data));
		await listPublicDomains(2, 10, "Alpha");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/list-public",
			data: { page: 2, page_size: 10, keyword: "Alpha" },
		});
	});

	it("listPublicDomains trims keyword and preserves pagination", async () => {
		(apiClient.post as any).mockResolvedValue(
			mockResult({ domains: [], total: 0 }),
		);

		await listPublicDomains(2, 12, "  alpha  ");

		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/list-public",
			data: { page: 2, page_size: 12, keyword: "alpha" },
		});
	});

	it("myDomains returns UUIDs from response", async () => {
		(apiClient.post as any).mockResolvedValue(
			mockResult({ domain_uuids: ["g-1", "g-2"] }),
		);
		const uuids = await myDomains();
		expect(uuids).toEqual(["g-1", "g-2"]);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/my-domains",
			data: {},
		});
	});

	it("previewDomainInvite sends the normalized invite code", async () => {
		const domain = { uuid: "g-1", name: "Preview" } as any;
		(apiClient.post as any).mockResolvedValue(mockResult(domain));
		const result = await previewDomainInvite("ABCDEFGH");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/preview",
			data: { invite_code: "ABCDEFGH" },
		});
		expect(result.name).toBe("Preview");
	});

	it("joinDomain calls with invite_code", async () => {
		const domain = { uuid: "g-1", name: "Joined" } as any;
		(apiClient.post as any).mockResolvedValue(mockResult(domain));
		const result = await joinDomain("CODE123");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/join",
			data: { invite_code: "CODE123" },
		});
		expect(result.name).toBe("Joined");
	});

	it("leaveDomain calls with domain_uuid", async () => {
		(apiClient.post as any).mockResolvedValue(mockResult(null));
		await leaveDomain("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/leave",
			data: { domain_uuid: "g-1" },
		});
	});

	it("updateDomain sends only provided fields", async () => {
		const domain = { uuid: "g-1", name: "Updated" } as any;
		(apiClient.post as any).mockResolvedValue(mockResult(domain));
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
		(apiClient.post as any).mockResolvedValue(mockResult(null));
		await deleteDomain("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/delete",
			data: { domain_uuid: "g-1" },
		});
	});

	it("kickDomainMember calls with domain_uuid and user_uuid", async () => {
		(apiClient.post as any).mockResolvedValue(mockResult(null));
		await kickDomainMember("g-1", "u-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/kick",
			data: { domain_uuid: "g-1", user_uuid: "u-1" },
		});
	});

	it("domainMembers returns member list", async () => {
		const members = [{ id: 1, user_uuid: "u-1", role_name: "member" }];
		(apiClient.post as any).mockResolvedValue(mockResult({ members }));
		const result = await domainMembers("g-1");
		expect(result).toEqual(members);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/members",
			data: { domain_uuid: "g-1" },
		});
	});
});
