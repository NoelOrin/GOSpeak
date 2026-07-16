import { createFileRoute, redirect } from "@tanstack/solid-router";
import Blocks from "lucide-solid/icons/blocks";
import Plus from "lucide-solid/icons/plus";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import Save from "lucide-solid/icons/save";
import Trash2 from "lucide-solid/icons/trash-2";
import {
	createEffect,
	createMemo,
	createResource,
	createSignal,
	For,
	Show,
} from "solid-js";
import { showToast } from "solid-notifications";
import { listPlugins, type PluginInfo, updatePlugin } from "@/api/plugin";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import { hasPermission } from "@/utils/permissions";

const LLM_PROTOCOLS = [
	{ value: "openai-compatible", label: "OpenAI Compatible" },
	{ value: "anthropic", label: "Anthropic" },
	{ value: "gemini", label: "Gemini" },
	{ value: "gemini-response", label: "Gemini Response API" },
	{ value: "ollama", label: "Ollama" },
	{ value: "custom-http", label: "Custom HTTP" },
] as const;

type LLMProviderForm = {
	name: string;
	display_name: string;
	protocol: string;
	base_url: string;
	api_key: string;
	model: string;
	enabled: boolean;
};

export const Route = createFileRoute("/(app)/manage/bot-plugins/")({
	beforeLoad: () => {
		if (!hasPermission("plugin:read") && !hasPermission("bot:manage")) {
			throw redirect({ to: "/" });
		}
	},
	component: BotPluginsPage,
	staticData: {
		title: "Bot 插件",
		icon: "icon-manage",
	},
});

function emptyProvider(): LLMProviderForm {
	return {
		name: "",
		display_name: "",
		protocol: "openai-compatible",
		base_url: "",
		api_key: "",
		model: "",
		enabled: true,
	};
}

