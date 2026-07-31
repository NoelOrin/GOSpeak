import { DEFAULT_SFU_PROVIDER, PROVIDER_LABELS } from "@gospeak/sfu-client";
import type { SFUProvider } from "@gospeak/sfu-client/types";
import { createFileRoute, redirect } from "@tanstack/solid-router";
import ArrowRight from "lucide-solid/icons/arrow-right";
import Info from "lucide-solid/icons/info";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import Save from "lucide-solid/icons/save";
import ServerCog from "lucide-solid/icons/server-cog";
import TriangleAlert from "lucide-solid/icons/triangle-alert";
import {
	createEffect,
	createResource,
	createSignal,
	For,
	Show,
} from "solid-js";
import { showToast } from "solid-notifications";
import {
	getSFUConfigByProvider,
	getSFUEnforcementProfile,
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
	emptyFormForProvider,
	emptySecretFlags,
	pickProviderForm,
	type SecretFlags,
	secretFlagsFromConfig,
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
	// 每个 provider 独立草稿，切换查看时不互相覆盖、不丢未保存输入。
	const [drafts, setDrafts] = createSignal<
		Partial<Record<SFUProvider, UpdateSFUConfigParams>>
	>({});
	const [secretFlagsByProvider, setSecretFlagsByProvider] = createSignal<
		Partial<Record<SFUProvider, SecretFlags>>
	>({});
	const [loadedProviders, setLoadedProviders] = createSignal<
		Partial<Record<SFUProvider, true>>
	>({});
	const [errors, setErrors] = createSignal<FieldErrors>({});
	const [saving, setSaving] = createSignal(false);

	const activeProvider = () =>
		providersResponse()?.active ?? DEFAULT_SFU_PROVIDER;

	const selected = () => selectedProvider() ?? activeProvider();

	const form = (): UpdateSFUConfigParams => {
		const provider = selected();
		return drafts()[provider] ?? emptyFormForProvider(provider);
	};

	const secretFlags = (): SecretFlags => {
		const provider = selected();
		return secretFlagsByProvider()[provider] ?? emptySecretFlags();
	};

	// Initialize selectedProvider = activeProvider
	createEffect(() => {
		const data = providersResponse();
		if (data && !selectedProvider()) {
			setSelectedProvider(data.active);
		}
	});

	// When selectedProvider changes, load its own draft/config only once unless refreshed.
	createEffect(() => {
		const provider = selectedProvider();
		if (!provider) return;
		if (loadedProviders()[provider]) return;

		const listData = providersResponse();
		const local = listData?.providers.find((c) => c.provider === provider);
		if (local) {
			populateProvider(provider, local, { preserveDraft: true });
			return;
		}

		getSFUConfigByProvider(provider)
			.then((cfg) => populateProvider(provider, cfg, { preserveDraft: true }))
			.catch(() =>
				populateProvider(provider, { provider }, { preserveDraft: true }),
			);
	});

	function populateProvider(
		provider: SFUProvider,
		data: Partial<UpdateSFUConfigParams> & {
			provider?: SFUProvider;
			livekit_secret_set?: boolean;
			agora_app_certificate_set?: boolean;
			agora_customer_secret_set?: boolean;
			srs_secret_set?: boolean;
			daily_api_key_set?: boolean;
			cf_app_secret_set?: boolean;
		},
		options?: { preserveDraft?: boolean },
	) {
		// 密钥字段不回填明文；仅记录是否已配置，提交时留空表示保留旧值。
		setSecretFlagsByProvider((current) => ({
			...current,
			[provider]: secretFlagsFromConfig(data),
		}));

		const base = pickProviderForm(provider, data);
		// 密钥永远不回填明文
		for (const key of [
			"livekit_secret",
			"agora_app_certificate",
			"agora_customer_secret",
			"srs_secret",
			"daily_api_key",
			"cf_app_secret",
		] as const) {
			if (key in base) base[key] = "";
		}

		setDrafts((current) => {
			if (options?.preserveDraft && current[provider]) {
				return current;
			}
			return {
				...current,
				[provider]: base,
			};
		});
		setLoadedProviders((current) => ({ ...current, [provider]: true }));
	}

	const capabilities = () => {
		const provider = selected();
		const fromAPI = providersResponse()?.capabilities?.[provider];
		return getSFUProviderCapabilities(provider, fromAPI);
	};

	const enforcementProfile = () => {
		const provider = selected();
		const fromAPI = providersResponse()?.capabilities?.[provider];
		return getSFUEnforcementProfile(provider, fromAPI);
	};

	const updateField = <K extends keyof UpdateSFUConfigParams>(
		key: K,
		value: UpdateSFUConfigParams[K],
	) => {
		const provider = selected();
		setDrafts((current) => {
			const prev = current[provider] ?? emptyFormForProvider(provider);
			return {
				...current,
				[provider]: {
					...prev,
					provider,
					[key]: value,
				},
			};
		});
		setErrors((cur) => {
			if (!cur[key]) return cur;
			const next = { ...cur };
			delete next[key];
			return next;
		});
	};

	const handleSave = async () => {
		const current = form();
		const e = validateSFUForm(current, secretFlags());
		setErrors(e);
		if (Object.keys(e).length > 0) {
			showToast("请修正配置错误", { type: "error" });
			return;
		}
		setSaving(true);
		try {
			// 只提交当前 provider 字段，其他 SFU 配置保持不动。
			const saved = await updateSFUConfig(cleanForm(current));
			populateProvider(saved.provider, saved);
			setSelectedProvider(saved.provider);
			void refetchList();
			showToast(
				`${PROVIDER_LABELS[saved.provider]} 配置已保存并激活，所有客户端将强制刷新`,
				{ type: "success" },
			);
		} catch {
		} finally {
			setSaving(false);
		}
	};

	const handleSwitch = async (provider: SFUProvider) => {
		setSaving(true);
		try {
			const cfg = await switchSFUProvider(provider);
			// 切换激活态不覆盖未保存草稿，只刷新密钥已配置标记。
			populateProvider(provider, cfg, { preserveDraft: true });
			setSecretFlagsByProvider((current) => ({
				...current,
				[provider]: secretFlagsFromConfig(cfg),
			}));
			setSelectedProvider(provider);
			void refetchList();
			showToast(`已切换到 ${PROVIDER_LABELS[provider]}，所有客户端将强制刷新`, {
				type: "success",
			});
		} catch {
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
								label="服务端静音"
								active={!!capabilities().serverMute}
							/>
							<CapabilityBadge
								label="服务端踢人"
								active={!!capabilities().serverKick}
							/>
							<CapabilityBadge
								label="专属信令"
								active={capabilities().requiresSignalAdapter}
							/>
						</div>
					</div>

					<ProviderCardGrid
						activeProvider={activeProvider()}
						selectedProvider={selectedProvider()}
						providers={providersResponse()?.providers ?? []}
						disabled={saving()}
						onSelect={(provider) => {
							// 仅切换查看目标；各 provider 草稿独立保存。
							setSelectedProvider(provider);
							setErrors({});
						}}
					/>
				</section>

				<section class="rounded-2xl border border-base-300/80 bg-base-100 p-4 shadow-sm md:p-5">
					<div class="mb-3 flex flex-wrap items-start justify-between gap-3">
						<div>
							<div class="text-sm font-semibold">
								{PROVIDER_LABELS[selected()]} · 强制能力对比
							</div>
							<p class="mt-0.5 text-xs text-base-content/50">
								{enforcementProfile().summary}
							</p>
						</div>
						<div class="flex flex-wrap items-center gap-1.5 text-[11px] text-base-content/55">
							<span class="rounded-full border border-base-300 px-2 py-0.5">
								hard 原生
							</span>
							<span class="rounded-full border border-base-300 px-2 py-0.5">
								degraded 降级
							</span>
							<span class="rounded-full border border-base-300 px-2 py-0.5">
								soft 信令
							</span>
							<span class="rounded-full border border-base-300 px-2 py-0.5 opacity-60">
								none
							</span>
						</div>
					</div>

					<div class="grid grid-cols-1 gap-2 md:grid-cols-2">
						<For each={enforcementProfile().details}>
							{(item) => (
								<div class="rounded-xl border border-base-300/70 bg-base-200/20 px-3 py-2.5">
									<div class="mb-1 flex items-center justify-between gap-2">
										<span class="text-xs font-medium text-base-content">
											{item.label}
										</span>
										<span class="rounded-full border border-base-300 px-2 py-0.5 text-[11px] text-base-content/65">
											{item.level}
										</span>
									</div>
									<p class="text-[11px] leading-relaxed text-base-content/70">
										<span class="font-medium text-base-content/80">实现：</span>
										{item.impl}
									</p>
									<p class="mt-1 text-[11px] leading-relaxed text-base-content/50">
										<span class="font-medium">降级/底座：</span>
										{item.fallback}
									</p>
								</div>
							)}
						</For>
					</div>

					<div class="mt-3 rounded-xl border border-base-300/70 bg-base-200/20 px-3 py-2 text-[11px] text-base-content/55">
						统一策略：业务/信令 soft 始终执行；媒体强制成功时事件标
						<code class="mx-1">enforcement=hard|degraded</code>
						，否则为
						<code class="mx-1">soft</code>
						。不会把 soft 伪装成 hard。
					</div>
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
								仅编辑当前提供商参数；密钥留空表示保留已有配置
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
