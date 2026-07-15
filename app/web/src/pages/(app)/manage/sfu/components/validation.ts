import type { UpdateSFUConfigParams } from "@/api/sfu";
import type { SecretFlags } from "./constants";

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

export function validateSFUForm(
	form: UpdateSFUConfigParams,
	flags: SecretFlags,
): FieldErrors {
	const f = form;
	const e: FieldErrors = {};
	const p = f.provider;

	const require = (key: keyof UpdateSFUConfigParams, msg: string) => {
		if (!clean(String(f[key] ?? ""))) e[key] = msg;
	};
	const requireSecret = (
		key: keyof UpdateSFUConfigParams,
		set: boolean,
		msg: string,
	) => {
		if (!set && !clean(String(f[key] ?? ""))) e[key] = msg;
	};

	if (p === "livekit") {
		if (!isUrl(f.livekit_host, ["ws", "wss"]))
			e.livekit_host = "需要 ws:// 或 wss:// 开头的合法 URL";
		require("livekit_key", "API Key 必填");
		requireSecret(
			"livekit_secret",
			flags.livekit_secret_set,
			"API Secret 必填",
		);
	} else if (p === "agora") {
		require("agora_app_id", "App ID 必填");
		requireSecret(
			"agora_app_certificate",
			flags.agora_app_certificate_set,
			"App Certificate 必填",
		);
		require("agora_customer_id", "Customer ID 必填");
		requireSecret(
			"agora_customer_secret",
			flags.agora_customer_secret_set,
			"Customer Secret 必填",
		);
		if (f.agora_host && !isUrl(f.agora_host, ["http", "https"]))
			e.agora_host = "需要 http(s):// 开头的合法 URL";
	} else if (p === "mediasoup") {
		if (!isUrl(f.mediasoup_bridge_url, ["http", "https"]))
			e.mediasoup_bridge_url = "需要 http(s):// 开头的合法 URL";
		if (!isUrl(f.mediasoup_host, ["ws", "wss"]))
			e.mediasoup_host = "需要 ws:// 或 wss:// 开头的合法 URL";
	} else if (p === "srs") {
		if (!isHost(f.srs_host))
			e.srs_host = "需要域名或 IP, 不含 scheme / 路径 / 引号";
		if (!isPort(f.srs_api_port)) e.srs_api_port = "1-65535 数字";
		requireSecret("srs_secret", flags.srs_secret_set, "Secret 必填");
		if (
			f.srs_public_host &&
			!isUrl(f.srs_public_host, ["http", "https", "ws", "wss"])
		)
			e.srs_public_host = "需要 http(s)/ws(s) URL，或留空";
		if (
			f.srs_whip_url &&
			!(
				f.srs_whip_url.startsWith("/") ||
				isUrl(f.srs_whip_url, ["http", "https"])
			)
		)
			e.srs_whip_url = "需要绝对路径或 http(s) URL";
	} else if (p === "daily") {
		requireSecret("daily_api_key", flags.daily_api_key_set, "API Key 必填");
		if (!isHost(f.daily_domain))
			e.daily_domain = "需要域名, 不含 scheme / 路径 / 引号";
	} else if (p === "cloudflare") {
		require("cf_app_id", "App ID 必填");
		requireSecret("cf_app_secret", flags.cf_app_secret_set, "App Secret 必填");
	}
	return e;
}

export function cleanForm(form: UpdateSFUConfigParams): UpdateSFUConfigParams {
	const cleaned = { ...form };
	for (const k of Object.keys(cleaned) as (keyof UpdateSFUConfigParams)[]) {
		if (typeof cleaned[k] === "string") {
			cleaned[k] = clean(cleaned[k] as string) as never;
		}
	}
	return cleaned;
}
