import { describe, expect, it } from "vitest";
import { api, assertSuccess, getAdminToken, registerUser, unique } from "../helpers";

describe("user module", () => {
  it("reads and updates the current profile", async () => {
    const user = await registerUser("user");

    const profile = await api<{ name: string; display_name: string }>("/api/v1/user/profile", {
      token: user.access_token,
    });
    expect(profile.code).toBe(0);
    expect(profile.data?.name).toBe(user.username);

    const updated = await api<{ name: string; display_name: string }>("/api/v1/user/update-profile", {
      token: user.access_token,
      body: { display_name: "integration-profile" },
    });
    expect(updated.code).toBe(0);
    expect(updated.data?.display_name).toBe("integration-profile");
  });

  it("resolves users by name and id for authenticated callers", async () => {
    const user = await registerUser("user");

    const byName = await api<{ id: number; name: string }>("/api/v1/user/info", {
      token: user.access_token,
      body: { identity: user.username },
    });
    expect(byName.code).toBe(0);
    expect(byName.data?.name).toBe(user.username);

    const byId = await api<{ id: number; name: string }>("/api/v1/user/get", {
      token: user.access_token,
      body: { id: user.user.id },
    });
    expect(byId.code).toBe(0);
    expect(byId.data?.id).toBe(user.user.id);
  });

  it("allows admins to list and update roles", async () => {
    const admin = await getAdminToken();
    const user = await registerUser("user");

    const list = await api<{ list: Array<{ id: number; name: string; role: string }>; total: number }>(
      "/api/v1/user/list",
      { token: admin, body: { page: 1, page_size: 100, keyword: user.username } },
    );
    expect(list.code).toBe(0);
    expect(list.data?.list.some((item) => item.name === user.username)).toBe(true);

    const promote = await api("/api/v1/user/update-role", {
      token: admin,
      body: { id: user.user.id, role: "admin" },
    });
    expect(promote.code).toBe(0);

    const demote = await api("/api/v1/user/update-role", {
      token: admin,
      body: { id: user.user.id, role: "user" },
    });
    expect(demote.code).toBe(0);
  });

  it("lets admins delete a user", async () => {
    const admin = await getAdminToken();
    const user = await registerUser("user");

    const deleted = await api("/api/v1/user/delete", {
      token: admin,
      body: { id: user.user.id },
    });
    expect(deleted.code).toBe(0);

    const getDeleted = await api("/api/v1/user/get", { token: admin, body: { id: user.user.id } });
    expect(getDeleted.code).toBe(3001);
  });

  it("supports current-user group CRUD", async () => {
    const user = await registerUser("user");
    const groupName = unique("group");

    const created = await api<{ id: number; name: string }>("/api/v1/user-group/create", {
      token: user.access_token,
      body: { group_name: groupName },
    });
    const group = assertSuccess(created);
    expect(group.id).toBeGreaterThan(0);

    const listed = await api<{ groups: Array<{ id: number; name: string }> }>("/api/v1/user-group/list", {
      token: user.access_token,
    });
    expect(listed.data?.groups.some((item) => item.id === group.id)).toBe(true);

    const renamed = await api("/api/v1/user-group/update", {
      token: user.access_token,
      body: { id: group.id, group_name: `${groupName}_renamed` },
    });
    expect(renamed.code).toBe(0);

    const deleted = await api("/api/v1/user-group/delete", {
      token: user.access_token,
      body: { id: group.id },
    });
    expect(deleted.code).toBe(0);
  });
});
