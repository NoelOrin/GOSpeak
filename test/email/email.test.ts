import { describe, expect, it } from "vitest";
import { api, getAdminToken, registerUser, unique } from "../helpers";

describe("email module", () => {
  it("returns email config without enabling SMTP", async () => {
    const admin = await getAdminToken();
    const result = await api<{ enabled: boolean; available: boolean }>("/api/v1/email/config", {
      token: admin,
    });
    expect(result.code).toBe(0);
    expect(result.data?.enabled).toBe(false);
    expect(result.data?.available).toBe(false);
  });

  it("rejects send_code when email is not configured", async () => {
    const result = await api("/api/v1/email/send_code", {
      body: { email: `${unique("email")}@example.com`, scene: "register" },
    });
    expect(result.status).toBe(503);
    expect(result.code).toBe(8009);
  });

  it("keeps email config write access admin-only", async () => {
    const user = await registerUser("email");
    const result = await api("/api/v1/email/config", { token: user.access_token });
    expect(result.code).toBe(1013);
  });
});
