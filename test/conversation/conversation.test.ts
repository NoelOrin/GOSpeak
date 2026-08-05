import { describe, expect, it } from "vitest";
import { api, registerUser } from "../helpers";

describe("conversation module", () => {
  it("sends, lists, reads and marks private messages read", async () => {
    const sender = await registerUser("conv");
    const receiver = await registerUser("conv");

    const sent = await api<{ conversation_id: string; content: string; target_identity: string }>(
      "/api/v1/conversation/send",
      {
        token: sender.access_token,
        body: { target_identity: receiver.username, content: "private hello" },
      },
    );
    expect(sent.code).toBe(0);
    const conversationId = sent.data!.conversation_id;
    expect(conversationId).toBeTruthy();
    expect(sent.data?.target_identity).toBe(receiver.username);

    const receiverList = await api<Array<{ conversation_id: string; other_identity: string; unread_count: number }>>(
      "/api/v1/conversation/list",
      { token: receiver.access_token, body: {} },
    );
    const conversation = receiverList.data?.find((item) => item.conversation_id === conversationId);
    expect(conversation?.other_identity).toBe(sender.username);
    expect(conversation?.unread_count).toBe(1);

    const messages = await api<{ messages: Array<{ content: string; author_id: string }> }>(
      "/api/v1/conversation/messages",
      {
        token: receiver.access_token,
        body: { conversation_id: conversationId },
      },
    );
    expect(messages.code).toBe(0);
    expect(messages.data?.messages.some((item) => item.content === "private hello")).toBe(true);

    const markedRead = await api("/api/v1/conversation/mark-read", {
      token: receiver.access_token,
      body: { conversation_id: conversationId },
    });
    expect(markedRead.code).toBe(0);

    const afterRead = await api<Array<{ conversation_id: string; unread_count: number }>>(
      "/api/v1/conversation/list",
      { token: receiver.access_token, body: {} },
    );
    expect(afterRead.data?.find((item) => item.conversation_id === conversationId)?.unread_count).toBe(0);
  });
});
