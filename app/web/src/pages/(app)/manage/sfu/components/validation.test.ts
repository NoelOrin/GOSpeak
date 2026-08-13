import { describe, expect, it } from "vitest";
import { cleanForm, validateSFUForm } from "./validation";

const noStoredSecrets = {
	livekit_secret_set: false,
	agora_app_certificate_set: false,
	agora_customer_secret_set: false,
	srs_secret_set: false,
	cf_app_secret_set: false,
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

	it("validates srs host, port, and optional URLs", () => {
		expect(
			validateSFUForm(
				{
					provider: "srs",
					srs_host: "https://bad",
					srs_api_port: "0",
					srs_secret: "",
					srs_public_host: "not-a-url",
					srs_whip_url: "ftp://bad",
				},
				noStoredSecrets,
			),
		).toEqual({
			srs_host: "需要域名或 IP, 不含 scheme / 路径 / 引号",
			srs_api_port: "1-65535 数字",
			srs_secret: "Secret 必填",
			srs_public_host: "需要 http(s)/ws(s) URL，或留空",
			srs_whip_url: "需要绝对路径或 http(s) URL",
		});
	});

	it("accepts a valid srs config", () => {
		expect(
			validateSFUForm(
				{
					provider: "srs",
					srs_host: "localhost",
					srs_api_port: "1985",
					srs_secret: "secret",
					srs_public_host: "",
					srs_whip_url: "/rtc/v1/whip/",
				},
				noStoredSecrets,
			),
		).toEqual({});
	});
});

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
