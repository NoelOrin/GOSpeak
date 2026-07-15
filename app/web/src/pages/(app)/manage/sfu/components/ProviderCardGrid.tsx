import type { SFUProvider } from "@gospeak/sfu-client/types";
import Check from "lucide-solid/icons/check";
import CircleAlert from "lucide-solid/icons/circle-alert";
import { For, Show } from "solid-js";
import type { SFUConfig } from "@/api/sfu";
import {
	DISABLED_PROVIDERS,
	isProviderConfigured,
	PROVIDER_OPTIONS,
} from "./constants";

export interface ProviderCardGridProps {
	activeProvider: SFUProvider;
	selectedProvider?: SFUProvider;
	providers: Array<Partial<SFUConfig> & { provider: SFUProvider }>;
	disabled?: boolean;
	onSelect: (provider: SFUProvider) => void;
}

export default function ProviderCardGrid(props: ProviderCardGridProps) {
	const isProviderDisabled = (provider: SFUProvider) =>
		DISABLED_PROVIDERS.includes(provider);

	return (
		<div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
			<For each={PROVIDER_OPTIONS}>
				{(option) => {
					const isActive = () => option.value === props.activeProvider;
					const isSelected = () => option.value === props.selectedProvider;
					const configured = () => {
						const cfg = props.providers.find(
							(c) => c.provider === option.value,
						);
						return cfg ? isProviderConfigured(option.value, cfg) : false;
					};
					return (
						<button
							type="button"
							class="btn h-auto min-h-0 justify-start gap-3 rounded-box border px-3 py-3 text-left"
							classList={{
								"btn-primary": isSelected(),
								"btn-ghost border-base-300": !isSelected(),
								"opacity-60": isProviderDisabled(option.value),
							}}
							disabled={props.disabled || isProviderDisabled(option.value)}
							onClick={() => props.onSelect(option.value)}
						>
							<div class="flex flex-col items-start gap-1">
								<div class="flex items-center gap-2">
									<span class="font-semibold">{option.label}</span>
									<Show when={isActive()}>
										<span class="badge badge-sm badge-success gap-1">
											<Check size={12} />
											当前
										</span>
									</Show>
								</div>
								<div class="flex items-center gap-1">
									<span
										class="inline-block size-2 rounded-full"
										classList={{
											"bg-success": configured(),
											"bg-base-content/30": !configured(),
										}}
									/>
									<span class="text-xs opacity-70">
										{configured() ? "已配置" : "未配置"}
									</span>
								</div>
								<Show when={isProviderDisabled(option.value)}>
									<span class="text-xs text-warning flex items-center gap-1">
										<CircleAlert size={12} />
										暂不可用
									</span>
								</Show>
							</div>
						</button>
					);
				}}
			</For>
		</div>
	);
}
