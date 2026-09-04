import { describe, expect, it } from "vitest";
import {
  api,
  assertSuccess,
  createDomain,
  createDomainRole,
  joinDomain,
  listDomainRoles,
  registerUser,
  unique,
  updateDomainMemberRole,
} from "../helpers";

describe("domain module", () => {
  it("creates and reads a domain as its owner", async () => {
    const owner = await registerUser("domain");
    const domain = await createDomain(owner.access_token, unique("domain"));

    const get = await api<{ uuid: string; name: string; owner_uuid: string }>("/api/v1/domain/get", {
      token: owner.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(get.code).toBe(0);
    expect(get.data?.uuid).toBe(domain.uuid);
    expect(get.data?.owner_uuid).toBe(owner.user.uuid);
  });

  it("supports preview, join, members, kick, leave and delete", async () => {
    const owner = await registerUser("domain");
    const member = await registerUser("domain");
    const domain = await createDomain(owner.access_token, unique("domain"));

    const preview = await api<{ uuid: string; invite_code: string }>("/api/v1/domain/preview", {
      token: member.access_token,
      body: { invite_code: domain.invite_code },
    });
    expect(preview.code).toBe(0);
    expect(preview.data?.uuid).toBe(domain.uuid);

    await joinDomain(member.access_token, domain.invite_code);

    const members = await api<{ members: Array<{ user_uuid: string; role_name: string }> }>(
      "/api/v1/domain/members",
      { token: owner.access_token, body: { domain_uuid: domain.uuid } },
    );
    expect(members.code).toBe(0);
    expect(members.data?.members.some((item) => item.user_uuid === member.user.uuid)).toBe(true);

    const kicked = await api("/api/v1/domain/kick", {
      token: owner.access_token,
      body: { domain_uuid: domain.uuid, user_uuid: member.user.uuid },
    });
    expect(kicked.code).toBe(0);

    await joinDomain(member.access_token, domain.invite_code);

    const left = await api("/api/v1/domain/leave", {
      token: member.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(left.code).toBe(0);

    const updated = await api<{ name: string }>("/api/v1/domain/update", {
      token: owner.access_token,
      body: { domain_uuid: domain.uuid, name: `${domain.name}_updated` },
    });
    const updatedDomain = assertSuccess(updated);
    expect(updatedDomain.name.endsWith("_updated")).toBe(true);

    const deleted = await api("/api/v1/domain/delete", {
      token: owner.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(deleted.code).toBe(0);

    const afterDelete = await api("/api/v1/domain/get", {
      token: owner.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(afterDelete.code).toBe(1013);
  });

  it("lists public and my domains", async () => {
    const owner = await registerUser("domain");
    const name = unique("public_domain");
    const created = await api<{ uuid: string; invite_code: string; is_public: boolean }>(
      "/api/v1/domain/create",
      { token: owner.access_token, body: { name, is_public: true } },
    );
    const domain = assertSuccess(created);
    expect(domain.is_public).toBe(true);

    const publicList = await api<{ domains: Array<{ uuid: string }> }>("/api/v1/domain/list-public", {
      token: owner.access_token,
      body: { keyword: name },
    });
    expect(publicList.data?.domains.some((item) => item.uuid === domain.uuid)).toBe(true);

    const mine = await api<Array<{ uuid: string }>>("/api/v1/domain/my-domains", {
      token: owner.access_token,
    });
    expect(mine.data?.some((item) => item.uuid === domain.uuid)).toBe(true);
  });

  it("creates per-domain role and assigns member", async () => {
    const owner = await registerUser("domain_role_owner");
    const created = await api<{ uuid: string }>("/api/v1/domain/create", {
      token: owner.access_token,
      body: { name: unique("role_domain"), is_public: false },
    });
    const domain = assertSuccess(created);

    const roles = await listDomainRoles(owner.access_token, domain.uuid);
    expect(roles.roles.some((r) => r.name === "owner")).toBe(true);
    expect(roles.assignable).toContain("room:read");

    const member = await registerUser("domain_role_member");
    await joinDomain(member.access_token, domain.invite_code);
    await createDomainRole(owner.access_token, domain.uuid, "moderator", ["room:read", "message:delete_others"]);
    await updateDomainMemberRole(owner.access_token, domain.uuid, member.user.uuid, "moderator");

    const after = await listDomainRoles(owner.access_token, domain.uuid);
    expect(after.roles.find((r) => r.name === "moderator")?.permissions).toEqual(
      expect.arrayContaining(["room:read", "message:delete_others"]),
    );
  });
});
