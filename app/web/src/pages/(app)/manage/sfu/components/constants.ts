import { DEFAULT_SFU_PROVIDER } from "@gospeak/sfu-client";
import type { SFUProvider } from "@gospeak/sfu-client/types";
import type { SFUConfig, UpdateSFUConfigParams } from "@/api/sfu";

export const emptyForm: UpdateSFUConfigParams = {
	provider: DEFAULT_SFU_PROVIDER,
	livekit_host: "",
	livekit_key: "",
	livekit_secret: "",
	agora_app_id: "",
	agora_app_certificate: "",
	agora_host: "",
	agora_customer_id: "",
	agora_customer_secret: "",
	srs_host: "",
	srs_api_port: "1985",
	srs_secret: "",
	srs_whip_url: "",
	srs_public_host: "",
	cf_app_id: "",
	cf_app_secret: "",
	cf_stun_url: "stun.cloudflare.com:3478",
};

/** 每个 provider 仅提交/缓存自己的字段，避免“一个大表单塞所有参数”。 */
export const PROVIDER_FIELD_KEYS: Record<
	SFUProvider,
	ReadonlyArray<keyof UpdateSFUConfigParams>
> = {
	livekit: ["livekit_host", "livekit_key", "livekit_secret"],
	agora: [
		"agora_app_id",
		"agora_app_certificate",
		"agora_host",
		"agora_customer_id",
		"agora_customer_secret",
	],
	srs: [
		"srs_host",
		"srs_api_port",
		"srs_secret",
		"srs_whip_url",
		"srs_public_host",
	],
	cloudflare: ["cf_app_id", "cf_app_secret", "cf_stun_url"],
};

export const PROVIDER_OPTIONS: { value: SFUProvider; label: string }[] = [
	{ value: "livekit", label: "LiveKit" },
	{ value: "agora", label: "Agora" },
	{ value: "srs", label: "SRS" },
	{ value: "cloudflare", label: "Cloudflare" },
];

export function emptyFormForProvider(
	provider: SFUProvider,
): UpdateSFUConfigParams {
	return { ...emptyForm, provider };
}

export function pickProviderForm(
	provider: SFUProvider,
	source: Partial<UpdateSFUConfigParams> | Partial<SFUConfig> | undefined,
): UpdateSFUConfigParams {
	const next = emptyFormForProvider(provider);
	if (!source) return next;
	for (const key of PROVIDER_FIELD_KEYS[provider]) {
		const value = source[key];
		if (typeof value === "string") {
			next[key] = value as never;
		}
	}
	return next;
}

export function isProviderConfigured(
	provider: SFUProvider,
	config: Partial<SFUConfig> & { provider?: SFUProvider },
): boolean {
	switch (provider) {
		case "livekit":
			return !!config.livekit_host;
		case "agora":
			return !!config.agora_app_id;
		case "srs":
			return !!config.srs_host;
		case "cloudflare":
			return !!config.cf_app_id;
		default:
			return false;
	}
}

export type SecretFlags = {
	livekit_secret_set: boolean;
	agora_app_certificate_set: boolean;
	agora_customer_secret_set: boolean;
	srs_secret_set: boolean;
	cf_app_secret_set: boolean;
};

export function emptySecretFlags(): SecretFlags {
	return {
		livekit_secret_set: false,
		agora_app_certificate_set: false,
		agora_customer_secret_set: false,
		srs_secret_set: false,
		cf_app_secret_set: false,
	};
}

export function secretFlagsFromConfig(
	data: Partial<SFUConfig> | undefined,
): SecretFlags {
	return {
		livekit_secret_set: !!data?.livekit_secret_set,
		agora_app_certificate_set: !!data?.agora_app_certificate_set,
		agora_customer_secret_set: !!data?.agora_customer_secret_set,
		srs_secret_set: !!data?.srs_secret_set,
		cf_app_secret_set: !!data?.cf_app_secret_set,
	};
}
