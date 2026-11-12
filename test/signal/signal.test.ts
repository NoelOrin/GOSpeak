import { describe, expect, it } from "vitest";
import { api, registerUser, unique } from "../helpers";

describe("signal module", () => {
	it("issues a join token for an authenticated user", async () => {
		const user = await registerUser("signal");
		const room = unique("voice_room");
		const result = await api<{
			token: string;
			provider: string;
			sfuRoom: string;
		}>("/api/v1/signal/token", {
			token: user.access_token,
			body: { room, domain_uuid: "", identity: "ignored" },
		});
		expect(result.code).toBe(0);
		expect(result.data?.token).toBeTruthy();
	});

	it("generates a livekit join token", async () => {
		const user = await registerUser("signal");
		const room = unique("voice_room");
		const result = await api<{
			token: string;
			provider: string;
			sfuRoom: string;
		}>("/api/v1/signal/token", {
			token: user.access_token,
			body: { room, domain_uuid: "", identity: "ignored" },
		});
		expect(result.code).toBe(0);
		expect(result.data?.token).toBeTruthy();
		expect(result.data?.provider).toBe("livekit");
		expect(result.data?.sfuRoom).toBe(room);
	});
});
