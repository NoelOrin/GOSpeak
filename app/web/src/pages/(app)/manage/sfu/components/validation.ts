import type { UpdateSFUConfigParams } from "@/api/sfu";
import { PROVIDER_FIELD_KEYS, type SecretFlags } from "./constants";
import { z } from "zod";

export type FieldErrors = Partial<Record<keyof UpdateSFUConfigParams, string>>;

export const clean = (v: string): string =>
	v.trim().replace(/^["'\\\s]+|["'\\\s]+$/g, "");

export const isPort = (v: string): boolean => {
	const n = Number(v);
	return /^\d+$/.test(v.trim()) && n >= 1 && n <= 65535;
};

export const isUrl = (v: string, schemes: string[]): boolean => {
	const s = clean(v);
	if (!s) return false;
	try {
		const u = new URL(s);
		return schemes.includes(u.protocol.replace(":", ""));
	} catch {
		return false;
	}
};

export const isHost = (v: string): boolean => {
	const s = clean(v);
	if (!s) return false;
	if (/[/\\]"'/.test(s)) return false;
	if (s.includes("://")) return false;
	if (s.includes("/")) return false;
	return /^[a-zA-Z0-9.\-_:]+$/.test(s);
};

const requiredString = (message: string) =>
	z.unknown().refine((value) => clean(String(value ?? "")) !== "", message);

const secretString = (set: boolean, message: string) =>
	z
		.unknown()
		.refine((value) => set || clean(String(value ?? "")) !== "", message);

const urlString = (schemes: string[], message: string) =>
	z.unknown().refine((value) => isUrl(String(value ?? ""), schemes), message);

const optionalUrlString = (schemes: string[], message: string) =>
	z
		.unknown()
		.refine((value) => !value || isUrl(String(value), schemes), message);

function sfuSchema(form: UpdateSFUConfigParams, flags: SecretFlags) {
	switch (form.provider) {
		case "livekit":
			return z.object({
				provider: z.literal("livekit"),
				livekit_host: urlString(
					["ws", "wss"],
					"需要 ws:// 或 wss:// 开头的合法 URL",
				),
				livekit_key: requiredString("API Key 必填"),
				livekit_secret: secretString(
					flags.livekit_secret_set,
					"API Secret 必填",
				),
			});
		case "agora":
			return z.object({
				provider: z.literal("agora"),
				agora_app_id: requiredString("App ID 必填"),
				agora_app_certificate: secretString(
					flags.agora_app_certificate_set,
					"App Certificate 必填",
				),
				agora_customer_id: requiredString("Customer ID 必填"),
				agora_customer_secret: secretString(
					flags.agora_customer_secret_set,
					"Customer Secret 必填",
				),
				agora_host: optionalUrlString(
					["http", "https"],
					"需要 http(s):// 开头的合法 URL",
				),
			});
		case "srs":
			return z.object({
				provider: z.literal("srs"),
				srs_host: z
					.unknown()
					.refine(
						(value) => isHost(String(value ?? "")),
						"需要域名或 IP, 不含 scheme / 路径 / 引号",
					),
				srs_api_port: z
					.unknown()
					.refine((value) => isPort(String(value ?? "")), "1-65535 数字"),
				srs_secret: secretString(flags.srs_secret_set, "Secret 必填"),
				srs_public_host: z
					.unknown()
					.refine(
						(value) =>
							!value || isUrl(String(value), ["http", "https", "ws", "wss"]),
						"需要 http(s)/ws(s) URL，或留空",
					),
				srs_whip_url: z
					.unknown()
					.refine(
						(value) =>
							!value ||
							String(value).startsWith("/") ||
							isUrl(String(value), ["http", "https"]),
						"需要绝对路径或 http(s) URL",
					),
			});
		case "cloudflare":
			return z.object({
				provider: z.literal("cloudflare"),
				cf_app_id: requiredString("App ID 必填"),
				cf_app_secret: secretString(flags.cf_app_secret_set, "App Secret 必填"),
			});
		default:
			return z.object({ provider: z.string() });
	}
}

export function validateSFUForm(
	form: UpdateSFUConfigParams,
	flags: SecretFlags,
): FieldErrors {
	const result = sfuSchema(form, flags).safeParse(form);
	if (result.success) return {};

	const errors: FieldErrors = {};
	for (const issue of result.error.issues) {
		const key = issue.path[0] as keyof UpdateSFUConfigParams | undefined;
		if (key === undefined || key === "provider") continue;
		if (!errors[key]) errors[key] = issue.message;
	}
	return errors;
}

/** 仅清洗并保留当前 provider 的字段，提交体不再夹带其他 SFU 参数。 */
export function cleanForm(form: UpdateSFUConfigParams): UpdateSFUConfigParams {
	const provider = form.provider;
	const cleaned: UpdateSFUConfigParams = { provider };
	for (const key of PROVIDER_FIELD_KEYS[provider]) {
		const value = form[key];
		cleaned[key] = (typeof value === "string" ? clean(value) : value) as never;
	}
	return cleaned;
}
