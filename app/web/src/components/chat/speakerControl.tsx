import SvgIcon from "../svgIcon";

interface SpeakerControlProps {
	name?: string;
	volume: () => number;
	isMute: () => boolean;
	onChange: (volume: number) => void;
	onCheck: (isMute: boolean) => void;
}

const SpeakerControl = ({
	volume,
	isMute,
	onChange,
	onCheck,
	...props
}: SpeakerControlProps) => {
	return (
		<div class="dropdown-top dropdown dropdown-hover">
			<label class="swap swap-flip size-11 cursor-pointer">
				{/* this hidden checkbox controls the state */}
				<input
					type="checkbox"
					name={props.name || "output"}
					aria-label={isMute() ? "取消扬声器静音" : "扬声器静音"}
					checked={isMute()}
					onInput={(e) => onCheck(e.target.checked)}
				/>
				{/* daisyUI v5: checked→swap-on 显示。isMute=true(静音)→checked→swap-on=volume-off */}
				<SvgIcon name="volume-on" class="swap-off" />

				{/* volume off icon */}
				<SvgIcon name="volume-off" class="swap-on" />
			</label>

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
						aria-label="扬声器音量"
						min="0"
						max="100"
						value={volume()}
						onInput={(e) => onChange(Number(e.target.value))}
						class="w-22 rotate-270 origin-right -translate-x-1/2 -translate-y-11 range range-xs"
					/>
				</div>
			</div>
		</div>
	);
};
export default SpeakerControl;
