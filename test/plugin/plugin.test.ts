import { describe, expect, it } from "vitest";
import { api, getAdminToken, registerUser } from "../helpers";

describe("plugin module", () => {
  it("lists and reads the builtin plugin", async () => {
    const admin = await getAdminToken();
    const list = await api<Array<{ name: string; enabled: boolean; status: string }>>("/api/v1/plugins/list", {
      token: admin,
    });
    expect(list.code).toBe(0);
    expect(list.data?.length).toBeGreaterThan(0);
    expect(list.data?.some((item) => item.name === "bot-base")).toBe(true);

    const get = await api<{ name: string; display_name: string }>("/api/v1/plugins/get", {
      token: admin,
      body: { name: "bot-base" },
    });
    expect(get.code).toBe(0);
    expect(get.data?.name).toBe("bot-base");
  });

  it("keeps plugin management admin-only", async () => {
    const user = await registerUser("plugin");
    const result = await api("/api/v1/plugins/list", { token: user.access_token });
    expect(result.code).toBe(1013);
  });
});
