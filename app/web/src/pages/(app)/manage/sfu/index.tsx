import { DEFAULT_SFU_PROVIDER, PROVIDER_LABELS } from "@gospeak/sfu-client";
import type { SFUProvider } from "@gospeak/sfu-client/types";
import { createFileRoute, redirect } from "@tanstack/solid-router";
import ArrowRight from "lucide-solid/icons/arrow-right";
import Info from "lucide-solid/icons/info";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import Save from "lucide-solid/icons/save";
import ServerCog from "lucide-solid/icons/server-cog";
import TriangleAlert from "lucide-solid/icons/triangle-alert";
import { createEffect, createResource, createSignal, Show } from "solid-js";
import { showToast } from "solid-notifications";
import {
	getSFUConfigByProvider,
	getSFUProviderCapabilities,
	listSFUProviders,
	switchSFUProvider,
	type UpdateSFUConfigParams,
	updateSFUConfig,
} from "@/api/sfu";
import {
	ManageHeader,
	ManageLoading,
	ManagePage,
} from "@/components/manage/ManageShell";
import { hasPermission } from "@/utils/permissions";
import CapabilityBadge from "./components/CapabilityBadge";
import {
	DISABLED_PROVIDERS,
	emptyForm,
	emptySecretFlags,
	type SecretFlags,
} from "./components/constants";
import ProviderCardGrid from "./components/ProviderCardGrid";
import ProviderConfigForm from "./components/ProviderConfigForm";
import {
	cleanForm,
	type FieldErrors,
	validateSFUForm,
} from "./components/validation";

export const Route = createFileRoute("/(app)/manage/sfu/")({
	beforeLoad: () => {
		if (!hasPermission("sfu:manage")) {
			throw redirect({ to: "/" });
		}
	},
	component: SFUPage,
	staticData: {
		title: "SFU",
		icon: "icon-manage",
	},
});

