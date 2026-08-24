import { describe, expect, it } from "vitest";
import { api, createDomain, createRoom, registerUser, unique } from "../helpers";

describe("room module", () => {
  it("creates, lists, updates and deletes a domain room", async () => {
    const owner = await registerUser("room");
    const domain = await createDomain(owner.access_token, unique("room_domain"));
    const roomName = unique("voice_room");
    const room = await createRoom(owner.access_token, domain.uuid, roomName, "voice");
    expect(room.id).toBeGreaterThan(0);
    expect(room.domain_uuid).toBe(domain.uuid);

    const get = await api<{ id: number; name: string; created_by: string }>("/api/v1/room/get", {
      token: owner.access_token,
      body: { id: room.id },
    });
    expect(get.code).toBe(0);
    expect(get.data?.created_by).toBe(owner.username);

    const list = await api<{ rooms: Array<{ id: number; name: string }>; total: number }>("/api/v1/room/list", {
      token: owner.access_token,
      body: { page: 1, page_size: 50, domain_uuid: domain.uuid },
    });
    expect(list.code).toBe(0);
    expect(list.data?.rooms.some((item) => item.id === room.id)).toBe(true);

    const duplicate = await api("/api/v1/room/create", {
      token: owner.access_token,
      body: {
        name: roomName,
        domain_uuid: domain.uuid,
        type: "voice",
      },
    });
    expect(duplicate.code).toBe(3002);

    // 域房间管理权归属创建者或域内角色：房间的创建者（owner）本身即是域成员且为创建者，
    // 用其 token 做更新/删除，符合 handler 中 canManageRoom 的 CreatedBy 判定。
    const updated = await api<{ name: string }>("/api/v1/room/update", {
      token: owner.access_token,
      body: { id: room.id, name: `${roomName}_updated` },
    });
    expect(updated.code).toBe(0);
    expect(updated.data?.name.endsWith("_updated")).toBe(true);

    const deleted = await api("/api/v1/room/delete", {
      token: owner.access_token,
      body: { id: room.id },
    });
    expect(deleted.code).toBe(0);

    const afterDelete = await api("/api/v1/room/get", {
      token: owner.access_token,
      body: { id: room.id },
    });
    expect(afterDelete.code).toBe(3001);
  });

  it("rejects room creation outside the user's domain", async () => {
    const owner = await registerUser("room");
    const outsider = await registerUser("room");
    const domain = await createDomain(owner.access_token, unique("room_domain"));

    const result = await api("/api/v1/room/create", {
      token: outsider.access_token,
      body: {
        name: unique("blocked_room"),
        domain_uuid: domain.uuid,
        type: "voice",
      },
    });
    expect(result.code).toBe(1013);
  });
});
