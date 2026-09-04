import { describe, expect, it } from "vitest";
import { api, createDomain, createRoom, joinDomain, registerUser, unique } from "../helpers";

interface MessageData {
  uuid: string;
  room_uuid: string;
  author_id: string;
  content: string;
  edited_at?: string | null;
}

describe("message module", () => {
  it("sends, lists, edits, reacts and deletes room messages", async () => {
    const author = await registerUser("message");
    const reader = await registerUser("message");
    const domain = await createDomain(author.access_token, unique("message_domain"));
    await joinDomain(reader.access_token, domain.invite_code);
    const room = await createRoom(author.access_token, domain.uuid, unique("text_room"), "text");

    const sent = await api<MessageData>("/api/v1/room/messages/send", {
      token: author.access_token,
      body: { room_uuid: room.uuid, content: "hello integration" },
    });
    expect(sent.code).toBe(0);
    const message = sent.data!;
    expect(message.uuid).toBeTruthy();
    expect(message.author_id).toBe(author.username);

    const listed = await api<{ items: MessageData[] }>("/api/v1/room/messages/list", {
      token: reader.access_token,
      body: { room_uuid: room.uuid, limit: 20 },
    });
    expect(listed.code).toBe(0);
    expect(listed.data?.items.some((item) => item.uuid === message.uuid)).toBe(true);

    const edited = await api<MessageData>("/api/v1/room/messages/edit", {
      token: author.access_token,
      body: { room_uuid: room.uuid, message_uuid: message.uuid, content: "edited integration" },
    });
    expect(edited.code).toBe(0);
    expect(edited.data?.content).toBe("edited integration");

    const reacted = await api("/api/v1/room/messages/react", {
      token: reader.access_token,
      body: { room_uuid: room.uuid, message_uuid: message.uuid, emoji: "+1" },
    });
    expect(reacted.code).toBe(0);

    const unreacted = await api("/api/v1/room/messages/unreact", {
      token: reader.access_token,
      body: { room_uuid: room.uuid, message_uuid: message.uuid, emoji: "+1" },
    });
    expect(unreacted.code).toBe(0);

    const forbiddenDelete = await api("/api/v1/room/messages/delete", {
      token: reader.access_token,
      body: { room_uuid: room.uuid, message_uuid: message.uuid },
    });
    expect(forbiddenDelete.code).toBe(1013);

    const deleted = await api("/api/v1/room/messages/delete", {
      token: author.access_token,
      body: { room_uuid: room.uuid, message_uuid: message.uuid },
    });
    expect(deleted.code).toBe(0);
  });
});
