import { describe, expect, it } from "vitest";
import { api, assertSuccess, getAdminToken, registerUser, unique } from "../helpers";

describe("oauth module", () => {
  it("creates, exposes, updates and deletes a provider", async () => {
    const admin = await getAdminToken();
    const name = unique("oauth_provider");

    const created = await api<{ id: number; name: string; client_secret_set: boolean }>(
      "/api/v1/oauth/admin/providers",
      {
        method: "POST",
        token: admin,
        body: {
          name,
          display_name: "Integration Provider",
          client_id: "client-id",
          client_secret: "client-secret",
          auth_url: "https://example.com/oauth/authorize",
          token_url: "https://example.com/oauth/token",
          user_info_url: "https://example.com/oauth/userinfo",
          redirect_url: "https://example.com/callback",
          enabled: true,
        },
      },
    );
    const provider = assertSuccess(created);
    expect(provider.id).toBeGreaterThan(0);
    expect(provider.client_secret_set).toBe(true);

    const enabled = await api<Array<{ name: string }>>("/api/v1/oauth/providers", {
      method: "GET",
    });
    expect(enabled.data?.some((item) => item.name === name)).toBe(true);

    const login = await api("/api/v1/oauth/login/" + name, {
      method: "GET",
      redirect: "manual",
    });
    expect(login.status).toBe(302);

    const updated = await api<{ id: number; display_name: string }>("/api/v1/oauth/admin/providers", {
      method: "PUT",
      token: admin,
      body: { id: provider.id, display_name: "Renamed Provider" },
    });
    expect(updated.code).toBe(0);
    expect(updated.data?.display_name).toBe("Renamed Provider");

    const deleted = await api(`/api/v1/oauth/admin/providers/${provider.id}`, {
      method: "DELETE",
      token: admin,
    });
    expect(deleted.code).toBe(0);
  });

  it("keeps oauth admin routes admin-only", async () => {
    const user = await registerUser("oauth");
    const result = await api("/api/v1/oauth/admin/providers", {
      method: "GET",
      token: user.access_token,
    });
    expect(result.code).toBe(1013);
  });
});
