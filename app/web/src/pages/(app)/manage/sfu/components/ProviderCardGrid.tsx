import type { SFUProvider } from "@gospeak/sfu-client/types";
import Check from "lucide-solid/icons/check";
import Cloud from "lucide-solid/icons/cloud";
import Radio from "lucide-solid/icons/radio";
import Server from "lucide-solid/icons/server";
import { For, type JSX, Show } from "solid-js";
import type { SFUConfig } from "@/api/sfu";
import { isProviderConfigured, PROVIDER_OPTIONS } from "./constants";

export interface ProviderCardGridProps {
	activeProvider: SFUProvider;
	selectedProvider?: SFUProvider;
	providers: Array<Partial<SFUConfig> & { provider: SFUProvider }>;
	disabled?: boolean;
	onSelect: (provider: SFUProvider) => void;
}

const PROVIDER_ICONS: Record<SFUProvider, () => JSX.Element> = {
	livekit: () => <Radio size={16} />,
	agora: () => <Radio size={16} />,
	srs: () => <Server size={16} />,
	cloudflare: () => <Cloud size={16} />,
};

export default function ProviderCardGrid(props: ProviderCardGridProps) {
	return (
		<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
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
					const disabled = () => !!props.disabled;

					return (
						<button
							type="button"
							class="group relative flex min-h-[5.25rem] flex-col justify-between gap-3 overflow-hidden rounded-2xl border px-4 py-3.5 text-left transition-all duration-150"
							classList={{
								"border-base-content/25 bg-base-100 shadow-sm ring-1 ring-base-content/10":
									isSelected() && !disabled(),
								"border-base-300/80 bg-base-100 hover:border-base-content/15 hover:bg-base-200/40 hover:shadow-sm":
									!isSelected() && !disabled(),
								"border-base-300 bg-base-200/30 opacity-55": disabled(),
								"cursor-not-allowed": disabled(),
							}}
							disabled={disabled()}
							onClick={() => props.onSelect(option.value)}
						>
							<div class="flex w-full items-start justify-between gap-3">
								<div class="flex min-w-0 items-center gap-2.5">
									<span
										class="flex size-9 shrink-0 items-center justify-center rounded-xl border"
										classList={{
											"border-base-content/20 bg-base-200 text-base-content":
												isSelected(),
											"border-base-300 bg-base-200/70 text-base-content/65":
												!isSelected(),
										}}
									>
										{PROVIDER_ICONS[option.value]()}
									</span>
									<div class="min-w-0">
										<div class="flex min-w-0 items-center gap-2">
											<span
												class="truncate text-sm font-semibold"
												classList={{
													"text-base-content": true,
												}}
											>
												{option.label}
											</span>
											<Show when={isActive()}>
												<span class="inline-flex shrink-0 items-center gap-1 rounded-full border border-base-300 bg-base-100 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide text-base-content/70">
													<Check size={10} />
													当前
												</span>
											</Show>
										</div>
										<div class="mt-1 flex items-center gap-1.5 text-xs text-base-content/55">
											<span
												class="inline-block size-1.5 rounded-full"
												classList={{
													"bg-base-content/55": configured(),
													"bg-base-content/25": !configured(),
												}}
											/>
											<span>{configured() ? "已配置" : "未配置"}</span>
										</div>
									</div>
								</div>

								<span
									class="mt-1 size-2.5 shrink-0 rounded-full border"
									classList={{
										"border-base-content/40 bg-base-content/70": isSelected(),
										"border-base-content/20 bg-transparent": !isSelected(),
									}}
								/>
							</div>

							<div class="flex min-h-4 items-center justify-between gap-2">
								<span class="text-[11px] text-base-content/40">
									{isActive() ? "运行中" : "点击选择"}
								</span>
							</div>
						</button>
					);
				}}
			</For>
		</div>
	);
}
