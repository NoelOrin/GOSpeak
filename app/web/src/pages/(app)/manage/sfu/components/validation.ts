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
