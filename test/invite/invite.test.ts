import { describe, expect, it } from "vitest";
import { api, assertSuccess, createDomain, registerUser, unique } from "../helpers";

interface DomainInviteDTO {
  id: number;
  domain_uuid: string;
  code: string;
  max_uses: number;
  used_count: number;
  disabled_at?: string | null;
}

async function createInvite(
  token: string,
  domainUUID: string,
  body: Record<string, unknown> = {},
): Promise<DomainInviteDTO> {
  const res = await api<DomainInviteDTO>("/api/v1/domain/invites/create", {
    token,
    body: { domain_uuid: domainUUID, label: unique("invite"), ...body },
  });
  return assertSuccess(res);
}

async function listInvites(token: string, domainUUID: string): Promise<DomainInviteDTO[]> {
  const res = await api<{ invites: DomainInviteDTO[] }>("/api/v1/domain/invites/list", {
    token,
    body: { domain_uuid: domainUUID },
  });
  return assertSuccess(res).invites;
}

async function setRequireApproval(token: string, domainUUID: string, require: boolean): Promise<void> {
  const res = await api("/api/v1/domain/join-approval/config", {
    token,
    body: { domain_uuid: domainUUID, require_join_approval: require },
  });
  expect(res.code).toBe(0);
}

describe("invite registration and approval", () => {
  it("creates a domain with a default invite code", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invites = await listInvites(owner.access_token, domain.uuid);
    expect(invites.length).toBeGreaterThanOrEqual(1);
    const code = invites[0].code;
    expect(code.length).toBe(36);
    expect(/^[A-HJ-NP-Z2-9]{36}$/.test(code)).toBe(true);
  });

  it("previews an invite without leaking the code", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invite = await createInvite(owner.access_token, domain.uuid);

    const preview = await api<{
      domain: { uuid: string; name: string };
      requires_approval: boolean;
      remaining_uses: number;
    }>("/api/v1/invite/preview", { body: { code: invite.code } });
    expect(preview.code).toBe(0);
    expect(preview.data?.domain.uuid).toBe(domain.uuid);
    expect(preview.data?.requires_approval).toBe(false);
    expect(preview.raw.includes(invite.code)).toBe(false);
  });

  it("registers and joins directly when approval is off", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invite = await createInvite(owner.access_token, domain.uuid);

    const username = unique("newbie");
    const res = await api<{ status: string; user: { uuid: string }; domain: { uuid: string } }>(
      "/api/v1/invite/register",
      { body: { code: invite.code, username, password: "password123", nickname: "新人" } },
    );
    const data = assertSuccess(res);
    expect(data.status).toBe("joined");
    expect(data.domain.uuid).toBe(domain.uuid);

    const members = await api<{ members: Array<{ user_uuid: string; role_name: string }> }>(
      "/api/v1/domain/members",
      { token: owner.access_token, body: { domain_uuid: domain.uuid } },
    );
    const joined = members.data?.members.find((m) => m.user_uuid === data.user.uuid);
    expect(joined?.role_name).toBe("member");

    // 免审核域不得留下任何申请记录（§8.2）
    const pendingList = await api<{ items: Array<{ user_uuid: string }>; total: number }>(
      "/api/v1/domain/join-requests/list",
      { token: owner.access_token, body: { domain_uuid: domain.uuid } },
    );
    expect(pendingList.data?.total).toBe(0);
  });

  it("registers into pending when approval is on", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invite = await createInvite(owner.access_token, domain.uuid);
    await setRequireApproval(owner.access_token, domain.uuid, true);

    const username = unique("waiter");
    const res = await api<{ status: string; user: { uuid: string }; access_token: string }>(
      "/api/v1/invite/register",
      { body: { code: invite.code, username, password: "password123" } },
    );
    const data = assertSuccess(res);
    expect(data.status).toBe("pending");

    const mine = await api<{ status: string; is_member: boolean }>("/api/v1/invite/my-request", {
      token: data.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(mine.data?.status).toBe("pending");
    expect(mine.data?.is_member).toBe(false);
  });

  it("approves a pending request and the user becomes a member", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invite = await createInvite(owner.access_token, domain.uuid);
    await setRequireApproval(owner.access_token, domain.uuid, true);

    const username = unique("approvee");
    const reg = await api<{ user: { uuid: string }; access_token: string }>(
      "/api/v1/invite/register",
      { body: { code: invite.code, username, password: "password123" } },
    );
    const registered = assertSuccess(reg);

    const list = await api<{ items: Array<{ id: number; user_uuid: string }> }>(
      "/api/v1/domain/join-requests/list",
      { token: owner.access_token, body: { domain_uuid: domain.uuid, status: "pending" } },
    );
    const request = list.data?.items.find((item) => item.user_uuid === registered.user.uuid);
    expect(request).toBeTruthy();

    const review = await api("/api/v1/domain/join-requests/review", {
      token: owner.access_token,
      body: { domain_uuid: domain.uuid, request_id: request!.id, approve: true },
    });
    expect(review.code).toBe(0);

    const mine = await api<{ status: string; is_member: boolean }>("/api/v1/invite/my-request", {
      token: registered.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(mine.data?.is_member).toBe(true);
    expect(mine.data?.status).toBe("approved");
  });

  // 真过期需要等到 expires_at 过去，集成测试无法在合理时间内构造；
  // 过期分支由 service/invite_join_service_test.go 的
  // TestInviteJoinService_RegisterAndJoin_ExpiredCodeCreatesNoUser 覆盖。
  // 这里覆盖「无效码不建用户、不泄露域信息」。
  it("rejects an invalid invite code without creating a user", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));

    const res = await api("/api/v1/invite/register", {
      body: { code: "A".repeat(36), username: unique("bad"), password: "password123" },
    });
    expect(res.code).toBe(2001);
    expect(res.raw.includes(domain.name)).toBe(false);
  });

  it("enforces the usage limit", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invite = await createInvite(owner.access_token, domain.uuid, { max_uses: 1 });

    const first = await api("/api/v1/invite/register", {
      body: { code: invite.code, username: unique("first"), password: "password123" },
    });
    expect(first.code).toBe(0);

    const second = await api("/api/v1/invite/register", {
      body: { code: invite.code, username: unique("second"), password: "password123" },
    });
    expect(second.code).toBe(2001);
  });

  it("keeps domains isolated", async () => {
    const ownerA = await registerUser("inv_a");
    const ownerB = await registerUser("inv_b");
    const domainA = await createDomain(ownerA.access_token, unique("inv_a"));
    const domainB = await createDomain(ownerB.access_token, unique("inv_b"));
    const inviteA = await createInvite(ownerA.access_token, domainA.uuid);

    const res = await api<{ domain: { uuid: string } }>("/api/v1/invite/register", {
      body: { code: inviteA.code, username: unique("cross"), password: "password123" },
    });
    const joined = assertSuccess(res);
    expect(joined.domain.uuid).toBe(domainA.uuid);
    expect(joined.domain.uuid).not.toBe(domainB.uuid);
  });

  it("lets a logged-in user join a public domain without a code", async () => {
    const owner = await registerUser("inv_pub");
    const created = await api<{ uuid: string }>("/api/v1/domain/create", {
      token: owner.access_token,
      body: { name: unique("pub_domain"), is_public: true },
    });
    const domain = assertSuccess(created);
    const joiner = await registerUser("inv_joiner");

    const res = await api<{ status: string; domain: { uuid: string } }>("/api/v1/invite/join", {
      token: joiner.access_token,
      body: { domain_uuid: domain.uuid },
    });
    const data = assertSuccess(res);
    expect(data.status).toBe("joined");
    expect(data.domain.uuid).toBe(domain.uuid);
  });

  it("puts public-domain joins into pending when approval is on", async () => {
    const owner = await registerUser("inv_pub2");
    const created = await api<{ uuid: string }>("/api/v1/domain/create", {
      token: owner.access_token,
      body: { name: unique("pub_domain"), is_public: true },
    });
    const domain = assertSuccess(created);
    await setRequireApproval(owner.access_token, domain.uuid, true);
    const joiner = await registerUser("inv_joiner2");

    const res = await api<{ status: string }>("/api/v1/invite/join", {
      token: joiner.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(assertSuccess(res).status).toBe("pending");
  });

  it("refuses code-less joins to private domains", async () => {
    const owner = await registerUser("inv_priv");
    const domain = await createDomain(owner.access_token, unique("priv_domain"));
    const joiner = await registerUser("inv_joiner3");

    const res = await api("/api/v1/invite/join", {
      token: joiner.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(res.code).toBe(2001);
  });

  it("requires JWT for list-public (discover is not anonymous)", async () => {
    const res = await api("/api/v1/domain/list-public", { body: {} });
    expect(res.code).toBe(1001);
  });
});
