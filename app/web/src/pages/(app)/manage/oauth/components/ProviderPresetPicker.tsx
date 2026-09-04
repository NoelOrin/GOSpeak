import ArrowLeft from "lucide-solid/icons/arrow-left";
import Plus from "lucide-solid/icons/plus";
import { For, Show } from "solid-js";
import ProviderIcon from "@/components/oauth/ProviderIcon";
import { PROVIDER_PRESETS, type ProviderPreset } from "./presets";

export interface ProviderPresetPickerProps {
	existingNames: Set<string>;
	onBack: () => void;
	onSelect: (preset: ProviderPreset) => void;
	onCustom: () => void;
}

export default function ProviderPresetPicker(props: ProviderPresetPickerProps) {
	return (
		<section class="rounded-2xl border border-base-300/80 bg-base-100 p-4 shadow-sm md:p-5">
			<div class="flex flex-col gap-4">
				<div class="flex items-center gap-2">
					<button
						type="button"
						class="btn btn-ghost btn-xs"
						onClick={props.onBack}
					>
						<ArrowLeft size={14} />
					</button>
					<h3 class="font-bold text-base">选择 OAuth 提供商</h3>
				</div>
				<p class="text-xs text-base-content/50">
					选择常用提供商可自动填充端点和字段映射，也可选择自定义手动配置
				</p>

				<div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
					<For each={PROVIDER_PRESETS}>
						{(preset) => {
							const alreadyExists = () => props.existingNames.has(preset.name);
							return (
								<button
									type="button"
									class="flex flex-col items-center gap-2 rounded-xl border border-base-300 p-4 transition hover:border-base-content/20 hover:bg-base-200/40"
									classList={{
										"opacity-50 cursor-not-allowed": alreadyExists(),
									}}
									disabled={alreadyExists()}
									onClick={() => props.onSelect(preset)}
								>
									<ProviderIcon name={preset.name} size={32} />
									<span class="text-sm font-medium">{preset.label}</span>
									<Show when={alreadyExists()}>
										<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2 py-0.5 text-[10px] font-medium text-base-content/70">
											已配置
										</span>
									</Show>
								</button>
							);
						}}
					</For>

					{/* 自定义 */}
					<button
						type="button"
						class="flex flex-col items-center gap-2 rounded-xl border border-dashed border-base-300 p-4 transition hover:border-base-content/20 hover:bg-base-200/40"
						onClick={props.onCustom}
					>
						<div class="flex h-8 w-8 items-center justify-center rounded-full bg-base-300">
							<Plus size={16} />
						</div>
						<span class="text-sm font-medium">自定义</span>
						<span class="text-xs text-base-content/50">手动填写端点</span>
					</button>
				</div>
			</div>
		</section>
	);
}