function BotPluginsPage() {
	const canManage = () =>
		hasPermission("plugin:manage") || hasPermission("bot:manage");

	const [pluginsData, { refetch }] = createResource(() => listPlugins());
	const [selectedName, setSelectedName] = createSignal<string>("bot-base");
	const [saving, setSaving] = createSignal(false);

	// form state for bot-base
	const [enabled, setEnabled] = createSignal(true);
	const [sideEnabled, setSideEnabled] = createSignal(false);
	const [sideAddr, setSideAddr] = createSignal("127.0.0.1:9200");
	const [defaultProvider, setDefaultProvider] = createSignal("openai");
	const [providers, setProviders] = createSignal<LLMProviderForm[]>([]);

	const selected = createMemo(() => {
		const list = pluginsData() ?? [];
		return list.find((p) => p.name === selectedName()) ?? list[0];
	});

	const hydrate = (info?: PluginInfo) => {
		if (!info) return;
		setSelectedName(info.name);
		setEnabled(!!info.enabled);
		const cfg = info.config ?? {};
		const side = (cfg.side_server ?? {}) as {
			enabled?: boolean;
			addr?: string;
		};
		setSideEnabled(!!side.enabled);
		setSideAddr(side.addr || "127.0.0.1:9200");
		setDefaultProvider((cfg.default_provider as string) || "");
		const list = Array.isArray(cfg.llm_providers)
			? (cfg.llm_providers as any[])
			: [];
		setProviders(
			list.map((p) => ({
				name: String(p.name ?? ""),
				display_name: String(p.display_name ?? p.name ?? ""),
				protocol: String(p.protocol ?? "openai-compatible"),
				base_url: String(p.base_url ?? ""),
				// 后端不回传明文 key；编辑时留空表示不修改（提交时若空则保留旧值）
				api_key: "",
				model: String(p.model ?? ""),
				enabled: p.enabled !== false,
			})),
		);
	};

	createEffect(() => {
		const info = selected();
		if (info) hydrate(info);
	});

	const statusClass = (status?: string) => {
		switch (status) {
			case "running":
				return "badge-success";
			case "failed":
				return "badge-error";
			case "stopped":
				return "badge-warning";
			default:
				return "badge-ghost";
		}
	};

	const addProvider = () => {
		setProviders((prev) => [...prev, emptyProvider()]);
	};

	const removeProvider = (idx: number) => {
		setProviders((prev) => prev.filter((_, i) => i !== idx));
	};

	const updateProviderField = <K extends keyof LLMProviderForm>(
		idx: number,
		key: K,
		value: LLMProviderForm[K],
	) => {
		setProviders((prev) =>
			prev.map((p, i) => (i === idx ? { ...p, [key]: value } : p)),
		);
	};

	const handleSave = async () => {
		if (!canManage()) {
			showToast("无插件管理权限", { type: "error" });
			return;
		}
		const info = selected();
		if (!info) return;

		// merge api_key: empty means keep existing
		const existing = Array.isArray(info.config?.llm_providers)
			? (info.config?.llm_providers as any[])
			: [];
		const existingKeyMap = new Map(
			existing.map((p) => [String(p.name), String(p.api_key ?? "")]),
		);

		const llm_providers = providers().map((p) => {
			const name = p.name.trim();
			const api_key =
				p.api_key.trim() ||
				existingKeyMap.get(name) ||
				existingKeyMap.get(p.name) ||
				"";
			return {
				name,
				display_name: p.display_name.trim() || name,
				protocol: p.protocol,
				base_url: p.base_url.trim(),
				api_key,
				model: p.model.trim(),
				enabled: p.enabled,
			};
		});

		if (llm_providers.some((p) => !p.name)) {
			showToast("供应商 name 不能为空", { type: "warning" });
			return;
		}
		const names = new Set(llm_providers.map((p) => p.name));
		if (names.size !== llm_providers.length) {
			showToast("供应商 name 不能重复", { type: "warning" });
			return;
		}

		const config = {
			side_server: {
				enabled: sideEnabled(),
				addr: sideAddr().trim(),
			},
			default_provider: defaultProvider().trim(),
			llm_providers,
		};

		setSaving(true);
		try {
			await updatePlugin({
				name: info.name,
				enabled: enabled(),
				config,
				restart: true,
			});
			showToast("插件配置已保存", { type: "success" });
			await refetch();
		} catch (e: any) {
			showToast(e?.message || "保存失败", { type: "error" });
		} finally {
			setSaving(false);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<Blocks size={18} />}
				title="Bot 插件"
				description="管理后端挂载的 Bot 基础插件、Side Server 与大模型供应商"
				actions={
					<button
						type="button"
						class="btn btn-ghost btn-sm gap-1"
						onClick={() => refetch()}
					>
						<RefreshCcw size={14} />
						刷新
					</button>
				}
			/>

			<div class="grid gap-4 lg:grid-cols-[260px_1fr]">
				<ManageSection title="已注册插件" description="后端多组件注册列表">
					<div class="flex flex-col gap-2">
						<Show
							when={!(pluginsData.loading && !pluginsData())}
							fallback={<div class="skeleton h-16 w-full" />}
						>
							<For each={pluginsData() ?? []}>
								{(p) => (
									<button
										type="button"
										class="btn btn-ghost justify-start h-auto min-h-0 py-3 px-3"
										classList={{ "btn-active": selectedName() === p.name }}
										onClick={() => {
											setSelectedName(p.name);
											hydrate(p);
										}}
									>
										<div class="flex w-full flex-col items-start gap-1 text-left">
											<div class="flex w-full items-center justify-between gap-2">
												<span class="font-medium">
													{p.display_name || p.name}
												</span>
												<span class={`badge badge-sm ${statusClass(p.status)}`}>
													{p.status}
												</span>
											</div>
											<span class="text-xs opacity-60">
												{p.name} · v{p.version}
											</span>
										</div>
									</button>
								)}
							</For>
						</Show>
					</div>
				</ManageSection>

				<div class="flex flex-col gap-4">
					<Show
						when={selected()}
						fallback={<div class="opacity-60">暂无插件</div>}
					>
						{(info) => (
							<>
								<ManageSection
									title={info().display_name || info().name}
									description={info().desc || "无描述"}
								>
									<div class="flex flex-wrap items-center gap-2 text-sm">
										<span class="badge badge-sm badge-ghost">
											{info().kind}
										</span>
										<span
											class={`badge badge-sm ${statusClass(info().status)}`}
										>
											{info().status}
										</span>
										<span class="opacity-60">作者 {info().author || "-"}</span>
										<span class="opacity-60">版本 {info().version}</span>
									</div>
									<Show when={info().error}>
										<div class="alert alert-error text-sm mt-2">
											{info().error}
										</div>
									</Show>
									<Show when={(info().side_servers?.length ?? 0) > 0}>
										<div class="mt-2 text-sm">
											<div class="font-medium mb-1">Side Servers</div>
											<ul class="list-disc pl-5 opacity-80">
												<For each={info().side_servers ?? []}>
													{(s) => (
														<li>
															{s.name}: {s.url}
														</li>
													)}
												</For>
											</ul>
										</div>
									</Show>
								</ManageSection>

								<ManageSection
									title="基础开关"
									description="启用插件并配置可选小服务端"
								>
									<label class="label cursor-pointer justify-start gap-3">
										<input
											type="checkbox"
											class="toggle toggle-primary"
											checked={enabled()}
											disabled={!canManage()}
											onChange={(e) => setEnabled(e.currentTarget.checked)}
										/>
										<span class="label-text">启用插件</span>
									</label>

									<label class="label cursor-pointer justify-start gap-3">
										<input
											type="checkbox"
											class="toggle"
											checked={sideEnabled()}
											disabled={!canManage()}
											onChange={(e) => setSideEnabled(e.currentTarget.checked)}
										/>
										<span class="label-text">
											启用 Side Server（插件自启小服务）
										</span>
									</label>

									<fieldset class="fieldset">
										<legend class="fieldset-legend text-[14px]">
											Side Server 地址
										</legend>
										<input
											type="text"
											class="input input-bordered input-sm w-full max-w-md"
											value={sideAddr()}
											disabled={!canManage() || !sideEnabled()}
											placeholder="127.0.0.1:9200 或 127.0.0.1:0"
											onInput={(e) => setSideAddr(e.currentTarget.value)}
										/>
									</fieldset>
								</ManageSection>

								<ManageSection
									title="大模型供应商"
									description="多供应商 / 多协议配置（OpenAI Compatible / Anthropic / Gemini Response / Ollama / Custom）"
									actions={
										<button
											type="button"
											class="btn btn-ghost btn-sm gap-1"
											disabled={!canManage()}
											onClick={addProvider}
										>
											<Plus size={14} />
											添加供应商
										</button>
									}
								>
									<fieldset class="fieldset mb-3">
										<legend class="fieldset-legend text-[14px]">
											默认供应商
										</legend>
										<select
											class="select select-bordered select-sm w-full max-w-md"
											value={defaultProvider()}
											disabled={!canManage()}
											onChange={(e) =>
												setDefaultProvider(e.currentTarget.value)
											}
										>
											<option value="">（未指定）</option>
											<For each={providers()}>
												{(p) => (
													<option value={p.name}>
														{p.display_name || p.name}
													</option>
												)}
											</For>
										</select>
									</fieldset>

									<div class="flex flex-col gap-3">
										<Show
											when={providers().length > 0}
											fallback={
												<div class="text-sm opacity-60">
													暂无供应商，点击右上角添加
												</div>
											}
										>
											<For each={providers()}>
												{(p, idx) => (
													<div class="rounded-box border border-base-300 bg-base-100 p-3">
														<div class="mb-2 flex items-center justify-between gap-2">
															<div class="font-medium text-sm">
																供应商 #{idx() + 1}
															</div>
															<button
																type="button"
																class="btn btn-ghost btn-xs text-error"
																disabled={!canManage()}
																onClick={() => removeProvider(idx())}
															>
																<Trash2 size={14} />
															</button>
														</div>
														<div class="grid gap-2 md:grid-cols-2">
															<label class="form-control">
																<span class="label-text text-xs">Name</span>
																<input
																	class="input input-bordered input-sm"
																	value={p.name}
																	disabled={!canManage()}
																	onInput={(e) =>
																		updateProviderField(
																			idx(),
																			"name",
																			e.currentTarget.value,
																		)
																	}
																/>
															</label>
															<label class="form-control">
																<span class="label-text text-xs">显示名</span>
																<input
																	class="input input-bordered input-sm"
																	value={p.display_name}
																	disabled={!canManage()}
																	onInput={(e) =>
																		updateProviderField(
																			idx(),
																			"display_name",
																			e.currentTarget.value,
																		)
																	}
																/>
															</label>
															<label class="form-control">
																<span class="label-text text-xs">协议</span>
																<select
																	class="select select-bordered select-sm"
																	value={p.protocol}
																	disabled={!canManage()}
																	onChange={(e) =>
																		updateProviderField(
																			idx(),
																			"protocol",
																			e.currentTarget.value,
																		)
																	}
																>
																	<For each={[...LLM_PROTOCOLS]}>
																		{(opt) => (
																			<option value={opt.value}>
																				{opt.label}
																			</option>
																		)}
																	</For>
																</select>
															</label>
															<label class="form-control">
																<span class="label-text text-xs">Model</span>
																<input
																	class="input input-bordered input-sm"
																	value={p.model}
																	disabled={!canManage()}
																	onInput={(e) =>
																		updateProviderField(
																			idx(),
																			"model",
																			e.currentTarget.value,
																		)
																	}
																/>
															</label>
															<label class="form-control md:col-span-2">
																<span class="label-text text-xs">Base URL</span>
																<input
																	class="input input-bordered input-sm"
																	value={p.base_url}
																	disabled={!canManage()}
																	placeholder="https://api.openai.com/v1"
																	onInput={(e) =>
																		updateProviderField(
																			idx(),
																			"base_url",
																			e.currentTarget.value,
																		)
																	}
																/>
															</label>
															<label class="form-control md:col-span-2">
																<span class="label-text text-xs">
																	API Key（留空表示不修改已保存密钥）
																</span>
																<input
																	type="password"
																	class="input input-bordered input-sm"
																	value={p.api_key}
																	disabled={!canManage()}
																	placeholder="sk-..."
																	onInput={(e) =>
																		updateProviderField(
																			idx(),
																			"api_key",
																			e.currentTarget.value,
																		)
																	}
																/>
															</label>
															<label class="label cursor-pointer justify-start gap-2">
																<input
																	type="checkbox"
																	class="checkbox checkbox-sm"
																	checked={p.enabled}
																	disabled={!canManage()}
																	onChange={(e) =>
																		updateProviderField(
																			idx(),
																			"enabled",
																			e.currentTarget.checked,
																		)
																	}
																/>
																<span class="label-text text-sm">
																	启用该供应商
																</span>
															</label>
														</div>
													</div>
												)}
											</For>
										</Show>
									</div>
								</ManageSection>

								<div class="flex justify-end">
									<button
										type="button"
										class="btn btn-primary gap-1"
										disabled={!canManage() || saving()}
										onClick={() => void handleSave()}
									>
										<Save size={14} />
										{saving() ? "保存中..." : "保存并重启插件"}
									</button>
								</div>
							</>
						)}
					</Show>
				</div>
			</div>
		</ManagePage>
	);
}
