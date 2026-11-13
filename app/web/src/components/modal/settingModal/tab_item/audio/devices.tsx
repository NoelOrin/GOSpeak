import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@gospeak/components";
import AudioDeviceStore, {
	type useAudioDeviceStore,
} from "@/stores/audioDeviceStore";
import { Field, Section, Toggle } from "../shared";

// Radix Select 不接受空字符串 value，空设备 id 用哨兵值映射
const DEFAULT_DEVICE = "__default__";

export const DevicesSection = ({
	store,
	testing,
	level,
	toggle,
}: {
	store: ReturnType<typeof useAudioDeviceStore.getState>;
	testing: boolean;
	level: number;
	toggle: () => void;
}) => (
	<Section
		title="设备"
		action={
			<button
				type="button"
				className="hover:bg-muted inline-flex h-7 cursor-pointer items-center rounded-sm px-2 text-xs transition-colors"
				onClick={() =>
					void AudioDeviceStore.fetchAudioDevices({ notify: true })
				}
			>
				刷新设备
			</button>
		}
	>
		{store.deviceLoadError && (
			<div
				role="alert"
				className="border-destructive/40 bg-destructive/10 text-destructive flex flex-wrap items-center gap-2 rounded-sm border px-3 py-2 text-xs"
			>
				<span>{store.deviceLoadError}</span>
				<button
					type="button"
					className="underline underline-offset-2"
					onClick={() =>
						void AudioDeviceStore.fetchAudioDevices({ notify: true })
					}
				>
					重试
				</button>
			</div>
		)}
		<Field label="输入设备（麦克风）">
			<Select
				value={store.selectedAudioInput || DEFAULT_DEVICE}
				onValueChange={(value) =>
					AudioDeviceStore.setSelectedAudioInput(
						value === DEFAULT_DEVICE ? "" : value,
					)
				}
			>
				<SelectTrigger>
					<SelectValue placeholder="选择输入设备" />
				</SelectTrigger>
				<SelectContent>
					{store.audioinputs.map((d) => (
						<SelectItem
							key={d.deviceId || DEFAULT_DEVICE}
							value={d.deviceId || DEFAULT_DEVICE}
						>
							{d.label || `麦克风 (${d.deviceId.slice(0, 8)})`}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</Field>

		<Field
			label="输出设备（扬声器）"
			hint="部分浏览器仅支持 setSinkId；不支持时将使用系统默认输出。"
		>
			<Select
				value={store.selectedAudioOutput || DEFAULT_DEVICE}
				onValueChange={(value) =>
					AudioDeviceStore.setSelectedAudioOutput(
						value === DEFAULT_DEVICE ? "" : value,
					)
				}
			>
				<SelectTrigger>
					<SelectValue placeholder="选择输出设备" />
				</SelectTrigger>
				<SelectContent>
					{store.audiooutputs.map((d) => (
						<SelectItem
							key={d.deviceId || DEFAULT_DEVICE}
							value={d.deviceId || DEFAULT_DEVICE}
						>
							{d.label || `扬声器 (${d.deviceId.slice(0, 8)})`}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</Field>

		<Toggle
			label="设备变化时提醒"
			desc="插入或拔出麦克风/扬声器时弹窗询问是否切换；关闭后静默沿用当前设备"
			checked={store.deviceChangeNotice}
			onChange={AudioDeviceStore.setDeviceChangeNotice}
		/>

		<div className="border-border bg-muted/40 rounded-lg border p-3">
			<div className="mb-2 flex items-center justify-between gap-3">
				<div>
					<div className="text-sm font-medium">麦克风测试</div>
					<div className="text-faint-foreground text-xs">
						本地电平检测，不会推流
					</div>
				</div>
				<button
					type="button"
					className={`inline-flex h-9 cursor-pointer items-center rounded-sm border px-3 text-sm transition-colors ${testing ? "border-destructive/50 text-destructive hover:bg-destructive/10" : "border-border hover:bg-muted"}`}
					onClick={() => void toggle()}
				>
					{testing ? "停止" : "开始测试"}
				</button>
			</div>
			<div className="bg-border h-2 w-full overflow-hidden rounded-full">
				<div
					className="bg-success h-full transition-[width] duration-75"
					style={{ width: `${level}%` }}
				/>
			</div>
		</div>
	</Section>
);
