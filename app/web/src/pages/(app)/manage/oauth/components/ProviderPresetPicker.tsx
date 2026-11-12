import { ArrowLeft, Plus } from "lucide-react";
import ProviderIcon from "@/components/oauth/ProviderIcon";
import { PROVIDER_PRESETS, type ProviderPreset } from "./presets";

export interface ProviderPresetPickerProps {
	existingNames: Set<string>;
	onBack: () => void;
	onSelect: (preset: ProviderPreset) => void;
	onCustom: () => void;
}

export default function ProviderPresetPicker({
	existingNames,
	onBack,
	onSelect,
	onCustom,
}: ProviderPresetPickerProps) {
	return (
		<section className="border-border/80 bg-card rounded-lg border p-4 shadow-sm md:p-5">
			<div className="flex flex-col gap-4">
				<div className="flex items-center gap-2">
					<button
						type="button"
						className="hover:bg-muted inline-flex size-7 cursor-pointer items-center justify-center rounded-sm transition-colors"
						onClick={onBack}
					>
						<ArrowLeft size={14} />
					</button>
					<h3 className="text-base font-bold">选择 OAuth 提供商</h3>
				</div>
				<p className="text-faint-foreground text-xs">
					选择常用提供商可自动填充端点和字段映射，也可选择自定义手动配置
				</p>

				<div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
					{PROVIDER_PRESETS.map((preset) => {
						const alreadyExists = existingNames.has(preset.name);
						return (
							<button
								key={preset.name}
								type="button"
								className={`border-border hover:border-foreground/20 hover:bg-muted/40 flex flex-col items-center gap-2 rounded-lg border p-4 transition ${alreadyExists ? "pointer-events-none opacity-50" : "cursor-pointer"}`}
								disabled={alreadyExists}
								onClick={() => onSelect(preset)}
							>
								<ProviderIcon name={preset.name} size={32} />
								<span className="text-sm font-medium">{preset.label}</span>
								{alreadyExists && (
									<span className="border-border bg-card text-muted-foreground inline-flex items-center rounded-full border px-2 py-0.5 text-2xs font-medium">
										已配置
									</span>
								)}
							</button>
						);
					})}

					{/* 自定义 */}
					<button
						type="button"
						className="border-border hover:border-foreground/20 hover:bg-muted/40 flex cursor-pointer flex-col items-center gap-2 rounded-lg border border-dashed p-4 transition"
						onClick={onCustom}
					>
						<div className="bg-border flex h-8 w-8 items-center justify-center rounded-full">
							<Plus size={16} />
						</div>
						<span className="text-sm font-medium">自定义</span>
						<span className="text-faint-foreground text-xs">手动填写端点</span>
					</button>
				</div>
			</div>
		</section>
	);
}
