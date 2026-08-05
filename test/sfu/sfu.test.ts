import { describe, expect, it } from "vitest";
import { api, getAdminToken, registerUser } from "../helpers";

describe("sfu module", () => {
  it("reads the active srs provider config", async () => {
    const admin = await getAdminToken();
    const result = await api<{ provider: string; srs_secret_set: boolean }>("/api/v1/sfu/config", {
      token: admin,
    });
    expect(result.code).toBe(0);
    expect(result.data?.provider).toBe("srs");
    expect(result.data?.srs_secret_set).toBe(true);
  });

  it("lists providers and capabilities", async () => {
    const admin = await getAdminToken();
    const result = await api<{ providers: Array<{ provider: string }>; active: string; capabilities: Record<string, unknown> }>(
      "/api/v1/sfu/providers",
      { token: admin },
    );
    expect(result.code).toBe(0);
    expect(result.data?.active).toBe("srs");
    expect(result.data?.providers.some((item) => item.provider === "srs")).toBe(true);
    expect(Object.keys(result.data?.capabilities ?? {}).length).toBeGreaterThan(0);
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
    const result = await api("/api/v1/sfu/config", { token: user.access_token });
    expect(result.code).toBe(1013);
  });
});
