import type { SFUProvider } from "@gospeak/sfu-client/types";
import { Show } from "solid-js";
import type { UpdateSFUConfigParams } from "@/api/sfu";
import FormField from "@/components/form/FormField";
import type { SecretFlags } from "./constants";
import type { FieldErrors } from "./validation";

export interface ProviderConfigFormProps {
	provider: SFUProvider;
	form: UpdateSFUConfigParams;
	errors: FieldErrors;
	secretFlags: SecretFlags;
	saving: boolean;
	updateField: <K extends keyof UpdateSFUConfigParams>(
		key: K,
		value: UpdateSFUConfigParams[K],
	) => void;
}

export default function ProviderConfigForm(props: ProviderConfigFormProps) {
	const form = () => props.form;
	const errors = () => props.errors;
	const secretFlags = () => props.secretFlags;
	const saving = () => props.saving;
	const updateField = props.updateField;
	const selectedProvider = () => props.provider;

	return (
		<>
			{/* LiveKit config fields */}
			<Show when={selectedProvider() === "livekit"}>
				<div class="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
					<FormField label="Host" error={errors().livekit_host}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().livekit_host,
							}}
							placeholder="wss://livekit.example.com"
							value={form().livekit_host ?? ""}
							onInput={(event) =>
								updateField("livekit_host", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="API Key" error={errors().livekit_key}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().livekit_key,
							}}
							placeholder="API key"
							value={form().livekit_key ?? ""}
							onInput={(event) =>
								updateField("livekit_key", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="API Secret" error={errors().livekit_secret}>
						<input
							type="password"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().livekit_secret,
							}}
							placeholder={
								secretFlags().livekit_secret_set
									? "已配置，留空保留"
									: "API secret"
							}
							value={form().livekit_secret ?? ""}
							onInput={(event) =>
								updateField("livekit_secret", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
				</div>
			</Show>

			{/* Agora config fields */}
			<Show when={selectedProvider() === "agora"}>
				<div class="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
					<FormField label="App ID" error={errors().agora_app_id}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().agora_app_id,
							}}
							placeholder="Agora App ID"
							value={form().agora_app_id ?? ""}
							onInput={(event) =>
								updateField("agora_app_id", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="REST Host" error={errors().agora_host}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().agora_host,
							}}
							placeholder="https://api.agora.io"
							value={form().agora_host ?? ""}
							onInput={(event) =>
								updateField("agora_host", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField
						label="App Certificate"
						error={errors().agora_app_certificate}
					>
						<input
							type="password"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().agora_app_certificate,
							}}
							placeholder={
								secretFlags().agora_app_certificate_set
									? "已配置，留空保留"
									: "App certificate"
							}
							value={form().agora_app_certificate ?? ""}
							onInput={(event) =>
								updateField("agora_app_certificate", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField
						label="Customer Secret"
						error={errors().agora_customer_secret}
					>
						<input
							type="password"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().agora_customer_secret,
							}}
							placeholder={
								secretFlags().agora_customer_secret_set
									? "已配置，留空保留"
									: "Customer secret"
							}
							value={form().agora_customer_secret ?? ""}
							onInput={(event) =>
								updateField("agora_customer_secret", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="Customer ID" error={errors().agora_customer_id}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().agora_customer_id,
							}}
							placeholder="Customer ID"
							value={form().agora_customer_id ?? ""}
							onInput={(event) =>
								updateField("agora_customer_id", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
				</div>
			</Show>

			{/* MediaSoup config fields */}
			<Show when={selectedProvider() === "mediasoup"}>
				<div class="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
					<FormField label="Bridge URL" error={errors().mediasoup_bridge_url}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().mediasoup_bridge_url,
							}}
							placeholder="https://mediasoup-bridge.example.com"
							value={form().mediasoup_bridge_url ?? ""}
							onInput={(event) =>
								updateField("mediasoup_bridge_url", event.currentTarget.value)
							}
							disabled={true}
						/>
					</FormField>
					<FormField label="Host" error={errors().mediasoup_host}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().mediasoup_host,
							}}
							placeholder="wss://mediasoup.example.com"
							value={form().mediasoup_host ?? ""}
							onInput={(event) =>
								updateField("mediasoup_host", event.currentTarget.value)
							}
							disabled={true}
						/>
					</FormField>
				</div>
			</Show>

			{/* SRS config fields */}
			<Show when={selectedProvider() === "srs"}>
				<div class="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
					<FormField label="Host" error={errors().srs_host}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().srs_host,
							}}
							placeholder="srs.example.com"
							value={form().srs_host ?? ""}
							onInput={(event) =>
								updateField("srs_host", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="API Port" error={errors().srs_api_port}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().srs_api_port,
							}}
							placeholder="1985"
							value={form().srs_api_port ?? ""}
							onInput={(event) =>
								updateField("srs_api_port", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="Secret" error={errors().srs_secret}>
						<input
							type="password"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().srs_secret,
							}}
							placeholder={
								secretFlags().srs_secret_set
									? "已配置，留空保留"
									: "Bearer secret"
							}
							value={form().srs_secret ?? ""}
							onInput={(event) =>
								updateField("srs_secret", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="WHIP URL" error={errors().srs_whip_url}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().srs_whip_url,
							}}
							placeholder="/rtc/v1/whip/ 或 https://srs.example.com/rtc/v1/whip/"
							value={form().srs_whip_url ?? ""}
							onInput={(event) =>
								updateField("srs_whip_url", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="Public Host" error={errors().srs_public_host}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().srs_public_host,
							}}
							placeholder="https://voice.example.com 或留空使用 Host"
							value={form().srs_public_host ?? ""}
							onInput={(event) =>
								updateField("srs_public_host", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
				</div>
			</Show>

			{/* Daily config fields */}
			<Show when={selectedProvider() === "daily"}>
				<div class="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
					<FormField label="API Key" error={errors().daily_api_key}>
						<input
							type="password"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().daily_api_key,
							}}
							placeholder={
								secretFlags().daily_api_key_set
									? "已配置，留空保留"
									: "Daily API key"
							}
							value={form().daily_api_key ?? ""}
							onInput={(event) =>
								updateField("daily_api_key", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="Domain" error={errors().daily_domain}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().daily_domain,
							}}
							placeholder="your-team.daily.co"
							value={form().daily_domain ?? ""}
							onInput={(event) =>
								updateField("daily_domain", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
				</div>
			</Show>

			{/* Cloudflare Realtime config fields */}
			<Show when={selectedProvider() === "cloudflare"}>
				<div class="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
					<FormField label="App ID" error={errors().cf_app_id}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().cf_app_id,
							}}
							placeholder="Cloudflare Realtime App ID"
							value={form().cf_app_id ?? ""}
							onInput={(event) =>
								updateField("cf_app_id", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="STUN URL" error={errors().cf_stun_url}>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().cf_stun_url,
							}}
							placeholder="stun.cloudflare.com:3478"
							value={form().cf_stun_url ?? ""}
							onInput={(event) =>
								updateField("cf_stun_url", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
					<FormField label="App Secret" error={errors().cf_app_secret}>
						<input
							type="password"
							class="input input-bordered input-sm w-full"
							classList={{
								"input-error": !!errors().cf_app_secret,
							}}
							placeholder={
								secretFlags().cf_app_secret_set
									? "已配置，留空保留"
									: "Cloudflare Realtime App Secret"
							}
							value={form().cf_app_secret ?? ""}
							onInput={(event) =>
								updateField("cf_app_secret", event.currentTarget.value)
							}
							disabled={saving()}
						/>
					</FormField>
				</div>
			</Show>
		</>
	);
}
