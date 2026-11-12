import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./apiClient", () => ({
	default: { get: vi.fn() },
}));

import apiClient from "./apiClient";
import { getWSEndpoint } from "./ws";

describe("getWSEndpoint", () => {
	beforeEach(() => {
		vi.mocked(apiClient.get).mockReset();
	});

	it("passes domain_uuid for worker-aware reconnect", async () => {
		vi.mocked(apiClient.get).mockResolvedValue({
			url: "https://entry.example/ws?worker=worker-1",
		});

		const result = await getWSEndpoint("domain-a");

		expect(apiClient.get).toHaveBeenCalledWith(
			expect.objectContaining({
				url: "/api/v1/signal/ws-endpoint",
				params: { domain_uuid: "domain-a" },
			}),
		);
		expect(result).toEqual({
			url: "https://entry.example/ws?worker=worker-1",
		});
	});

	it("omits params when no domain is active", async () => {
		vi.mocked(apiClient.get).mockResolvedValue({});

		const result = await getWSEndpoint();

		expect(apiClient.get).toHaveBeenCalledWith(
			expect.objectContaining({
				url: "/api/v1/signal/ws-endpoint",
				params: undefined,
			}),
		);
		expect(result.url).toBeUndefined();
	});
});
