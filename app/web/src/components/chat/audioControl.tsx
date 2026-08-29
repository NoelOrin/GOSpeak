import type { Component } from "solid-js";
import { Show } from "solid-js";

export interface ControlIconProps {
	class?: string;
	size?: number;
}

interface AudioControlProps {
	name: string;
	/** 静音态图标（daisyUI swap-on） */
	MutedIcon: Component<ControlIconProps>;
	/** 非静音态图标（daisyUI swap-off） */
	UnmutedIcon: Component<ControlIconProps>;
	muteLabel: string;
	unmuteLabel: string;
	volumeLabel: string;
	volume: () => number;
	isMute: () => boolean;
	onChange: (volume: number) => void;
	onCheck: (isMute: boolean) => void;
	/** 禁用时开关不可点击（如被服务端禁言），鼠标悬停提示 reason。 */
	disabled?: () => boolean;
	disabledTip?: string;
}

const AudioControl = ({
	name,
	MutedIcon,
	UnmutedIcon,
	muteLabel,
	unmuteLabel,
	volumeLabel,
	volume,
	isMute,
	onChange,
	onCheck,
	disabled,
	disabledTip,
}: AudioControlProps) => {
	return (
		<div class="dropdown dropdown-top dropdown-hover">
			<label
				class="swap swap-flip size-11 cursor-pointer"
				classList={{
					"pointer-events-none opacity-40 cursor-not-allowed": disabled?.(),
				}}
			>
				<input
					type="checkbox"
					name={name}
					aria-label={isMute() ? unmuteLabel : muteLabel}
					checked={isMute()}
					disabled={disabled?.()}
					onInput={(e) => onCheck(e.target.checked)}
				/>

				{/* daisyUI v5: checked→swap-on 显示。isMute=true(静音)→checked→swap-on=MutedIcon */}
				<UnmutedIcon class="swap-off" size={20} />
				<MutedIcon class="swap-on" size={20} />
			</label>

			<Show when={disabled?.() && disabledTip}>
				<div class="z-1 px-2 pb-3 -translate-x-1/3 dropdown-content menu">
					<div class="px-2 py-1 text-xs text-warning bg-base-100 shadow-sm border rounded-box whitespace-nowrap">
						{disabledTip}
					</div>
				</div>
			</Show>

			<Show when={!disabled?.()}>
				<div
					tabIndex="-1"
					class="z-1 px-2 pb-3 -translate-x-1/3 dropdown-content menu"
				>
					<div
						data-tip={volume()}
						class="flex flex-col justify-center items-center bg-base-100 shadow-sm border rounded-box w-6 h-25 tooltip"
					>
						<input
							type="range"
							aria-label={volumeLabel}
							min="0"
							max="100"
							value={volume()}
							onInput={(e) => onChange(Number(e.target.value))}
							class="w-22 rotate-270 origin-right -translate-x-1/2 -translate-y-11 range range-xs"
						/>
					</div>
				</div>
			</Show>
		</div>
	);
};
export default AudioControl;
