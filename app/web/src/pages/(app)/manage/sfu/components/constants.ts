import { DEFAULT_SFU_PROVIDER } from "@gospeak/sfu-client";
import type { SFUProvider } from "@gospeak/sfu-client/types";
import type { SFUConfig, UpdateSFUConfigParams } from "@/api/sfu";

export const emptyForm: UpdateSFUConfigParams = {
	provider: DEFAULT_SFU_PROVIDER,
	livekit_host: "",
	livekit_key: "",
	livekit_secret: "",
};

/** 每个 provider 仅提交/缓存自己的字段，避免“一个大表单塞所有参数”。 */
export const PROVIDER_FIELD_KEYS: Record<
	SFUProvider,
	ReadonlyArray<keyof UpdateSFUConfigParams>
> = {
	livekit: ["livekit_host", "livekit_key", "livekit_secret"],
};

export const PROVIDER_OPTIONS: { value: SFUProvider; label: string }[] = [
	{ value: "livekit", label: "LiveKit" },
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
		default:
			return false;
	}
}

export type SecretFlags = {
	livekit_secret_set: boolean;
};

export function emptySecretFlags(): SecretFlags {
	return {
		livekit_secret_set: false,
	};
}

export function secretFlagsFromConfig(
	data: Partial<SFUConfig> | undefined,
): SecretFlags {
	return {
		livekit_secret_set: !!data?.livekit_secret_set,
	};
}
