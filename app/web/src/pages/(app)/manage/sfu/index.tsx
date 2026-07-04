import { createFileRoute, redirect } from "@tanstack/solid-router";
import userStore from "@/stores/userStore";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import Save from "lucide-solid/icons/save";
import ServerCog from "lucide-solid/icons/server-cog";
import { createEffect, createResource, createSignal, For, Show } from "solid-js";
import { showToast } from "solid-notifications";
import {
	getSFUProviderCapabilities,
	getSFUConfig,
	updateSFUConfig,
	type UpdateSFUConfigParams,
} from "@/api/sfu";
import { DEFAULT_SFU_PROVIDER, PROVIDER_LABELS } from "@gospeak/sfu-client";
import type { SFUProvider } from "@gospeak/sfu-client/types";

export const Route = createFileRoute("/(app)/manage/sfu/")({
	beforeLoad: () => {
		if (userStore.user()?.role !== "admin") {
			throw redirect({ to: "/" });
		}
	},
	component: SFUPage,
	staticData: {
		title: "SFU",
		icon: "icon-manage",
	},
});

const emptyForm: UpdateSFUConfigParams = {
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
	srs_whip_port: "1985",
	srs_secret: "",
	daily_api_key: "",
	daily_domain: "",
};

const PROVIDER_OPTIONS: { value: SFUProvider; label: string }[] = [
	{ value: "livekit", label: "LiveKit" },
	{ value: "agora", label: "Agora" },
	{ value: "mediasoup", label: "MediaSoup" },
	{ value: "srs", label: "SRS" },
	{ value: "daily", label: "Daily" },
];

const DISABLED_PROVIDERS: SFUProvider[] = ["mediasoup"];

