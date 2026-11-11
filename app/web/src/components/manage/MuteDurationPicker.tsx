import { Clock, Infinity as InfinityIcon } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { formatDurationSeconds, formatMuteEndsAt } from "@/utils/format";

export type MuteDurationValue = {
	permanent: boolean;
	duration: number; // seconds
};

export interface MuteDurationPickerProps {
	permanent: boolean;
	duration: number;
	onChange: (value: MuteDurationValue) => void;
}

type Preset = {
	id: string;
	label: string;
	permanent?: boolean;
	seconds?: number;
};

const PRESETS: Preset[] = [
	{ id: "10m", label: "10 分钟", seconds: 10 * 60 },
	{ id: "30m", label: "30 分钟", seconds: 30 * 60 },
	{ id: "1h", label: "1 小时", seconds: 60 * 60 },
	{ id: "6h", label: "6 小时", seconds: 6 * 60 * 60 },
	{ id: "1d", label: "1 天", seconds: 24 * 60 * 60 },
	{ id: "7d", label: "7 天", seconds: 7 * 24 * 60 * 60 },
	{ id: "perm", label: "永久", permanent: true },
];

function splitSeconds(total: number) {
	const safe = Math.max(0, Math.floor(total || 0));
	return {
		days: Math.floor(safe / 86400),
		hours: Math.floor((safe % 86400) / 3600),
		minutes: Math.floor((safe % 3600) / 60),
	};
}

function isPresetSeconds(seconds: number) {
	return PRESETS.some((p) => !p.permanent && p.seconds === seconds);
}

