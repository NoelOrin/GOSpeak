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
	mediasoup_bridge_url: "",
	mediasoup_host: "",
	srs_host: "",
	srs_api_port: "1985",
	srs_secret: "",
	srs_whip_url: "",
	srs_public_host: "",
	daily_api_key: "",
	daily_domain: "",
	cf_app_id: "",
	cf_app_secret: "",
	cf_stun_url: "stun.cloudflare.com:3478",
};

export const PROVIDER_OPTIONS: { value: SFUProvider; label: string }[] = [
	{ value: "livekit", label: "LiveKit" },
	{ value: "agora", label: "Agora" },
	{ value: "mediasoup", label: "MediaSoup" },
	{ value: "srs", label: "SRS" },
	{ value: "daily", label: "Daily" },
	{ value: "cloudflare", label: "Cloudflare" },
];

export const DISABLED_PROVIDERS: SFUProvider[] = ["mediasoup"];

export function isProviderConfigured(
	provider: SFUProvider,
	config: Partial<SFUConfig> & { provider?: SFUProvider },
): boolean {
	switch (provider) {
		case "livekit":
			return !!config.livekit_host;
		case "agora":
			return !!config.agora_app_id;
		case "mediasoup":
			return !!(config.mediasoup_bridge_url || config.mediasoup_host);
		case "srs":
			return !!config.srs_host;
		case "daily":
			return !!(config.daily_api_key_set || config.daily_domain);
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
	daily_api_key_set: boolean;
	cf_app_secret_set: boolean;
};

export const emptySecretFlags = (): SecretFlags => ({
	livekit_secret_set: false,
	agora_app_certificate_set: false,
	agora_customer_secret_set: false,
	srs_secret_set: false,
	daily_api_key_set: false,
	cf_app_secret_set: false,
});