function SFUPage() {
	const [config, { refetch }] = createResource(getSFUConfig);
	const [form, setForm] = createSignal<UpdateSFUConfigParams>(emptyForm);
	const [saving, setSaving] = createSignal(false);
	const selectedProvider = () => form().provider;
	const capabilities = () => getSFUProviderCapabilities(selectedProvider());
	const isProviderDisabled = (provider: SFUProvider) =>
		DISABLED_PROVIDERS.includes(provider);

	createEffect(() => {
		const data = config();
		if (!data) return;
		setForm({
			provider: data.provider,
			livekit_host: data.livekit_host || "",
			livekit_key: data.livekit_key || "",
			livekit_secret: data.livekit_secret || "",
			agora_app_id: data.agora_app_id || "",
			agora_app_certificate: data.agora_app_certificate || "",
			agora_host: data.agora_host || "",
			agora_customer_id: data.agora_customer_id || "",
			agora_customer_secret: data.agora_customer_secret || "",
			mediasoup_bridge_url: data.mediasoup_bridge_url || "",
			mediasoup_host: data.mediasoup_host || "",
			srs_host: data.srs_host || "",
			srs_api_port: data.srs_api_port || "1985",
			srs_whip_port: data.srs_whip_port || "1985",
			srs_secret: data.srs_secret || "",
			daily_api_key: data.daily_api_key || "",
			daily_domain: data.daily_domain || "",
		});
	});

	const updateField = <K extends keyof UpdateSFUConfigParams>(
		key: K,
		value: UpdateSFUConfigParams[K],
	) => {
		setForm((current) => ({ ...current, [key]: value }));
	};

	const handleSave = async () => {
		setSaving(true);
		try {
			const saved = await updateSFUConfig(form());
			setForm({
				provider: saved.provider,
				livekit_host: saved.livekit_host || "",
				livekit_key: saved.livekit_key || "",
				livekit_secret: saved.livekit_secret || "",
				agora_app_id: saved.agora_app_id || "",
				agora_app_certificate: saved.agora_app_certificate || "",
				agora_host: saved.agora_host || "",
				agora_customer_id: saved.agora_customer_id || "",
				agora_customer_secret: saved.agora_customer_secret || "",
				mediasoup_bridge_url: saved.mediasoup_bridge_url || "",
				mediasoup_host: saved.mediasoup_host || "",
				srs_host: saved.srs_host || "",
				srs_api_port: saved.srs_api_port || "1985",
				srs_whip_port: saved.srs_whip_port || "1985",
				srs_secret: saved.srs_secret || "",
				daily_api_key: saved.daily_api_key || "",
				daily_domain: saved.daily_domain || "",
			});
			showToast("SFU 配置已保存", { type: "success" });
		} catch (error) {
			showToast(error instanceof Error ? error.message : "保存失败", {
				type: "error",
			});
		} finally {
			setSaving(false);
		}
	};

	return (
    <Show
      when={!config.loading}
      fallback={
        <div class="flex h-full min-h-52 items-center justify-center">
          <span class="loading loading-spinner loading-lg" />
        </div>
      }
    >
      <div class="p-4 flex flex-col gap-4">
        <div class="flex items-center justify-between gap-3">
          <div class="flex items-center gap-2">
            <ServerCog size={20} />
            <h3 class="font-bold text-lg">SFU 提供商</h3>
          </div>
          <button
            class="btn btn-ghost btn-sm"
            onClick={() => void refetch()}
            disabled={saving()}
            title="重新加载"
          >
            <RefreshCcw size={16} />
          </button>
        </div>

        <Show when={config.error}>
          <div class="alert alert-error text-sm">
            {config.error instanceof Error ? config.error.message : "加载失败"}
          </div>
        </Show>

        <div class="divider my-0 text-xs text-base-content/40">提供商选择</div>
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
          <For each={PROVIDER_OPTIONS}>
            {(option) => (
              <button
                type="button"
                class="btn btn-sm justify-between px-3"
                classList={{
                  "btn-primary btn-soft shadow-none":
                    form().provider === option.value,
                  "btn-ghost shadow-none hover:bg-base-200":
                    form().provider !== option.value &&
                    !isProviderDisabled(option.value),
                  "btn-ghost shadow-none cursor-not-allowed opacity-60":
                    isProviderDisabled(option.value),
                }}
                onClick={() => {
                  if (isProviderDisabled(option.value)) return;
                  updateField("provider", option.value);
                }}
                disabled={saving() || isProviderDisabled(option.value)}
              >
                <span>{option.label}</span>
                <Show when={isProviderDisabled(option.value)}>
                  <span class="rounded-full bg-base-300 px-2 py-0.5 text-[11px] text-base-content/55">
                    暂禁用
                  </span>
                </Show>
              </button>
            )}
          </For>
        </div>

        <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
          <CapabilityBadge
            label="参与者列表"
            active={capabilities().supportsParticipants}
          />
          <CapabilityBadge
            label="专属信令适配"
            active={capabilities().requiresSignalAdapter}
          />
          <CapabilityBadge
            label="信令踢人"
            active={capabilities().kickViaSignal}
          />
        </div>

        <div class="rounded-box border border-base-300 bg-base-200/40 px-4 py-3 text-sm text-base-content/70">
          当前提供商为{" "}
          <span class="font-medium text-base-content">
            {PROVIDER_LABELS[selectedProvider()]}
          </span>
          。 成员静音仍为前端本地远端轨道静音，不依赖服务端能力。
        </div>

        <Show when={DISABLED_PROVIDERS.includes("mediasoup")}>
          <div class="rounded-box border border-warning/20 bg-warning/8 px-4 py-3 text-sm text-base-content/70">
            MediaSoup 当前保留展示但暂不开放配置与切换。
          </div>
        </Show>

        <div class="divider my-0 text-xs text-base-content/40">
          {PROVIDER_LABELS[selectedProvider()]} 配置
        </div>

        <Show when={selectedProvider() === "livekit"}>
          <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
            <Field label="Host">
              <input
                type="text"
                class="input input-bordered input-sm w-full"
                placeholder="wss://livekit.example.com"
                value={form().livekit_host}
                onInput={(event) =>
                  updateField("livekit_host", event.currentTarget.value)
                }
                disabled={saving()}
              />
            </Field>
            <div />
            <Field label="API Key">
              <input
                type="text"
                class="input input-bordered input-sm w-full"
                placeholder="API key"
                value={form().livekit_key}
                onInput={(event) =>
                  updateField("livekit_key", event.currentTarget.value)
                }
                disabled={saving()}
              />
            </Field>
            <Field label="API Secret">
              <input
                type="password"
                class="input input-bordered input-sm w-full"
                placeholder="API secret"
                value={form().livekit_secret}
                onInput={(event) =>
                  updateField("livekit_secret", event.currentTarget.value)
                }
                disabled={saving()}
              />
            </Field>
          </div>
        </Show>

        <Show when={selectedProvider() === "agora"}>
          <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
            <Field label="App ID">
              <input
                type="text"
                class="input input-bordered input-sm w-full"
                placeholder="Agora App ID"
                value={form().agora_app_id}
                onInput={(event) =>
                  updateField("agora_app_id", event.currentTarget.value)
                }
                disabled={saving()}
              />
            </Field>
            <Field label="REST Host">
              <input
                type="text"
                class="input input-bordered input-sm w-full"
                placeholder="https://api.agora.io"
                value={form().agora_host}
                onInput={(event) =>
                  updateField("agora_host", event.currentTarget.value)
                }
                disabled={saving()}
              />
            </Field>
            <Field label="App Certificate">
              <input
                type="password"
                class="input input-bordered input-sm w-full"
                placeholder="App certificate"
                value={form().agora_app_certificate}
                onInput={(event) =>
                  updateField(
                    "agora_app_certificate",
                    event.currentTarget.value,
                  )
                }
                disabled={saving()}
              />
            </Field>
            <Field label="Customer Secret">
              <input
                type="password"
                class="input input-bordered input-sm w-full"
                placeholder="Customer secret"
                value={form().agora_customer_secret}
                onInput={(event) =>
                  updateField(
                    "agora_customer_secret",
                    event.currentTarget.value,
                  )
                }
                disabled={saving()}
              />
            </Field>
            <Field label="Customer ID">
              <input
                type="text"
                class="input input-bordered input-sm w-full"
                placeholder="Customer ID"
                value={form().agora_customer_id}
                onInput={(event) =>
                  updateField("agora_customer_id", event.currentTarget.value)
                }
                disabled={saving()}
              />
            </Field>
          </div>
        </Show>

		<Show when={selectedProvider() === "mediasoup"}>
          <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
            <Field label="Bridge URL">
              <input
                type="text"
                class="input input-bordered input-sm w-full"
                placeholder="https://mediasoup-bridge.example.com"
                value={form().mediasoup_bridge_url}
                onInput={(event) =>
                  updateField("mediasoup_bridge_url", event.currentTarget.value)
                }
                disabled={saving()}
              />
            </Field>
            <Field label="Host">
              <input
                type="text"
                class="input input-bordered input-sm w-full"
                placeholder="wss://mediasoup.example.com"
                value={form().mediasoup_host}
                onInput={(event) =>
                  updateField("mediasoup_host", event.currentTarget.value)
                }
                disabled={saving()}
              />
            </Field>
          </div>
		</Show>

		<Show when={selectedProvider() === "srs"}>
		  <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
			<Field label="Host">
			  <input
				type="text"
				class="input input-bordered input-sm w-full"
				placeholder="srs.example.com"
				value={form().srs_host}
				onInput={(event) => updateField("srs_host", event.currentTarget.value)}
				disabled={saving()}
			  />
			</Field>
			<Field label="API Port">
			  <input
				type="text"
				class="input input-bordered input-sm w-full"
				placeholder="1985"
				value={form().srs_api_port}
				onInput={(event) => updateField("srs_api_port", event.currentTarget.value)}
				disabled={saving()}
			  />
			</Field>
			<Field label="WHIP Port">
			  <input
				type="text"
				class="input input-bordered input-sm w-full"
				placeholder="1985"
				value={form().srs_whip_port}
				onInput={(event) => updateField("srs_whip_port", event.currentTarget.value)}
				disabled={saving()}
			  />
			</Field>
			<Field label="Secret">
			  <input
				type="password"
				class="input input-bordered input-sm w-full"
				placeholder="Bearer secret"
				value={form().srs_secret}
				onInput={(event) => updateField("srs_secret", event.currentTarget.value)}
				disabled={saving()}
			  />
			</Field>
		  </div>
		</Show>

		<Show when={selectedProvider() === "daily"}>
		  <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
			<Field label="API Key">
			  <input
				type="password"
				class="input input-bordered input-sm w-full"
				placeholder="Daily API key"
				value={form().daily_api_key}
				onInput={(event) => updateField("daily_api_key", event.currentTarget.value)}
				disabled={saving()}
			  />
			</Field>
			<Field label="Domain">
			  <input
				type="text"
				class="input input-bordered input-sm w-full"
				placeholder="your-team.daily.co"
				value={form().daily_domain}
				onInput={(event) => updateField("daily_domain", event.currentTarget.value)}
				disabled={saving()}
			  />
			</Field>
		  </div>
		</Show>

        <div class="flex justify-end pt-2">
          <button
            type="button"
            class="btn btn-primary btn-sm"
            classList={{ "btn-disabled": saving() }}
            onClick={handleSave}
          >
            <Show when={saving()} fallback={<Save size={16} />}>
              <span class="loading loading-spinner loading-xs" />
            </Show>
            {saving() ? "保存中..." : "保存配置"}
          </button>
        </div>
      </div>
    </Show>
  );
}

interface FieldProps {
	label: string;
	children: any;
}

const Field = (props: FieldProps) => (
	<fieldset class="fieldset">
		<legend class="fieldset-legend text-[14px]">{props.label}</legend>
		{props.children}
	</fieldset>
);

interface CapabilityBadgeProps {
	label: string;
	active: boolean;
}

const CapabilityBadge = (props: CapabilityBadgeProps) => (
	<div
		class="rounded-box flex items-center justify-center gap-2 border px-3 py-2 text-xs"
		classList={{
			"border-base-300 bg-base-200/70 text-base-content/75": props.active,
			"border-base-300 bg-base-100 text-base-content/45": !props.active,
		}}
	>
		<span
			class="inline-block size-2 rounded-full"
			classList={{
				"bg-base-content/55": props.active,
				"bg-base-content/30": !props.active,
			}}
		/>
		<span>{props.label}</span>
	</div>
);
