import { describe, expect, it } from "vitest";
import { cleanForm, validateSFUForm } from "./validation";

const noStoredSecrets = {
	livekit_secret_set: false,
};

describe("validateSFUForm", () => {
	it("requires livekit host and credentials", () => {
		expect(
			validateSFUForm(
				{
					provider: "livekit",
					livekit_host: "",
					livekit_key: "",
					livekit_secret: "",
				},
				noStoredSecrets,
			),
		).toEqual({
			livekit_host: "需要 ws:// 或 wss:// 开头的合法 URL",
			livekit_key: "API Key 必填",
			livekit_secret: "API Secret 必填",
		});
	});

	it("allows an empty livekit secret when a secret is already stored", () => {
		expect(
			validateSFUForm(
				{
					provider: "livekit",
					livekit_host: "wss://livekit.example.com",
					livekit_key: "key",
					livekit_secret: "",
				},
				{ ...noStoredSecrets, livekit_secret_set: true },
			),
		).toEqual({});
	});
});

describe("cleanForm", () => {
	it("only keeps current provider fields", () => {
		const payload = cleanForm({
			provider: "livekit",
			livekit_host: " wss://livekit.example.com ",
			livekit_key: " key ",
			livekit_secret: " secret ",
		});

		expect(payload).toEqual({
			provider: "livekit",
			livekit_host: "wss://livekit.example.com",
			livekit_key: "key",
			livekit_secret: "secret",
		});
	});
});
