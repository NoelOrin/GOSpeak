import Plus from "lucide-solid/icons/plus";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import Save from "lucide-solid/icons/save";
import Trash2 from "lucide-solid/icons/trash-2";
import X from "lucide-solid/icons/x";
import { type Accessor, createSignal, For, Index, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { listPluginModels, type PluginInfo } from "@/api/plugin";
import {
	emptyProvider,
	kindLabel,
	LLM_PROTOCOLS,
	type LLMProviderForm,
	protocolLabel,
	statusMeta,
} from "./shared";

export interface PluginSettingsModalProps {
	open: Accessor<boolean>;
	plugin: Accessor<PluginInfo | null>;
	canManage: Accessor<boolean>;
	saving: Accessor<boolean>;
	enabled: Accessor<boolean>;
	setEnabled: (value: boolean) => void;
	sideEnabled: Accessor<boolean>;
	setSideEnabled: (value: boolean) => void;
	sideAddr: Accessor<string>;
	setSideAddr: (value: string) => void;
	defaultProvider: Accessor<string>;
	setDefaultProvider: (value: string) => void;
	providers: Accessor<LLMProviderForm[]>;
	setProviders: (
		value: LLMProviderForm[] | ((prev: LLMProviderForm[]) => LLMProviderForm[]),
	) => void;
	onClose: () => void;
	onSave: () => void;
}

export default function PluginSettingsModal(props: PluginSettingsModalProps) {
	const meta = () => statusMeta(props.plugin()?.status);
	const [modelOptions, setModelOptions] = createSignal<
		Record<number, string[]>
	>({});
	const [loadingModels, setLoadingModels] = createSignal<
		Record<number, boolean>
	>({});

	const addProvider = () => {
		props.setProviders((prev) => [...prev, emptyProvider()]);
	};

	const removeProvider = (idx: number) => {
		props.setProviders((prev) => prev.filter((_, i) => i !== idx));
	};

	const updateProviderField = <K extends keyof LLMProviderForm>(
		idx: number,
		key: K,
		value: LLMProviderForm[K],
	) => {
		props.setProviders((prev) =>
			prev.map((p, i) => (i === idx ? { ...p, [key]: value } : p)),
		);
		// base_url / protocol / api_key 变化后清空旧模型列表，避免错配
		if (key === "base_url" || key === "protocol" || key === "api_key") {
			setModelOptions((prev) => {
				const next = { ...prev };
				delete next[idx];
				return next;
			});
		}
	};

	const fetchModels = async (idx: number) => {
		const pluginName = props.plugin()?.name;
		if (!pluginName) return;
		const provider = props.providers()[idx];
		if (!provider) return;
		if (!provider.base_url.trim()) {
			showToast("请先填写 Base URL", { type: "warning" });
			return;
		}
		setLoadingModels((prev) => ({ ...prev, [idx]: true }));
		try {
			const models = await listPluginModels(pluginName, {
				provider: provider.name.trim() || undefined,
				protocol: provider.protocol,
				base_url: provider.base_url.trim(),
				api_key: provider.api_key.trim() || undefined,
			});
			setModelOptions((prev) => ({ ...prev, [idx]: models }));
			if (models.length === 0) {
				showToast("未获取到模型，可手工填写", { type: "warning" });
			} else {
				showToast(`已获取 ${models.length} 个模型`, { type: "success" });
				// 若当前 model 为空，自动选第一个；否则保留用户已选
				if (!provider.model.trim()) {
					updateProviderField(idx, "model", models[0]);
				}
			}
		} catch (e: any) {
			showToast(e?.message || "获取模型列表失败", { type: "error" });
		} finally {
			setLoadingModels((prev) => ({ ...prev, [idx]: false }));
		}
	};

	return (
		<dialog
			class="modal"
			classList={{ "modal-open": props.open() }}
			onClose={props.onClose}
		>
			<div class="modal-box flex max-h-[88vh] w-full max-w-4xl flex-col gap-0 overflow-hidden rounded-2xl p-0">
				<div class="flex shrink-0 items-start justify-between gap-3 border-b border-base-300/70 px-5 py-4">
					<div class="min-w-0">
						<div class="flex flex-wrap items-center gap-2">
							<h3 class="truncate text-base font-bold">
								{props.plugin()?.display_name ||
									props.plugin()?.name ||
									"插件设置"}
							</h3>
							<span
								class={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] font-medium ${meta().chip}`}
							>
								<span class={`size-1.5 rounded-full ${meta().dot}`} />
								{meta().label}
							</span>
						</div>
						<p class="mt-1 line-clamp-2 text-xs text-base-content/50">
							{props.plugin()?.desc ||
								"配置插件开关、Side Server 与大模型供应商"}
						</p>
					</div>
					<button
						type="button"
						class="btn btn-ghost btn-sm btn-square"
						onClick={props.onClose}
						aria-label="关闭"
					>
						<X size={16} />
					</button>
				</div>

				<div class="min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-4">
					<section class="space-y-3">
						<div class="flex flex-wrap items-center gap-2">
							<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2.5 py-1 text-[11px] font-medium text-base-content/70">
								{kindLabel(props.plugin()?.kind)}
							</span>
							<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2.5 py-1 text-[11px] font-medium text-base-content/70">
								作者 {props.plugin()?.author || "-"}
							</span>
							<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2.5 py-1 text-[11px] font-medium text-base-content/70">
								版本 {props.plugin()?.version || "-"}
							</span>
							<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2.5 py-1 font-mono text-[11px] font-medium text-base-content/55">
								{props.plugin()?.name}
							</span>
						</div>

						<Show when={props.plugin()?.error}>
							<div class="rounded-2xl border border-base-300 bg-base-200/40 px-4 py-3 text-sm text-base-content/70">
								{props.plugin()?.error}
							</div>
						</Show>

						<Show when={(props.plugin()?.side_servers?.length ?? 0) > 0}>
							<div class="rounded-2xl border border-base-300/80 bg-base-200/30 p-3.5">
								<div class="mb-2 text-xs font-semibold text-base-content/55">
									当前 Side Servers
								</div>
								<div class="space-y-2">
									<For each={props.plugin()?.side_servers ?? []}>
										{(s) => (
											<div class="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-base-300/70 bg-base-100 px-3 py-2 text-sm">
												<span class="font-medium text-base-content/80">
													{s.name}
												</span>
												<span class="font-mono text-xs text-base-content/55">
													{s.url}
												</span>
											</div>
										)}
									</For>
								</div>
							</div>
						</Show>
					</section>

					<section class="space-y-3">
						<div>
							<div class="text-sm font-semibold">基础开关</div>
							<p class="mt-0.5 text-xs text-base-content/50">
								启用插件并配置可选小服务端
							</p>
						</div>

						<div class="grid gap-3 md:grid-cols-2">
							<label
								class="flex cursor-pointer items-center justify-between gap-3 rounded-2xl border px-4 py-3.5 transition-colors"
								classList={{
									"border-primary/25 bg-primary/8": props.enabled(),
									"border-base-300/80 bg-base-100 hover:bg-base-200/30":
										!props.enabled(),
									"cursor-not-allowed opacity-60": !props.canManage(),
								}}
							>
								<div class="min-w-0">
									<div class="text-sm font-medium">启用插件</div>
									<div class="mt-0.5 text-xs text-base-content/50">
										关闭后插件不会加载运行
									</div>
								</div>
								<input
									type="checkbox"
									class="toggle"
									checked={props.enabled()}
									disabled={!props.canManage()}
									onChange={(e) => props.setEnabled(e.currentTarget.checked)}
								/>
							</label>

							<label
								class="flex cursor-pointer items-center justify-between gap-3 rounded-2xl border px-4 py-3.5 transition-colors"
								classList={{
									"border-primary/25 bg-primary/8": props.sideEnabled(),
									"border-base-300/80 bg-base-100 hover:bg-base-200/30":
										!props.sideEnabled(),
									"cursor-not-allowed opacity-60": !props.canManage(),
								}}
							>
								<div class="min-w-0">
									<div class="text-sm font-medium">启用 Side Server</div>
									<div class="mt-0.5 text-xs text-base-content/50">
										由插件自启独立小服务
									</div>
								</div>
								<input
									type="checkbox"
									class="toggle"
									checked={props.sideEnabled()}
									disabled={!props.canManage()}
									onChange={(e) =>
										props.setSideEnabled(e.currentTarget.checked)
									}
								/>
							</label>
						</div>

						<div>
							<label
								class="mb-1.5 block text-xs font-medium text-base-content/60"
								for="botbase-side-server-addr"
							>
								Side Server 地址
							</label>
							<input
								id="botbase-side-server-addr"
								type="text"
								class="input input-bordered input-sm w-full max-w-xl"
								value={props.sideAddr()}
								disabled={!props.canManage() || !props.sideEnabled()}
								placeholder="127.0.0.1:9200 或 127.0.0.1:0"
								onInput={(e) => props.setSideAddr(e.currentTarget.value)}
							/>
							<p class="mt-1.5 text-[11px] text-base-content/45">
								端口写 0 时由运行时自动分配。
							</p>
						</div>
					</section>

					<section class="space-y-3">
						<div class="flex flex-wrap items-start justify-between gap-3">
							<div>
								<div class="text-sm font-semibold">大模型供应商</div>
								<p class="mt-0.5 text-xs text-base-content/50">
									多供应商 / 多协议配置
								</p>
							</div>
							<button
								type="button"
								class="btn btn-ghost btn-sm gap-1.5"
								disabled={!props.canManage()}
								onClick={addProvider}
							>
								<Plus size={14} />
								添加供应商
							</button>
						</div>

						<div class="rounded-2xl border border-base-300/80 bg-base-200/25 p-3.5">
							<label
								class="mb-1.5 block text-xs font-medium text-base-content/60"
								for="botbase-default-provider"
							>
								默认供应商
							</label>
							<select
								id="botbase-default-provider"
								class="select select-bordered select-sm w-full max-w-md"
								value={props.defaultProvider()}
								disabled={!props.canManage()}
								onChange={(e) =>
									props.setDefaultProvider(e.currentTarget.value)
								}
							>
								<option value="">（未指定）</option>
								<For each={props.providers().filter((p) => p.name.trim())}>
									{(p) => (
										<option value={p.name}>{p.display_name || p.name}</option>
									)}
								</For>
							</select>
						</div>

						<div class="space-y-3">
							<Show
								when={props.providers().length > 0}
								fallback={
									<div class="rounded-2xl border border-dashed border-base-300 px-4 py-10 text-center">
										<div class="text-sm font-medium text-base-content/70">
											还没有供应商
										</div>
										<div class="mt-1 text-xs text-base-content/45">
											添加 OpenAI、Gemini、Ollama 等协议配置
										</div>
										<button
											type="button"
											class="btn btn-outline btn-sm mt-4 gap-1.5"
											disabled={!props.canManage()}
											onClick={addProvider}
										>
											<Plus size={14} />
											添加供应商
										</button>
									</div>
								}
							>
								<Index each={props.providers()}>
									{(p, idx) => (
										<div class="rounded-2xl border border-base-300/80 bg-base-100 p-4 shadow-sm">
											<div class="mb-3 flex flex-wrap items-center justify-between gap-2">
												<div class="flex min-w-0 flex-wrap items-center gap-2">
													<div class="text-sm font-semibold text-base-content">
														供应商 #{idx + 1}
													</div>
													<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2 py-0.5 text-[11px] font-medium text-base-content/65">
														{protocolLabel(p().protocol)}
													</span>
													<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2 py-0.5 text-[11px] font-medium text-base-content/65">
														{p().enabled ? "已启用" : "已停用"}
													</span>
												</div>
												<button
													type="button"
													class="btn btn-ghost btn-xs text-error"
													disabled={!props.canManage()}
													onClick={() => removeProvider(idx)}
													aria-label="删除供应商"
												>
													<Trash2 size={14} />
												</button>
											</div>

											<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
												<label class="block">
													<span class="mb-1.5 block text-xs font-medium text-base-content/60">
														Name
													</span>
													<input
														class="input input-bordered input-sm w-full"
														value={p().name}
														disabled={!props.canManage()}
														placeholder="openai"
														onInput={(e) =>
															updateProviderField(
																idx,
																"name",
																e.currentTarget.value,
															)
														}
													/>
												</label>
												<label class="block">
													<span class="mb-1.5 block text-xs font-medium text-base-content/60">
														显示名
													</span>
													<input
														class="input input-bordered input-sm w-full"
														value={p().display_name}
														disabled={!props.canManage()}
														placeholder="OpenAI"
														onInput={(e) =>
															updateProviderField(
																idx,
																"display_name",
																e.currentTarget.value,
															)
														}
													/>
												</label>
												<label class="block">
													<span class="mb-1.5 block text-xs font-medium text-base-content/60">
														协议
													</span>
													<select
														class="select select-bordered select-sm w-full"
														value={p().protocol}
														disabled={!props.canManage()}
														onChange={(e) =>
															updateProviderField(
																idx,
																"protocol",
																e.currentTarget.value,
															)
														}
													>
														<For each={[...LLM_PROTOCOLS]}>
															{(opt) => (
																<option value={opt.value}>{opt.label}</option>
															)}
														</For>
													</select>
												</label>
												<label class="block md:col-span-2">
													<span class="mb-1.5 block text-xs font-medium text-base-content/60">
														Base URL
													</span>
													<input
														class="input input-bordered input-sm w-full"
														value={p().base_url}
														disabled={!props.canManage()}
														placeholder="https://api.openai.com/v1"
														onInput={(e) =>
															updateProviderField(
																idx,
																"base_url",
																e.currentTarget.value,
															)
														}
													/>
												</label>
												<label class="block md:col-span-2">
													<span class="mb-1.5 block text-xs font-medium text-base-content/60">
														API Key（留空表示不修改已保存密钥）
													</span>
													<input
														type="password"
														class="input input-bordered input-sm w-full"
														value={p().api_key}
														disabled={!props.canManage()}
														placeholder="sk-..."
														onInput={(e) =>
															updateProviderField(
																idx,
																"api_key",
																e.currentTarget.value,
															)
														}
													/>
												</label>
												<div class="md:col-span-2">
													<div class="mb-1.5 flex items-center justify-between gap-2">
														<span class="text-xs font-medium text-base-content/60">
															Model
														</span>
														<button
															type="button"
															class="btn btn-ghost btn-xs gap-1"
															disabled={
																!props.canManage() || !!loadingModels()[idx]
															}
															onClick={() => void fetchModels(idx)}
														>
															<RefreshCcw
																size={12}
																classList={{
																	"animate-spin": !!loadingModels()[idx],
																}}
															/>
															{loadingModels()[idx] ? "拉取中" : "拉取模型"}
														</button>
													</div>
													<div class="grid grid-cols-1 gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
														<select
															class="select select-bordered select-sm w-full"
															value={p().model}
															disabled={
																!props.canManage() ||
																(modelOptions()[idx]?.length ?? 0) === 0
															}
															onChange={(e) =>
																updateProviderField(
																	idx,
																	"model",
																	e.currentTarget.value,
																)
															}
														>
															<option value="">
																{(modelOptions()[idx]?.length ?? 0) > 0
																	? "选择模型"
																	: "先拉取模型列表"}
															</option>
															{/* 当前已选但不在列表中时保留 */}
															<Show
																when={
																	p().model &&
																	!(modelOptions()[idx] ?? []).includes(
																		p().model,
																	)
																}
															>
																<option value={p().model}>{p().model}</option>
															</Show>
															<For each={modelOptions()[idx] ?? []}>
																{(m) => <option value={m}>{m}</option>}
															</For>
														</select>
														<input
															class="input input-bordered input-sm w-full"
															value={p().model}
															disabled={!props.canManage()}
															placeholder="也可手工填写模型 ID"
															onInput={(e) =>
																updateProviderField(
																	idx,
																	"model",
																	e.currentTarget.value,
																)
															}
														/>
													</div>
													<p class="mt-1.5 text-[11px] text-base-content/45">
														使用当前 Base URL / API Key 拉取模型，再从列表选择。
													</p>
												</div>
											</div>

											<label
												class="mt-3 flex cursor-pointer items-center justify-between gap-3 rounded-xl border px-3.5 py-2.5"
												classList={{
													"border-primary/25 bg-primary/8": p().enabled,
													"border-base-300/80 bg-base-200/20": !p().enabled,
													"cursor-not-allowed opacity-60": !props.canManage(),
												}}
											>
												<div>
													<div class="text-sm font-medium">启用该供应商</div>
													<div class="text-[11px] text-base-content/45">
														仅启用状态可参与默认路由
													</div>
												</div>
												<input
													type="checkbox"
													class="toggle toggle-sm"
													checked={p().enabled}
													disabled={!props.canManage()}
													onChange={(e) =>
														updateProviderField(
															idx,
															"enabled",
															e.currentTarget.checked,
														)
													}
												/>
											</label>
										</div>
									)}
								</Index>
							</Show>
						</div>
					</section>
				</div>

				<div class="flex shrink-0 flex-wrap items-center justify-between gap-3 border-t border-base-300/70 bg-base-100 px-5 py-3.5">
					<div class="text-xs text-base-content/50">
						保存后会重启插件以应用最新配置
					</div>
					<div class="flex items-center gap-2">
						<button
							type="button"
							class="btn btn-ghost btn-sm"
							onClick={props.onClose}
						>
							取消
						</button>
						<button
							type="button"
							class="btn btn-neutral btn-sm gap-1.5"
							disabled={!props.canManage() || props.saving()}
							onClick={() => void props.onSave()}
						>
							<Save size={14} />
							{props.saving() ? "保存中..." : "保存并重启"}
						</button>
					</div>
				</div>
			</div>
			<form method="dialog" class="modal-backdrop" onSubmit={props.onClose}>
				<button type="submit" class="cursor-default">
					close
				</button>
			</form>
		</dialog>
	);
}
