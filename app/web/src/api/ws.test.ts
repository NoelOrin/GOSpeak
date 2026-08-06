import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./apiClient", () => ({
	default: { get: vi.fn() },
}));

import apiClient from "./apiClient";
import { getWSTicket } from "./ws";

describe("getWSTicket", () => {
	beforeEach(() => {
		vi.mocked(apiClient.get).mockReset();
	});

	it("passes domain_uuid for worker-aware reconnect", async () => {
		vi.mocked(apiClient.get).mockResolvedValue({
			ticket: "ticket-1",
			url: "https://entry.example/ws?worker=worker-1",
		});

		const result = await getWSTicket("domain-a");

		expect(apiClient.get).toHaveBeenCalledWith(
			expect.objectContaining({
				url: "/api/v1/signal/ws-ticket",
				params: { domain_uuid: "domain-a" },
			}),
		);
		expect(result).toEqual({
			token: "ticket-1",
			url: "https://entry.example/ws?worker=worker-1",
		});
	});

	it("omits params when no domain is active", async () => {
		vi.mocked(apiClient.get).mockResolvedValue({ ticket: "ticket-1" });

		const result = await getWSTicket();

		expect(apiClient.get).toHaveBeenCalledWith(
			expect.objectContaining({
				url: "/api/v1/signal/ws-ticket",
				params: undefined,
			}),
		);
		expect(result.url).toBeUndefined();
	});
});
