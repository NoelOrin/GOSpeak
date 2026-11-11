import { beforeEach, describe, expect, it, vi } from "vitest";

const post = vi.fn();
vi.mock("@/api/apiClient", () => ({
	default: { post: (...args: unknown[]) => post(...args) },
}));

const {
	createInvite,
	disableInvite,
	inviteStatus,
	listInvites,
	listRedemptions,
	previewInvite,
} = await import("./invite");

describe("invite api", () => {
	beforeEach(() => post.mockReset());

	it("previewInvite posts the code with skipErrorToast", async () => {
		post.mockResolvedValue({
			domain: { uuid: "d1" },
			requires_approval: false,
			remaining_uses: -1,
		});
		const result = await previewInvite("CODE");
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/invite/preview",
			data: { code: "CODE" },
			skipErrorToast: true,
		});
		expect(result.requires_approval).toBe(false);
	});

	it("createInvite sends domain uuid and options", async () => {
		post.mockResolvedValue({ id: 1, code: "C" });
		await createInvite({
			domain_uuid: "d1",
			label: "测试",
			expires_in_hours: 24,
			max_uses: 5,
		});
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/domain/invites/create",
			data: {
				domain_uuid: "d1",
				label: "测试",
				expires_in_hours: 24,
				max_uses: 5,
			},
		});
	});

	it("listInvites unwraps the invites array", async () => {
		post.mockResolvedValue({ invites: [{ id: 1 }] });
		const list = await listInvites("d1");
		expect(list).toHaveLength(1);
	});

	it("disableInvite posts invite id", async () => {
		post.mockResolvedValue(null);
		await disableInvite("d1", 7);
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/domain/invites/disable",
			data: { domain_uuid: "d1", invite_id: 7 },
		});
	});

	it("listRedemptions unwraps items and total", async () => {
		post.mockResolvedValue({ items: [{ id: 1 }], total: 1 });
		const result = await listRedemptions("d1", 3, 1, 20);
		expect(result.total).toBe(1);
	});
});

describe("inviteStatus", () => {
	const base = {
		id: 1,
		domain_uuid: "d",
		label: "",
		code: "C",
		expires_at: null,
		max_uses: 0,
		used_count: 0,
		disabled_at: null,
		created_by: "u",
		created_at: "2026-11-10T00:00:00Z",
	};

	it("reports active for a fresh unlimited invite", () => {
		expect(inviteStatus(base)).toBe("active");
	});

	it("reports disabled when disabled_at is set", () => {
		expect(inviteStatus({ ...base, disabled_at: "2026-11-10T01:00:00Z" })).toBe(
			"disabled",
		);
	});

	it("reports expired for a past expiry", () => {
		expect(inviteStatus({ ...base, expires_at: "2020-01-01T00:00:00Z" })).toBe(
			"expired",
		);
	});

	it("reports exhausted when used up", () => {
		expect(inviteStatus({ ...base, max_uses: 2, used_count: 2 })).toBe(
			"exhausted",
		);
	});
});
