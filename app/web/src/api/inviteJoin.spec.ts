import { beforeEach, describe, expect, it, vi } from "vitest";

const post = vi.fn();
vi.mock("./apiClient", () => ({
	default: { post: (...args: unknown[]) => post(...args) },
}));

import {
	cancelJoinRequest,
	getJoinRequest,
	joinByCode,
	joinByDomain,
	previewInvite,
	registerViaInvite,
	setJoinApproval,
} from "./inviteJoin";

describe("inviteJoin api", () => {
	beforeEach(() => {
		post.mockReset();
	});

	it("previewInvite posts code to /api/v1/invite/preview without toast", async () => {
		post.mockResolvedValue({
			domain: { uuid: "d1", name: "Realm", icon_url: "", description: "" },
			requires_approval: true,
			remaining_uses: 3,
		});
		const got = await previewInvite("CODE");
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/invite/preview",
			data: { code: "CODE" },
			skipErrorToast: true,
		});
		expect(got.requires_approval).toBe(true);
	});

	it("registerViaInvite posts the inline registration payload", async () => {
		post.mockResolvedValue({ status: "joined", domain: { uuid: "d1" } });
		await registerViaInvite({
			code: "CODE",
			username: "u1",
			password: "password123",
			nickname: "小明",
		});
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/invite/register",
			data: {
				code: "CODE",
				username: "u1",
				password: "password123",
				nickname: "小明",
				email: "",
				email_code: "",
			},
			skipErrorToast: true,
		});
	});

	it("joinByCode and joinByDomain hit the same endpoint with different bodies", async () => {
		post.mockResolvedValue({ status: "joined", domain: { uuid: "d1" } });
		await joinByCode("CODE");
		expect(post).toHaveBeenLastCalledWith({
			url: "/api/v1/invite/join",
			data: { code: "CODE" },
			skipErrorToast: true,
		});
		await joinByDomain("d1");
		expect(post).toHaveBeenLastCalledWith({
			url: "/api/v1/invite/join",
			data: { domain_uuid: "d1" },
			skipErrorToast: true,
		});
	});

	it("getJoinRequest and cancelJoinRequest use domain_uuid", async () => {
		post.mockResolvedValue({ request: null, status: "", is_member: false });
		await getJoinRequest("d1");
		expect(post).toHaveBeenLastCalledWith({
			url: "/api/v1/invite/my-request",
			data: { domain_uuid: "d1" },
			skipErrorToast: true,
		});
		post.mockResolvedValue({ ok: true });
		await cancelJoinRequest("d1");
		expect(post).toHaveBeenLastCalledWith({
			url: "/api/v1/invite/cancel-request",
			data: { domain_uuid: "d1" },
		});
	});

	it("setJoinApproval posts to the domain config endpoint", async () => {
		post.mockResolvedValue({ uuid: "d1", require_join_approval: true });
		const got = await setJoinApproval("d1", true);
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/domain/join-approval/config",
			data: { domain_uuid: "d1", require_join_approval: true },
		});
		expect(got.require_join_approval).toBe(true);
	});
});
