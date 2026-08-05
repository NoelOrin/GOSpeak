import { describe, expect, it } from "vitest";
import { api, assertSuccess, getAdminToken, registerUser, unique } from "../helpers";

describe("bot module", () => {
  it("keeps bot management admin-only", async () => {
    const user = await registerUser("acct");
    const result = await api("/api/v1/bot/create", {
      token: user.access_token,
      body: { name: unique("user_bot"), permissions: ["room:read"] },
    });
    expect(result.code).toBe(1013);
  });

  it("creates, uses and revokes a scoped bot token", async () => {
    const admin = await getAdminToken();
    const name = unique("svc_bot");

    const created = await api<{ token: string; token_uuid: string; user: { is_bot: boolean } }>("/api/v1/bot/create", {
      token: admin,
      body: { name, permissions: ["room:read"], expires_in: "" },
    });
    const bot = assertSuccess(created);
    expect(bot.token).toBeTruthy();
    expect(bot.user.is_bot).toBe(true);

    const rooms = await api<{ rooms: unknown[]; total: number }>("/api/v1/room/list", {
      token: bot.token,
      body: {},
    });
    expect(rooms.code).toBe(0);

    const list = await api<Array<{ uuid: string; name: string; revoked: boolean }>>("/api/v1/bot/list", {
      token: admin,
    });
    expect(list.data?.some((item) => item.uuid === bot.token_uuid && !item.revoked)).toBe(true);

    const revoked = await api("/api/v1/bot/revoke", {
      token: admin,
      body: { uuid: bot.token_uuid },
    });
    expect(revoked.code).toBe(0);

    const afterRevoke = await api("/api/v1/room/list", { token: bot.token, body: {} });
    expect(afterRevoke.code).toBe(1014);
  });
});
