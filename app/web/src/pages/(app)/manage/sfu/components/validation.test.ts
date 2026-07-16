import { describe, expect, it } from "vitest";
import { cleanForm } from "./validation";

describe("cleanForm", () => {
	it("only keeps current provider fields", () => {
		const payload = cleanForm({
			provider: "srs",
			srs_host: " localhost ",
			srs_api_port: "1985",
			srs_secret: "",
			srs_whip_url: " /rtc/v1/whip/ ",
			srs_public_host: "",
			livekit_host: "wss://should-not-submit",
			agora_app_id: "should-not-submit",
		});

		expect(payload).toEqual({
			provider: "srs",
			srs_host: "localhost",
			srs_api_port: "1985",
			srs_secret: "",
			srs_whip_url: "/rtc/v1/whip/",
			srs_public_host: "",
		});
		expect(payload).not.toHaveProperty("livekit_host");
		expect(payload).not.toHaveProperty("agora_app_id");
	});
});
