import { For, Show } from "solid-js";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import Trash2 from "lucide-solid/icons/trash-2";
import { LLM_PROTOCOLS, type LLMProviderForm, protocolLabel } from "./shared";

export type ProviderUpdate = <K extends keyof LLMProviderForm>(
	index: number,
	key: K,
	value: LLMProviderForm[K],
) => void;

export interface ProviderCardProps {
	provider: LLMProviderForm;
	index: number;
	canManage: boolean;
	modelOptions: string[];
	loadingModels: boolean;
	onUpdate: ProviderUpdate;
	onRemove: (index: number) => void;
	onFetchModels: (index: number) => void;
}

export function ProviderCard(props: ProviderCardProps) {
	const {
		provider,
		index,
		canManage,
		modelOptions,
		loadingModels,
		onUpdate,
		onRemove,
		onFetchModels,
	} = props;
	const updateProviderField = onUpdate;
	return (
		<div class="rounded-2xl border border-base-300/80 bg-base-100 p-4 shadow-sm">
			<div class="mb-3 flex flex-wrap items-center justify-between gap-2">
				<div class="flex min-w-0 flex-wrap items-center gap-2">
					<div class="text-sm font-semibold text-base-content">
						供应商 #{index + 1}
					</div>
					<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2 py-0.5 text-[11px] font-medium text-base-content/65">
						{protocolLabel(provider.protocol)}
					</span>
					<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2 py-0.5 text-[11px] font-medium text-base-content/65">
						{provider.enabled ? "已启用" : "已停用"}
					</span>
				</div>
				<button
					type="button"
					class="btn btn-ghost btn-xs text-error"
					disabled={!canManage}
					onClick={() => onRemove(index)}
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
						value={provider.name}
						disabled={!canManage}
						placeholder="openai"
						onInput={(e) =>
							updateProviderField(index, "name", e.currentTarget.value)
						}
					/>
				</label>
				<label class="block">
					<span class="mb-1.5 block text-xs font-medium text-base-content/60">
						显示名
					</span>
					<input
						class="input input-bordered input-sm w-full"
						value={provider.display_name}
						disabled={!canManage}
						placeholder="OpenAI"
						onInput={(e) =>
							updateProviderField(index, "display_name", e.currentTarget.value)
						}
					/>
				</label>
				<label class="block">
					<span class="mb-1.5 block text-xs font-medium text-base-content/60">
						协议
					</span>
					<select
						class="select select-bordered select-sm w-full"
						value={provider.protocol}
						disabled={!canManage}
						onChange={(e) =>
							updateProviderField(index, "protocol", e.currentTarget.value)
						}
					>
						<For each={[...LLM_PROTOCOLS]}>
							{(opt) => <option value={opt.value}>{opt.label}</option>}
						</For>
					</select>
				</label>
				<label class="block md:col-span-2">
					<span class="mb-1.5 block text-xs font-medium text-base-content/60">
						Base URL
					</span>
					<input
						class="input input-bordered input-sm w-full"
						value={provider.base_url}
						disabled={!canManage}
						placeholder="https://api.openai.com/v1"
						onInput={(e) =>
							updateProviderField(index, "base_url", e.currentTarget.value)
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
						value={provider.api_key}
						disabled={!canManage}
						placeholder="sk-..."
						onInput={(e) =>
							updateProviderField(index, "api_key", e.currentTarget.value)
						}
					/>
				</label>
				<div class="md:col-span-2">
					<div class="mb-1.5 flex items-center justify-between gap-2">
						<span class="text-xs font-medium text-base-content/60">Model</span>
						<button
							type="button"
							class="btn btn-ghost btn-xs gap-1"
							disabled={!canManage || !!loadingModels}
							onClick={() => onFetchModels(index)}
						>
							<RefreshCcw
								size={12}
								classList={{
									"animate-spin": !!loadingModels,
								}}
							/>
							{loadingModels ? "拉取中" : "拉取模型"}
						</button>
					</div>
					<div class="grid grid-cols-1 gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
						<select
							class="select select-bordered select-sm w-full"
							value={provider.model}
							disabled={!canManage || (modelOptions?.length ?? 0) === 0}
							onChange={(e) =>
								updateProviderField(index, "model", e.currentTarget.value)
							}
						>
							<option value="">
								{(modelOptions?.length ?? 0) > 0
									? "选择模型"
									: "先拉取模型列表"}
							</option>
							{/* 当前已选但不在列表中时保留 */}
							<Show
								when={
									provider.model &&
									!(modelOptions ?? []).includes(provider.model)
								}
							>
								<option value={provider.model}>{provider.model}</option>
							</Show>
							<For each={modelOptions ?? []}>
								{(m) => <option value={m}>{m}</option>}
							</For>
						</select>
						<input
							class="input input-bordered input-sm w-full"
							value={provider.model}
							disabled={!canManage}
							placeholder="也可手工填写模型 ID"
							onInput={(e) =>
								updateProviderField(index, "model", e.currentTarget.value)
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
					"border-primary/25 bg-primary/8": provider.enabled,
					"border-base-300/80 bg-base-200/20": !provider.enabled,
					"cursor-not-allowed opacity-60": !canManage,
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
					checked={provider.enabled}
					disabled={!canManage}
					onChange={(e) =>
						updateProviderField(index, "enabled", e.currentTarget.checked)
					}
				/>
			</label>
		</div>
	);
}
