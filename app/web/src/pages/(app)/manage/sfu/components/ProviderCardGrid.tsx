import { Check, Radio } from "lucide-react";
import type { SFUProvider } from "@gospeak/sfu-client/types";
import type { SFUConfig } from "@/api/sfu";
import { isProviderConfigured, PROVIDER_OPTIONS } from "./constants";

export interface ProviderCardGridProps {
	activeProvider: SFUProvider;
	selectedProvider?: SFUProvider;
	providers: Array<Partial<SFUConfig> & { provider: SFUProvider }>;
	disabled?: boolean;
	onSelect: (provider: SFUProvider) => void;
}

const PROVIDER_ICONS: Record<SFUProvider, typeof Radio> = {
	livekit: Radio,
};

export default function ProviderCardGrid({
	activeProvider,
	selectedProvider,
	providers,
	disabled,
	onSelect,
}: ProviderCardGridProps) {
	return (
		<div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
			{PROVIDER_OPTIONS.map((option) => {
				const isActive = option.value === activeProvider;
				const isSelected = option.value === selectedProvider;
				const cfg = providers.find((c) => c.provider === option.value);
				const configured = cfg
					? isProviderConfigured(option.value, cfg)
					: false;
				const Icon = PROVIDER_ICONS[option.value];
				return (
					<button
						key={option.value}
						type="button"
						className={`group relative flex min-h-[5.25rem] cursor-pointer flex-col justify-between gap-3 overflow-hidden rounded-lg border px-4 py-3.5 text-left transition-all duration-150 ${
							isSelected && !disabled
								? "border-foreground/25 bg-card shadow-sm ring-foreground/10 ring-1"
								: !isSelected && !disabled
									? "border-border/80 bg-card hover:border-foreground/15 hover:bg-muted/40 hover:shadow-sm"
									: "border-border bg-muted/30 opacity-55 cursor-not-allowed"
						}`}
						disabled={disabled}
						onClick={() => onSelect(option.value)}
					>
						<div className="flex w-full items-start justify-between gap-3">
							<div className="flex min-w-0 items-center gap-2.5">
								<span
									className={`flex size-9 shrink-0 items-center justify-center rounded-lg border ${
										isSelected
											? "border-foreground/20 bg-muted text-foreground"
											: "border-border bg-muted/70 text-muted-foreground"
									}`}
								>
									<Icon size={16} />
								</span>
								<div className="min-w-0">
									<div className="flex min-w-0 items-center gap-2">
										<span className="truncate text-sm font-semibold">
											{option.label}
										</span>
										{isActive && (
											<span className="border-border bg-card text-muted-foreground inline-flex shrink-0 items-center gap-1 rounded-full border px-1.5 py-0.5 text-2xs font-semibold tracking-wide">
												<Check size={12} />
												当前
											</span>
										)}
									</div>
									<div className="text-muted-foreground mt-1 flex items-center gap-1.5 text-xs">
										<span
											className={`inline-block size-1.5 rounded-full ${configured ? "bg-foreground/55" : "bg-foreground/25"}`}
										/>
										<span>{configured ? "已配置" : "未配置"}</span>
									</div>
								</div>
							</div>

							<span
								className={`mt-1 size-2.5 shrink-0 rounded-full border ${
									isSelected
										? "border-foreground/40 bg-foreground/70"
										: "border-foreground/20 bg-transparent"
								}`}
							/>
						</div>

						<div className="flex min-h-4 items-center justify-between gap-2">
							<span className="text-faint-foreground text-2xs">
								{isActive ? "运行中" : "点击选择"}
							</span>
						</div>
					</button>
				);
			})}
		</div>
	);
}