export default function MuteDurationPicker({
	permanent,
	duration,
	onChange,
}: MuteDurationPickerProps) {
	const initial = splitSeconds(duration || 3600);
	const [days, setDays] = useState(initial.days);
	const [hours, setHours] = useState(initial.hours);
	const [minutes, setMinutes] = useState(initial.minutes);
	const [customMode, setCustomMode] = useState(
		!permanent && !isPresetSeconds(duration || 3600),
	);

	const totalSeconds = useMemo(
		() => days * 86400 + hours * 3600 + minutes * 60,
		[days, hours, minutes],
	);

	const activePresetId = (() => {
		if (permanent) return "perm";
		if (customMode || !isPresetSeconds(duration)) return "custom";
		const match = PRESETS.find((p) => !p.permanent && p.seconds === duration);
		return match?.id ?? "custom";
	})();

	// 外部重置（提交后清空）时同步内部状态
	// biome-ignore lint/correctness/useExhaustiveDependencies: 仅在 permanent/duration 变化时拆分时长，setter 稳定，不应因父组件渲染反复回写
	useEffect(() => {
		if (permanent) {
			setCustomMode(false);
			return;
		}
		if (isPresetSeconds(duration)) {
			setCustomMode(false);
			const parts = splitSeconds(duration);
			setDays(parts.days);
			setHours(parts.hours);
			setMinutes(parts.minutes);
			return;
		}
		// 自定义值：仅在总秒变化时回填，避免输入时抖动
		if (duration !== totalSeconds) {
			const parts = splitSeconds(duration);
			setCustomMode(true);
			setDays(parts.days);
			setHours(parts.hours);
			setMinutes(parts.minutes);
		}
	}, [permanent, duration]);

	const emitTimed = (seconds: number) => {
		onChange({
			permanent: false,
			duration: Math.max(0, Math.floor(seconds)),
		});
	};

	const selectPreset = (preset: Preset) => {
		if (preset.permanent) {
			setCustomMode(false);
			onChange({ permanent: true, duration: duration || 3600 });
			return;
		}
		const seconds = preset.seconds ?? 3600;
		const parts = splitSeconds(seconds);
		setCustomMode(false);
		setDays(parts.days);
		setHours(parts.hours);
		setMinutes(parts.minutes);
		emitTimed(seconds);
	};

	const enableCustom = () => {
		const base = permanent
			? 30 * 60
			: duration > 0
				? duration
				: totalSeconds || 30 * 60;
		const parts = splitSeconds(base);
		// 至少 1 分钟，避免 0 时长
		const nextMinutes =
			parts.days === 0 && parts.hours === 0 && parts.minutes === 0
				? 30
				: parts.minutes;
		setCustomMode(true);
		setDays(parts.days);
		setHours(parts.hours);
		setMinutes(nextMinutes);
		emitTimed(parts.days * 86400 + parts.hours * 3600 + nextMinutes * 60);
	};

	const updateCustom = (
		next: Partial<{ days: number; hours: number; minutes: number }>,
	) => {
		const d = Math.max(0, Math.floor(next.days ?? days));
		const h = Math.min(23, Math.max(0, Math.floor(next.hours ?? hours)));
		const m = Math.min(59, Math.max(0, Math.floor(next.minutes ?? minutes)));
		setDays(d);
		setHours(h);
		setMinutes(m);
		setCustomMode(true);
		emitTimed(d * 86400 + h * 3600 + m * 60);
	};

	const clampInput = (raw: string, max?: number) => {
		const n = Number(raw);
		if (!Number.isFinite(n) || n < 0) return 0;
		const v = Math.floor(n);
		return max === undefined ? v : Math.min(max, v);
	};

	const presetBtn = (active: boolean) =>
		`inline-flex h-9 cursor-pointer items-center gap-1.5 rounded-sm border px-3 text-sm shadow-none transition-colors ${
			active
				? "border-foreground/20 bg-muted text-foreground"
				: "border-border bg-card text-foreground hover:bg-muted/50"
		}`;

	const inputClass =
		"border-input bg-card text-foreground focus-visible:ring-ring flex h-9 w-full rounded-sm border px-3 text-sm focus-visible:outline-none focus-visible:ring-2";

	return (
		<div className="space-y-3">
			<div className="flex flex-wrap gap-2">
				{PRESETS.map((preset) => {
					const active = activePresetId === preset.id;
					return (
						<button
							key={preset.id}
							type="button"
							className={presetBtn(active)}
							onClick={() => selectPreset(preset)}
						>
							{preset.permanent && <InfinityIcon size={14} />}
							{preset.label}
						</button>
					);
				})}
				<button
					type="button"
					className={presetBtn(activePresetId === "custom")}
					onClick={enableCustom}
				>
					自定义
				</button>
			</div>

			{!permanent && (customMode || activePresetId === "custom") && (
				<div className="border-border bg-muted/30 rounded-lg border p-3">
					<div className="text-foreground mb-2 flex items-center gap-2 text-xs font-medium">
						<Clock size={14} />
						<span>自定义时长</span>
						<span className="text-muted-foreground">天 / 小时 / 分钟</span>
					</div>
					<div className="grid grid-cols-3 gap-2">
						<label className="block">
							<span className="text-foreground mb-1 block text-xs">天</span>
							<input
								type="number"
								min={0}
								className={inputClass}
								value={days}
								onChange={(e) =>
									updateCustom({ days: clampInput(e.currentTarget.value) })
								}
							/>
						</label>
						<label className="block">
							<span className="text-foreground mb-1 block text-xs">小时</span>
							<input
								type="number"
								min={0}
								max={23}
								className={inputClass}
								value={hours}
								onChange={(e) =>
									updateCustom({
										hours: clampInput(e.currentTarget.value, 23),
									})
								}
							/>
						</label>
						<label className="block">
							<span className="text-foreground mb-1 block text-xs">分钟</span>
							<input
								type="number"
								min={0}
								max={59}
								className={inputClass}
								value={minutes}
								onChange={(e) =>
									updateCustom({
										minutes: clampInput(e.currentTarget.value, 59),
									})
								}
							/>
						</label>
					</div>
				</div>
			)}

			<div className="flex flex-wrap items-center gap-2">
				{!permanent ? (
					<div className="border-border bg-card text-foreground inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-medium">
						<Clock size={12} />
						{formatDurationSeconds(duration)} · {formatMuteEndsAt(duration)}
					</div>
				) : (
					<span className="border-border bg-card text-foreground inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-medium">
						<InfinityIcon size={12} />
						永久禁言，需手动解除
					</span>
				)}
			</div>
		</div>
	);
}