function SFUPage() {
	const [providersResponse, { refetch: refetchList }] =
		createResource(listSFUProviders);
	const [selectedProvider, setSelectedProvider] = createSignal<
		SFUProvider | undefined
	>();
	const [form, setForm] = createSignal<UpdateSFUConfigParams>(emptyForm);
	const [secretFlags, setSecretFlags] = createSignal<SecretFlags>(
		emptySecretFlags(),
	);
	const [errors, setErrors] = createSignal<FieldErrors>({});
	const [saving, setSaving] = createSignal(false);

	const activeProvider = () =>
		providersResponse()?.active ?? DEFAULT_SFU_PROVIDER;

	// Initialize selectedProvider = activeProvider
	createEffect(() => {
		const data = providersResponse();
		if (data && !selectedProvider()) {
			setSelectedProvider(data.active);
		}
	});

	// When selectedProvider changes, load its config into the form
	createEffect(() => {
		const p = selectedProvider();
		if (!p) return;
		const listData = providersResponse();
		const local = listData?.providers.find((c) => c.provider === p);
		if (local) {
			populateForm(local);
			return;
		}
		getSFUConfigByProvider(p)
			.then((cfg) => populateForm(cfg))
			.catch(() => populateForm({ ...emptyForm, provider: p }));
	});

	function populateForm(data: { provider: SFUProvider; [key: string]: any }) {
		// 密钥字段不回填明文；仅记录是否已配置，提交时留空表示保留旧值。
		setSecretFlags({
			livekit_secret_set: !!data.livekit_secret_set,
			agora_app_certificate_set: !!data.agora_app_certificate_set,
			agora_customer_secret_set: !!data.agora_customer_secret_set,
			srs_secret_set: !!data.srs_secret_set,
			daily_api_key_set: !!data.daily_api_key_set,
			cf_app_secret_set: !!data.cf_app_secret_set,
		});
		setForm({
			provider: data.provider,
			livekit_host: data.livekit_host || "",
			livekit_key: data.livekit_key || "",
			livekit_secret: "",
			agora_app_id: data.agora_app_id || "",
			agora_app_certificate: "",
			agora_host: data.agora_host || "",
			agora_customer_id: data.agora_customer_id || "",
			agora_customer_secret: "",
			mediasoup_bridge_url: data.mediasoup_bridge_url || "",
			mediasoup_host: data.mediasoup_host || "",
			srs_host: data.srs_host || "",
			srs_api_port: data.srs_api_port || "1985",
			srs_secret: "",
			srs_whip_url: data.srs_whip_url || "",
			srs_public_host: data.srs_public_host || "",
			daily_api_key: "",
			daily_domain: data.daily_domain || "",
			cf_app_id: data.cf_app_id || "",
			cf_app_secret: "",
			cf_stun_url: data.cf_stun_url || "stun.cloudflare.com:3478",
		});
	}

	const capabilities = () =>
		getSFUProviderCapabilities(selectedProvider() ?? activeProvider());

	const updateField = <K extends keyof UpdateSFUConfigParams>(
		key: K,
		value: UpdateSFUConfigParams[K],
	) => {
		setForm((current) => ({ ...current, [key]: value }));
		setErrors((cur) => {
			if (!cur[key]) return cur;
			const next = { ...cur };
			delete next[key];
			return next;
		});
	};

	const handleSave = async () => {
		const e = validateSFUForm(form(), secretFlags());
		setErrors(e);
		if (Object.keys(e).length > 0) {
			showToast("请修正配置错误", { type: "error" });
			return;
		}
		setSaving(true);
		try {
			const saved = await updateSFUConfig(cleanForm(form()));
			populateForm(saved);
			void refetchList();
			showToast(
				`${PROVIDER_LABELS[form().provider]} 配置已保存并激活，所有客户端将强制刷新`,
				{ type: "success" },
			);
		} catch (error) {
			showToast(error instanceof Error ? error.message : "保存失败", {
				type: "error",
			});
		} finally {
			setSaving(false);
		}
	};

	const handleSwitch = async (provider: SFUProvider) => {
		setSaving(true);
		try {
			const cfg = await switchSFUProvider(provider);
			populateForm(cfg);
			void refetchList();
			showToast(`已切换到 ${PROVIDER_LABELS[provider]}，所有客户端将强制刷新`, {
				type: "success",
			});
		} catch (error) {
			showToast(error instanceof Error ? error.message : "切换失败", {
				type: "error",
			});
		} finally {
			setSaving(false);
		}
	};

	const showSwitch = () => {
		const sel = selectedProvider();
		return sel && sel !== activeProvider();
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<ServerCog size={18} />}
				title="SFU 提供商"
				description="选择实时语音后端，并配置对应连接参数"
				actions={
					<button
						class="btn btn-ghost btn-sm btn-square"
						onClick={() => void refetchList()}
						disabled={saving() || providersResponse.loading}
						title="重新加载"
					>
						<RefreshCcw size={16} />
					</button>
				}
			/>

			<Show when={providersResponse.loading}>
				<ManageLoading />
			</Show>

			<Show when={!providersResponse.loading && providersResponse.error}>
				<div class="alert alert-error text-sm">
					{providersResponse.error instanceof Error
						? providersResponse.error.message
						: "加载失败"}
				</div>
			</Show>

			<Show when={!providersResponse.loading && !providersResponse.error}>
				<section class="rounded-2xl border border-base-300/80 bg-base-100 p-4 shadow-sm md:p-5">
					<div class="mb-4 flex flex-wrap items-end justify-between gap-3">
						<div>
							<div class="text-sm font-semibold">提供商选择</div>
							<p class="mt-0.5 text-xs text-base-content/50">
								当前激活{" "}
								<span class="font-medium text-base-content">
									{PROVIDER_LABELS[activeProvider()]}
								</span>
								，点击卡片可切换查看配置
							</p>
						</div>
						<div class="flex flex-wrap items-center gap-1.5">
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
					</div>

					<ProviderCardGrid
						activeProvider={activeProvider()}
						selectedProvider={selectedProvider()}
						providers={providersResponse()?.providers ?? []}
						disabled={saving()}
						onSelect={(provider) => {
							setSelectedProvider(provider);
							setForm((current) => ({ ...current, provider }));
							setErrors({});
						}}
					/>
				</section>

				<section class="space-y-3">
					<div class="flex items-start gap-2.5 rounded-2xl border border-base-300/80 bg-base-100 px-4 py-3 text-sm text-base-content/70 shadow-sm">
						<Info size={16} class="mt-0.5 shrink-0 text-base-content/45" />
						<div>
							当前激活{" "}
							<span class="font-medium text-base-content">
								{PROVIDER_LABELS[activeProvider()]}
							</span>
							。成员静音仍为前端本地远端轨道静音，不依赖服务端能力。
						</div>
					</div>

					<Show when={showSwitch()}>
						<div class="flex items-start gap-2.5 rounded-2xl border border-base-300/80 bg-base-200/30 px-4 py-3 text-sm text-base-content/75">
							<Info size={16} class="mt-0.5 shrink-0 text-base-content/45" />
							<div>
								当前查看{" "}
								<span class="font-medium text-base-content">
									{PROVIDER_LABELS[selectedProvider() ?? activeProvider()]}
								</span>
								的配置，尚未激活。保存配置将自动激活，或直接切换。
							</div>
						</div>
					</Show>

					<Show when={DISABLED_PROVIDERS.includes("mediasoup")}>
						<div class="flex items-start gap-2.5 rounded-2xl border border-warning/20 bg-warning/8 px-4 py-3 text-sm text-base-content/70">
							<TriangleAlert
								size={16}
								class="mt-0.5 shrink-0 text-base-content/45"
							/>
							<div>MediaSoup 当前保留展示但暂不开放配置与切换。</div>
						</div>
					</Show>
				</section>

				<section class="rounded-2xl border border-base-300/80 bg-base-100 p-4 shadow-sm md:p-5">
					<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
						<div>
							<div class="text-sm font-semibold">
								{PROVIDER_LABELS[selectedProvider() ?? activeProvider()]} 配置
							</div>
							<p class="mt-0.5 text-xs text-base-content/50">
								密钥字段留空表示保留已有配置
							</p>
						</div>
						<div class="flex items-center gap-2">
							<Show when={showSwitch()}>
								<button
									type="button"
									class="btn btn-sm border border-base-300 bg-base-100 text-base-content/80 shadow-none hover:bg-base-200"
									onClick={() => {
										const provider = selectedProvider();
										if (provider) void handleSwitch(provider);
									}}
									disabled={saving()}
								>
									<Show when={saving()} fallback={<ArrowRight size={16} />}>
										<span class="loading loading-spinner loading-xs" />
									</Show>
									{saving() ? "切换中..." : "切换到此提供商"}
								</button>
							</Show>
							<button
								type="button"
								class="btn btn-sm border border-base-300 bg-base-100 text-base-content shadow-none hover:bg-base-200"
								classList={{ "btn-disabled": saving() }}
								onClick={handleSave}
							>
								<Show when={saving()} fallback={<Save size={16} />}>
									<span class="loading loading-spinner loading-xs" />
								</Show>
								{saving() ? "保存中..." : "保存并激活"}
							</button>
						</div>
					</div>

					<Show when={selectedProvider()}>
						{(provider) => (
							<div class="rounded-xl border border-base-300/70 bg-base-200/20 p-3 md:p-4">
								<ProviderConfigForm
									provider={provider()}
									form={form()}
									errors={errors()}
									secretFlags={secretFlags()}
									saving={saving()}
									updateField={updateField}
								/>
							</div>
						)}
					</Show>
				</section>
			</Show>
		</ManagePage>
	);
}
