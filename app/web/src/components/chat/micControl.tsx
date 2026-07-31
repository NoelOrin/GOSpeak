import { Show } from "solid-js";
import SvgIcon from "../svgIcon";

interface MicControlProps {
	name?: string;
	volume: () => number;
	isMute: () => boolean;
	onChange: (volume: number) => void;
	onCheck: (isMute: boolean) => void;
	/** 禁用时开关不可点击（如被服务端禁言），鼠标悬停提示 reason。 */
	disabled?: () => boolean;
	disabledTip?: string;
}

const MicControl = ({
	volume,
	isMute,
	onChange,
	onCheck,
	disabled,
	disabledTip,
	...props
}: MicControlProps) => {
	return (
		<div class="dropdown-top dropdown dropdown-hover">
			<label
				class="swap swap-flip size-11 cursor-pointer"
				classList={{
					"pointer-events-none opacity-40 cursor-not-allowed": disabled?.(),
				}}
			>
				<input
					type="checkbox"
					name={props.name || "input"}
					aria-label={isMute() ? "取消麦克风静音" : "麦克风静音"}
					checked={isMute()}
					disabled={disabled?.()}
					onInput={(e) => onCheck(e.target.checked)}
				/>

				{/* daisyUI v5: checked→swap-on 显示。isMute=true(静音)→checked→swap-on=mic-off */}
				<SvgIcon name="microphone-on" class="swap-off" />
				<SvgIcon name="microphone-off" class="swap-on" />
			</label>

			<Show when={disabled?.() && disabledTip}>
				<div class="z-1 px-2 pb-3 -translate-x-[30%] dropdown-content menu">
					<div class="px-2 py-1 text-xs text-warning bg-base-100 shadow-sm border rounded-box whitespace-nowrap">
						{disabledTip}
					</div>
				</div>
			</Show>

			<Show when={!disabled?.()}>
				<div
					tabIndex="-1"
					class="z-1 px-2 pb-3 -translate-x-[30%] dropdown-content menu"
				>
					<div
						data-tip={volume()}
						class="flex flex-col justify-center items-center bg-base-100 shadow-sm border rounded-box w-6 h-25 tooltip"
					>
						<input
							type="range"
							aria-label="麦克风音量"
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
export default MicControl;
