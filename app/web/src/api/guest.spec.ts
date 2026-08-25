import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/apiClient", () => {
	const mockPost = vi.fn();
	return { default: { post: mockPost }, mockPost };
});

import apiClient from "@/api/apiClient";
import {
	banGuest,
	cleanupInactiveGuests,
	getGuestConfig,
	guestJoin,
	guestRenew,
	listGuestBans,
	unbanGuest,
	updateGuestConfig,
} from "@/api/guest";

const postMock = vi.mocked(apiClient.post);

describe("guestApi", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("guestJoin posts to /auth/guest", async () => {
		postMock.mockResolvedValueOnce({ access_token: "a" } as never);
		await guestJoin({ nickname: "路人", invite_code: "code1" });
		expect(postMock).toHaveBeenCalledWith({
			url: "/api/v1/auth/guest",
			data: { nickname: "路人", invite_code: "code1" },
		});
	});

	it("guestRenew posts to /auth/guest/renew", async () => {
		postMock.mockResolvedValueOnce({} as never);
		await guestRenew({ invite_code: "code2" });
		expect(postMock).toHaveBeenCalledWith({
			url: "/api/v1/auth/guest/renew",
			data: { invite_code: "code2" },
		});
	});

	it("getGuestConfig posts read-only payload", async () => {
		postMock.mockResolvedValueOnce({ allow_guest: true } as never);
		const cfg = await getGuestConfig("d-1");
		expect(cfg.allow_guest).toBe(true);
		expect(postMock).toHaveBeenCalledWith({
			url: "/api/v1/domain/guest/config",
			data: { domain_uuid: "d-1" },
		});
	});

	it("updateGuestConfig forwards partial fields", async () => {
		postMock.mockResolvedValueOnce({} as never);
		await updateGuestConfig({ domain_uuid: "d-1", guest_limit: 7 });
		expect(postMock).toHaveBeenCalledWith({
			url: "/api/v1/domain/guest/config",
			data: { domain_uuid: "d-1", guest_limit: 7 },
		});
	});

	it("ban/unban/list/cleanup hit their endpoints", async () => {
		postMock.mockResolvedValue(undefined as never);
		await banGuest({ domain_uuid: "d", user_uuid: "u", duration_hours: 24 });
		expect(postMock.mock.calls[0][0].url).toBe("/api/v1/domain/guest/ban");
		await unbanGuest({ domain_uuid: "d", user_uuid: "u" });
		expect(postMock.mock.calls[1][0].url).toBe("/api/v1/domain/guest/unban");
		postMock.mockResolvedValueOnce([] as never);
		await listGuestBans("d");
		expect(postMock.mock.calls[2][0].url).toBe("/api/v1/domain/guest/ban-list");
		postMock.mockResolvedValueOnce({ removed: 3 } as never);
		const res = await cleanupInactiveGuests({ domain_uuid: "d", days: 30 });
		expect(res.removed).toBe(3);
		expect(postMock.mock.calls[3][0].url).toBe("/api/v1/domain/guest/cleanup");
	});
});
