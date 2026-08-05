import { describe, expect, it } from "vitest";
import { api, registerUser, unique } from "../helpers";

describe("signal module", () => {
  it("relays a public signal request", async () => {
    const result = await api<{ type: string; room: string }>("/api/v1/signal/signal", {
      body: { type: "offer", room: unique("signal_room"), data: {} },
    });
    expect(result.code).toBe(0);
    expect(result.data?.type).toBe("offer");
  });

  it("issues a websocket ticket for an authenticated user", async () => {
    const user = await registerUser("signal");
    const result = await api<{ ticket: string }>("/api/v1/signal/ws-ticket", {
      method: "GET",
      token: user.access_token,
    });
    expect(result.code).toBe(0);
    expect(result.data?.ticket).toBeTruthy();
  });

  it("generates an srs join token", async () => {
    const user = await registerUser("signal");
    const room = unique("voice_room");
    const result = await api<{ token: string; provider: string; sfuRoom: string }>("/api/v1/signal/token", {
      token: user.access_token,
      body: { room, domain_uuid: "", identity: "ignored" },
    });
    expect(result.code).toBe(0);
    expect(result.data?.token).toBeTruthy();
    expect(result.data?.provider).toBe("srs");
    expect(result.data?.sfuRoom).toBe(room);
  });
});
