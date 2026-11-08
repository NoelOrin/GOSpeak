import { describe, expect, it } from "vitest";
import { api, assertSuccess, getAdminToken, unique } from "../helpers";

describe("role and permission module", () => {
  it("lists seeded roles and permissions", async () => {
    const admin = await getAdminToken();

    const roles = await api<Array<{ id: number; name: string }>>("/api/v1/role/list", { token: admin });
    expect(roles.code).toBe(0);
    expect(roles.data?.some((role) => role.name === "admin")).toBe(true);
    expect(roles.data?.some((role) => role.name === "user")).toBe(true);

    const permissions = await api<Array<{ code: string; name: string }>>("/api/v1/permission/list", {
      token: admin,
    });
    expect(permissions.code).toBe(0);
    expect(permissions.data?.length).toBeGreaterThan(0);
  });

  it("creates, syncs, updates and deletes a role", async () => {
    const admin = await getAdminToken();
    const roleName = unique("role");

    const created = await api<{ id: number; name: string }>("/api/v1/role/create", {
      token: admin,
      body: { name: roleName },
    });
    const role = assertSuccess(created);
    expect(role.id).toBeGreaterThan(0);

    const synced = await api("/api/v1/permission/sync", {
      token: admin,
      body: { role: roleName, permissions: ["room:read"] },
    });
    expect(synced.code).toBe(0);

    const rolePermissions = await api<{ role: string; permissions: string[] }>("/api/v1/permission/role", {
      token: admin,
      body: { role: roleName },
    });
    expect(rolePermissions.code).toBe(0);
    expect(rolePermissions.data?.permissions).toContain("room:read");

    const updated = await api<{ name: string }>("/api/v1/role/update", {
      token: admin,
      body: { id: role.id, name: `${roleName}_updated` },
    });
    expect(updated.code).toBe(0);
    expect(updated.data?.name.endsWith("_updated")).toBe(true);

    const deleted = await api("/api/v1/role/delete", {
      token: admin,
      body: { id: role.id },
    });
    expect(deleted.code).toBe(0);
  });
});
