import { describe, expect, it, vi } from "vitest";

const mockInstance = vi.hoisted(() => {
	const instance = {
		interceptors: {
			request: { use: vi.fn() },
			response: { use: vi.fn() },
		},
		request: vi.fn(),
	};
	return { instance };
});

vi.mock("axios", () => ({
	default: {
		create: vi.fn(() => mockInstance.instance),
	},
}));

import { APIClient } from "./apiClient";

describe("APIClient", () => {
	it("resolves Result.data instead of AxiosResponse", async () => {
		const client = new APIClient("/");
		mockInstance.instance.request.mockResolvedValue({
			data: { code: 0, msg: "success", data: { ok: true } },
			status: 200,
			statusText: "OK",
			headers: {},
			config: {},
		});

		const value = await client.get<{ ok: boolean }>({ url: "/x" });
		expect(value.ok).toBe(true);
	});
});
