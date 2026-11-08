import { buttonVariants, toast } from "@gospeak/components";
import { DEFAULT_SFU_PROVIDER, PROVIDER_LABELS } from "@gospeak/sfu-client";
import type { SFUProvider } from "@gospeak/sfu-client/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { ArrowRight, Info, RefreshCcw, Save, ServerCog } from "lucide-react";
import { useEffect, useState } from "react";
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
import ConfirmModal from "@/components/common/ConfirmModal";
import { localizedApiMessage } from "@/utils/errorMessages";
import { denyAccess } from "@/utils/accessDenied";
import { hasPermission } from "@/utils/permissions";
import { toastActionError } from "@/utils/toast";
import CapabilityBadge from "./components/CapabilityBadge";
import {
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
			denyAccess();
		}
	},
	component: SFUPage,
	staticData: {
		title: "SFU",
	},
});

const btn = buttonVariants({ variant: "outline" });

function SFUPage() {
	const queryClient = useQueryClient();
	const {
		data: providersResponse,
		isLoading,
		error,
		refetch: refetchList,
	} = useQuery({
		queryKey: ["sfu-providers-manage"],
		queryFn: listSFUProviders,
	});
	const [selectedProvider, setSelectedProvider] = useState<
		SFUProvider | undefined
	>();
	// 每个 provider 独立草稿，切换查看时不互相覆盖、不丢未保存输入。
	const [drafts, setDrafts] = useState<
		Partial<Record<SFUProvider, UpdateSFUConfigParams>>
	>({});
	const [secretFlagsByProvider, setSecretFlagsByProvider] = useState<
		Partial<Record<SFUProvider, SecretFlags>>
	>({});
	const [loadedProviders, setLoadedProviders] = useState<
		Partial<Record<SFUProvider, true>>
	>({});
	const [errors, setErrors] = useState<FieldErrors>({});
	const [saving, setSaving] = useState(false);
	const [pendingSwitch, setPendingSwitch] = useState<SFUProvider | null>(null);

	const activeProvider = providersResponse?.active ?? DEFAULT_SFU_PROVIDER;
	const selected = selectedProvider ?? activeProvider;

	const form = (): UpdateSFUConfigParams => {
		return drafts[selected] ?? emptyFormForProvider(selected);
	};

	const secretFlags = (): SecretFlags => {
		return secretFlagsByProvider[selected] ?? emptySecretFlags();
	};

	// Initialize selectedProvider = activeProvider
	useEffect(() => {
		if (providersResponse && !selectedProvider) {
			setSelectedProvider(providersResponse.active);
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [providersResponse]);

	function populateProvider(
		provider: SFUProvider,
		data: Partial<UpdateSFUConfigParams> & {
			provider?: SFUProvider;
			livekit_secret_set?: boolean;
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
		for (const key of ["livekit_secret"] as const) {
			if (key in base) base[key] = "";
		}

		setDrafts((current) => {
			if (options?.preserveDraft && current[provider]) {
				return current;
			}
			return { ...current, [provider]: base };
		});
		setLoadedProviders((current) => ({ ...current, [provider]: true }));
	}

	// When selectedProvider changes, load its own draft/config only once unless refreshed.
	useEffect(() => {
		const provider = selected;
		if (!provider) return;
		if (loadedProviders[provider]) return;

		const listData = providersResponse;
		const local = listData?.providers.find((c) => c.provider === provider);
		if (local) {
			populateProvider(provider, local, { preserveDraft: true });
			return;
		}

		getSFUConfigByProvider(provider)
			.then((cfg) => populateProvider(provider, cfg, { preserveDraft: true }))
			.catch((err) => {
				// 之前这里静默降级成空表单，界面看起来像"未配置"，用户分不清是没配还是加载失败
				toast.error(
					localizedApiMessage(err, "该提供商的语音配置加载失败，请刷新重试"),
				);
				populateProvider(provider, { provider }, { preserveDraft: true });
			});
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [selected, providersResponse]);

	const capabilities = () => {
		const fromAPI = providersResponse?.capabilities?.[selected];
		return getSFUProviderCapabilities(selected, fromAPI);
	};

	const enforcementProfile = () => {
		const fromAPI = providersResponse?.capabilities?.[selected];
		return getSFUEnforcementProfile(selected, fromAPI);
	};

	const updateField = <K extends keyof UpdateSFUConfigParams>(
		key: K,
		value: UpdateSFUConfigParams[K],
	) => {
		setDrafts((current) => {
			const prev = current[selected] ?? emptyFormForProvider(selected);
			return {
				...current,
				[selected]: {
					...prev,
					provider: selected,
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
		if (saving) return;
		const current = form();
		const e = validateSFUForm(current, secretFlags());
		setErrors(e);
		if (Object.keys(e).length > 0) return;
		setSaving(true);
		try {
			// 只提交当前 provider 字段，其他 SFU 配置保持不动。
			const saved = await updateSFUConfig(cleanForm(current));
			populateProvider(saved.provider, saved);
			setSelectedProvider(saved.provider);
			void refetchList();
			toast.success("SFU 配置已保存");
		} catch (error) {
			toastActionError(error, "保存 SFU 配置失败");
		} finally {
			setSaving(false);
		}
	};

	const handleSwitch = async (provider: SFUProvider) => {
		if (saving) return;
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
			setPendingSwitch(null);
			toast.success(`已切换到 ${PROVIDER_LABELS[provider] ?? provider}`);
		} catch (error) {
			toastActionError(error, "切换 SFU 提供商失败");
		} finally {
			setSaving(false);
		}
	};

	const showSwitch = selectedProvider && selectedProvider !== activeProvider;

	const _invalidate = () =>
		void queryClient.invalidateQueries({ queryKey: ["sfu-providers-manage"] });

	return (
		<ManagePage>
			<ManageHeader
				icon={<ServerCog size={20} />}
				title="SFU 提供商"
				description="选择实时语音后端，并配置对应连接参数"
				actions={
					<button
						type="button"
						className="hover:bg-muted flex size-9 cursor-pointer items-center justify-center rounded-sm transition-colors disabled:pointer-events-none disabled:opacity-50"
						onClick={() => {
							void refetchList();
						}}
						disabled={saving || isLoading}
						title="重新加载"
					>
						<RefreshCcw size={16} />
					</button>
				}
			/>

			{isLoading && <ManageLoading />}

			{!isLoading && error && (
				<div className="border-destructive/50 bg-destructive/10 text-destructive flex items-center rounded-sm px-3 py-2 text-sm">
					{error instanceof Error ? error.message : "加载失败"}
				</div>
			)}

			{!isLoading && !error && (
				<>
					<section className="border-border/80 bg-card rounded-lg border p-4 shadow-sm md:p-5">
						<div className="mb-4 flex flex-wrap items-end justify-between gap-3">
							<div>
								<div className="text-sm font-semibold">提供商选择</div>
								<p className="text-faint-foreground mt-0.5 text-xs">
									当前激活{" "}
									<span className="text-foreground font-medium">
										{PROVIDER_LABELS[activeProvider]}
									</span>
									，点击卡片可切换查看配置
								</p>
							</div>
							<div className="flex flex-wrap items-center gap-1.5">
								<CapabilityBadge
									label="服务端静音"
									active={!!capabilities().serverMute}
								/>
								<CapabilityBadge
									label="服务端踢人"
									active={!!capabilities().serverKick}
								/>
							</div>
						</div>

						<ProviderCardGrid
							activeProvider={activeProvider}
							selectedProvider={selectedProvider}
							providers={providersResponse?.providers ?? []}
							disabled={saving}
							onSelect={(provider) => {
								// 仅切换查看目标；各 provider 草稿独立保存。
								setSelectedProvider(provider);
								setErrors({});
							}}
						/>
					</section>

					<section className="border-border/80 bg-card rounded-lg border p-4 shadow-sm md:p-5">
						<div className="mb-3 flex flex-wrap items-start justify-between gap-3">
							<div>
								<div className="text-sm font-semibold">
									{PROVIDER_LABELS[selected]} · 强制能力对比
								</div>
								<p className="text-faint-foreground mt-0.5 text-xs">
									{enforcementProfile().summary}
								</p>
							</div>
							<div className="text-muted-foreground flex flex-wrap items-center gap-1.5 text-2xs">
								<span className="border-border rounded-full border px-2 py-0.5">
									hard 原生
								</span>
								<span className="border-border rounded-full border px-2 py-0.5">
									degraded 降级
								</span>
								<span className="border-border rounded-full border px-2 py-0.5">
									soft 信令
								</span>
								<span className="border-border rounded-full border px-2 py-0.5 opacity-60">
									none
								</span>
							</div>
						</div>

						<div className="grid grid-cols-1 gap-2 md:grid-cols-2">
							{enforcementProfile().details.map((item) => (
								<div
									key={item.label}
									className="border-border/70 bg-muted/20 rounded-lg border px-3 py-2.5"
								>
									<div className="mb-1 flex items-center justify-between gap-2">
										<span className="text-foreground text-xs font-medium">
											{item.label}
										</span>
										<span className="border-border text-muted-foreground rounded-full border px-2 py-0.5 text-2xs">
											{item.level}
										</span>
									</div>
									<p className="text-muted-foreground text-2xs leading-relaxed">
										<span className="text-foreground font-medium">实现：</span>
										{item.impl}
									</p>
									<p className="text-faint-foreground mt-1 text-2xs leading-relaxed">
										<span className="font-medium">降级/底座：</span>
										{item.fallback}
									</p>
								</div>
							))}
						</div>

						<div className="border-border/70 bg-muted/20 text-muted-foreground mt-3 rounded-lg border px-3 py-2 text-2xs">
							统一策略：业务/信令 soft 始终执行；媒体强制成功时事件标
							<code className="mx-1">enforcement=hard|degraded</code>
							，否则为
							<code className="mx-1">soft</code>
							。不会把 soft 伪装成 hard。
						</div>
					</section>

					<section className="space-y-3">
						<div className="border-border/80 bg-card text-muted-foreground flex items-start gap-2.5 rounded-lg border px-4 py-3 text-sm shadow-sm">
							<Info
								size={16}
								className="text-faint-foreground mt-0.5 shrink-0"
							/>
							<div>
								当前激活{" "}
								<span className="text-foreground font-medium">
									{PROVIDER_LABELS[activeProvider]}
								</span>
								。成员静音仍为前端本地远端轨道静音，不依赖服务端能力。
							</div>
						</div>

						{showSwitch && (
							<div className="border-border/80 bg-muted/30 text-foreground flex items-start gap-2.5 rounded-lg border px-4 py-3 text-sm">
								<Info
									size={16}
									className="text-faint-foreground mt-0.5 shrink-0"
								/>
								<div>
									当前查看{" "}
									<span className="text-foreground font-medium">
										{PROVIDER_LABELS[selectedProvider ?? activeProvider]}
									</span>
									的配置，尚未激活。保存配置将自动激活，或直接切换。
								</div>
							</div>
						)}
					</section>

					<section className="border-border/80 bg-card rounded-lg border p-4 shadow-sm md:p-5">
						<div className="mb-4 flex flex-wrap items-center justify-between gap-3">
							<div>
								<div className="text-sm font-semibold">
									{PROVIDER_LABELS[selectedProvider ?? activeProvider]} 配置
								</div>
								<p className="text-faint-foreground mt-0.5 text-xs">
									仅编辑当前提供商参数；密钥留空表示保留已有配置
								</p>
							</div>
							<div className="flex items-center gap-2">
								{showSwitch && (
									<button
										type="button"
										className={btn}
										onClick={() => {
											const provider = selectedProvider;
											// 切换提供商会影响全站语音后端，必须确认
											if (provider) setPendingSwitch(provider);
										}}
										disabled={saving}
									>
										<ArrowRight size={16} />
										{saving ? "切换中..." : "切换到此提供商"}
									</button>
								)}
								<button
									type="button"
									className={btn}
									disabled={saving}
									onClick={() => void handleSave()}
								>
									<Save size={16} />
									{saving ? "保存中..." : "保存并激活"}
								</button>
							</div>
						</div>

						{selectedProvider && (
							<div className="border-border/70 bg-muted/20 rounded-lg border p-3 md:p-4">
								<ProviderConfigForm
									provider={selectedProvider}
									form={form()}
									errors={errors}
									secretFlags={secretFlags()}
									saving={saving}
									updateField={updateField}
								/>
							</div>
						)}
					</section>
				</>
			)}

			<ConfirmModal
				open={!!pendingSwitch}
				title="切换语音服务提供商"
				message={
					<span>
						切换后全站语音将改用「
						{pendingSwitch
							? (PROVIDER_LABELS[pendingSwitch] ?? pendingSwitch)
							: ""}
						」。正在进行中的语音房间会断开，需要重新进入。确认切换？
					</span>
				}
				confirmText="切换"
				confirmVariant="default"
				loading={saving}
				onClose={() => setPendingSwitch(null)}
				onConfirm={() => {
					if (pendingSwitch) void handleSwitch(pendingSwitch);
				}}
			/>
		</ManagePage>
	);
}
