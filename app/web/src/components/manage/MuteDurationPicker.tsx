import Clock from "lucide-solid/icons/clock";
import InfinityIcon from "lucide-solid/icons/infinity";
import { createEffect, createMemo, createSignal, For, Show } from "solid-js";
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

export default function MuteDurationPicker(props: MuteDurationPickerProps) {
	const initial = splitSeconds(props.duration || 3600);
	const [days, setDays] = createSignal(initial.days);
	const [hours, setHours] = createSignal(initial.hours);
	const [minutes, setMinutes] = createSignal(initial.minutes);
	const [customMode, setCustomMode] = createSignal(
		!props.permanent && !isPresetSeconds(props.duration || 3600),
	);

	const totalSeconds = createMemo(
		() => days() * 86400 + hours() * 3600 + minutes() * 60,
	);

	const activePresetId = createMemo(() => {
		if (props.permanent) return "perm";
		if (customMode() || !isPresetSeconds(props.duration)) return "custom";
		const match = PRESETS.find(
			(p) => !p.permanent && p.seconds === props.duration,
		);
		return match?.id ?? "custom";
	});

	// 外部重置（提交后清空）时同步内部状态
	createEffect(() => {
		const permanent = props.permanent;
		const duration = props.duration;
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
		if (duration !== totalSeconds()) {
			const parts = splitSeconds(duration);
			setCustomMode(true);
			setDays(parts.days);
			setHours(parts.hours);
			setMinutes(parts.minutes);
		}
	});

	const emitTimed = (seconds: number) => {
		props.onChange({
			permanent: false,
			duration: Math.max(0, Math.floor(seconds)),
		});
	};

	const selectPreset = (preset: Preset) => {
		if (preset.permanent) {
			setCustomMode(false);
			props.onChange({ permanent: true, duration: props.duration || 3600 });
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
		const base = props.permanent
			? 30 * 60
			: props.duration > 0
				? props.duration
				: totalSeconds() || 30 * 60;
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
		const d = Math.max(0, Math.floor(next.days ?? days()));
		const h = Math.min(23, Math.max(0, Math.floor(next.hours ?? hours())));
		const m = Math.min(59, Math.max(0, Math.floor(next.minutes ?? minutes())));
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

	return (
		<div class="space-y-3">
			<div class="flex flex-wrap gap-2">
				<For each={PRESETS}>
					{(preset) => {
						const active = () => activePresetId() === preset.id;
						return (
							<button
								type="button"
								class="btn btn-sm h-8 min-h-8 rounded-full border px-3 shadow-none"
								classList={{
									"border-base-content/20 bg-base-200 text-base-content":
										active(),
									"border-base-300 bg-base-100 text-base-content/80 hover:bg-base-200/50":
										!active(),
								}}
								onClick={() => selectPreset(preset)}
							>
								<Show when={preset.permanent}>
									<InfinityIcon size={14} />
								</Show>
								{preset.label}
							</button>
						);
					}}
				</For>
				<button
					type="button"
					class="btn btn-sm h-8 min-h-8 rounded-full border px-3 shadow-none"
					classList={{
						"border-base-content/20 bg-base-200 text-base-content":
							activePresetId() === "custom",
						"border-base-300 bg-base-100 text-base-content/80 hover:bg-base-200/50":
							activePresetId() !== "custom",
					}}
					onClick={enableCustom}
				>
					自定义
				</button>
			</div>

			<Show
				when={
					!props.permanent && (customMode() || activePresetId() === "custom")
				}
			>
				<div class="rounded-xl border border-base-300 bg-base-200/30 p-3">
					<div class="mb-2 flex items-center gap-2 text-xs font-medium text-base-content/80">
						<Clock size={14} />
						<span>自定义时长</span>
						<span class="text-base-content/55">天 / 小时 / 分钟</span>
					</div>
					<div class="grid grid-cols-3 gap-2">
						<label class="form-control">
							<span class="label-text mb-1 text-xs text-base-content/75">
								天
							</span>
							<input
								type="number"
								min={0}
								class="input input-bordered input-sm bg-base-100 text-base-content"
								value={days()}
								onInput={(e) =>
									updateCustom({ days: clampInput(e.currentTarget.value) })
								}
							/>
						</label>
						<label class="form-control">
							<span class="label-text mb-1 text-xs text-base-content/75">
								小时
							</span>
							<input
								type="number"
								min={0}
								max={23}
								class="input input-bordered input-sm bg-base-100 text-base-content"
								value={hours()}
								onInput={(e) =>
									updateCustom({
										hours: clampInput(e.currentTarget.value, 23),
									})
								}
							/>
						</label>
						<label class="form-control">
							<span class="label-text mb-1 text-xs text-base-content/75">
								分钟
							</span>
							<input
								type="number"
								min={0}
								max={59}
								class="input input-bordered input-sm bg-base-100 text-base-content"
								value={minutes()}
								onInput={(e) =>
									updateCustom({
										minutes: clampInput(e.currentTarget.value, 59),
									})
								}
							/>
						</label>
					</div>
				</div>
			</Show>

			<div class="flex flex-wrap items-center gap-2">
				<Show
					when={!props.permanent}
					fallback={
						<span class="inline-flex items-center gap-1 rounded-full border border-base-300 bg-base-100 px-2.5 py-1 text-xs font-medium text-base-content/75">
							<InfinityIcon size={12} />
							永久禁言，需手动解除
						</span>
					}
				>
					<span class="inline-flex items-center gap-1 rounded-full border border-base-300 bg-base-100 px-2.5 py-1 text-xs font-medium text-base-content/75">
						<Clock size={12} />
						时长 {formatDurationSeconds(props.duration || totalSeconds())}
					</span>
					<span class="text-xs text-base-content/70">
						预计至 {formatMuteEndsAt(props.duration || totalSeconds())}
					</span>
					<Show when={(props.duration || totalSeconds()) <= 0}>
						<span class="text-xs font-medium text-base-content/70">
							请设置大于 0 的时长
						</span>
					</Show>
				</Show>
			</div>
		</div>
	);
}
