import { describe, expect, it } from "vitest";
import { api, getAdminToken, registerUser } from "../helpers";

describe("sfu module", () => {
	it("reads the active livekit provider config", async () => {
		const admin = await getAdminToken();
		const result = await api<{ provider: string }>("/api/v1/sfu/config", {
			token: admin,
		});
		expect(result.code).toBe(0);
		expect(result.data?.provider).toBe("livekit");
	});

	it("lists providers and capabilities", async () => {
		const admin = await getAdminToken();
		const result = await api<{
			providers: Array<{ provider: string }>;
			active: string;
			capabilities: Record<string, unknown>;
		}>("/api/v1/sfu/providers", { token: admin });
		expect(result.code).toBe(0);
		expect(result.data?.active).toBe("livekit");
		expect(
			result.data?.providers.some((item) => item.provider === "livekit"),
		).toBe(true);
		expect(Object.keys(result.data?.capabilities ?? {}).length).toBeGreaterThan(
			0,
		);
	});

	it("rejects invalid provider switching", async () => {
		const admin = await getAdminToken();
		const result = await api("/api/v1/sfu/switch-provider", {
			token: admin,
			body: { provider: "not-a-provider" },
		});
		expect(result.code).toBe(2001);
	});

	it("keeps sfu config management admin-only", async () => {
		const user = await registerUser("sfu");
		const result = await api("/api/v1/sfu/config", {
			token: user.access_token,
		});
		expect(result.code).toBe(1013);
	});
});
